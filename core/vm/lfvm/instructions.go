package lfvm

import (
	"math/big"
	"math/bits"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"
)

var (
	big0 = big.NewInt(0)
)

func opStop(c *context) {
	//fmt.Printf("STOP\n")
	c.status = STOPPED
}

func opRevert(c *context) {
	//fmt.Printf("REVERT\n")
	c.result_offset = *c.stack.pop()
	c.result_size = *c.stack.pop()
	c.status = REVERTED
}

func opReturn(c *context) {
	//fmt.Printf("RETURN\n")
	c.result_offset = *c.stack.pop()
	c.result_size = *c.stack.pop()
	c.status = RETURNED
}
func opInvalid(c *context) {
	//fmt.Printf("INVALID\n")
	c.status = INVALID_INSTRUCTION
}

func opPc(c *context) {
	c.stack.pushEmpty().SetUint64(uint64(c.code[c.pc].arg))
}

func checkJumpDest(c *context) {
	if int(c.pc+1) >= len(c.code) || c.code[c.pc+1].opcode != JUMPDEST {
		c.SignalError(vm.ErrInvalidJump)
	}
	// Skip the interpretation of the JUMPDEST instruction
	c.pc++
	c.UseGas(1)
}

func opJump(c *context) {
	destination := c.stack.pop()
	// Update the PC to the jump destination -1 since interpreter will increase PC by 1 afterward.
	c.pc = int32(destination.Uint64()) - 1
	checkJumpDest(c)
}

func opJumpi(c *context) {
	destination := c.stack.pop()
	condition := c.stack.pop()
	//fmt.Printf("JUMPI %v %v\n", destination, condition)
	if !condition.IsZero() {
		// Update the PC to the jump destination -1 since interpreter will increase PC by 1 afterward.
		c.pc = int32(destination.Uint64()) - 1
		checkJumpDest(c)
	}
}

func opJumpTo(c *context) {
	// Update the PC to the jump destination -1 since interpreter will increase PC by 1 afterward.
	c.pc = int32(c.code[c.pc].arg) - 1
}

func opNoop(c *context) {
	// Nothing to do.
	panic("Should not be reachable")
}

func opPop(c *context) {
	//fmt.Printf("POP\n")
	c.stack.pop()
}

func opPush(c *context, n int) {
	z := c.stack.pushEmpty()
	num_instructions := int32(n/2 + n%2)
	data := c.code[c.pc : c.pc+num_instructions]

	_ = data[num_instructions-1]
	var value [32]byte
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			value[i] = byte(data[i/2].arg >> 8)
		} else {
			value[i] = byte(data[i/2].arg)
		}
	}
	z.SetBytes(value[0:n])
	c.pc += num_instructions - 1
}

func opPush1(c *context) {
	z := c.stack.pushEmpty()
	z[3], z[2], z[1] = 0, 0, 0
	z[0] = uint64(c.code[c.pc].arg >> 8)
}

func opPush2(c *context) {
	z := c.stack.pushEmpty()
	z[3], z[2], z[1] = 0, 0, 0
	z[0] = uint64(c.code[c.pc].arg)
}

func opPush3(c *context) {
	z := c.stack.pushEmpty()
	z[3], z[2], z[1] = 0, 0, 0
	data := c.code[c.pc : c.pc+2]
	_ = data[1]
	z[0] = uint64(data[0].arg)<<8 | uint64(data[1].arg>>8)
	c.pc += 1
}

func opPush4(c *context) {
	z := c.stack.pushEmpty()
	z[3], z[2], z[1] = 0, 0, 0

	data := c.code[c.pc : c.pc+2]
	_ = data[1] // causes bound check to be performed only once (may become unneded in the future)
	z[0] = (uint64(data[0].arg) << 16) | uint64(data[1].arg)
	c.pc += 1
}

func opPush32(c *context) {
	z := c.stack.pushEmpty()

	data := c.code[c.pc : c.pc+16]
	_ = data[15] // causes bound check to be performed only once (may become unneded in the future)
	z[3] = (uint64(data[0].arg) << 48) | (uint64(data[1].arg) << 32) | (uint64(data[2].arg) << 16) | uint64(data[3].arg)
	z[2] = (uint64(data[4].arg) << 48) | (uint64(data[5].arg) << 32) | (uint64(data[6].arg) << 16) | uint64(data[7].arg)
	z[1] = (uint64(data[8].arg) << 48) | (uint64(data[9].arg) << 32) | (uint64(data[10].arg) << 16) | uint64(data[11].arg)
	z[0] = (uint64(data[12].arg) << 48) | (uint64(data[13].arg) << 32) | (uint64(data[14].arg) << 16) | uint64(data[15].arg)
	c.pc += 15
}

func opDup(c *context, pos int) {
	//fmt.Printf("DUP%d\n", pos)
	c.stack.dup(pos)
}

func opSwap(c *context, pos int) {
	//fmt.Printf("SWAP%d\n", pos)
	c.stack.swap(pos + 1)
}

func opMstore(c *context) {
	var addr = c.stack.pop()
	var value = c.stack.pop()

	offset := int(addr.Uint64())
	c.memory.EnsureCapacity(offset, 32, c)
	if c.status == RUNNING {
		c.memory.SetWord(offset, value)
	}
}

func opMstore8(c *context) {
	var addr = c.stack.pop()
	var value = c.stack.pop()

	offset := int(addr.Uint64())
	c.memory.EnsureCapacity(offset, 1, c)
	if c.status == RUNNING {
		c.memory.SetByte(offset, byte(value.Uint64()))
	}
}

func opMload(c *context) {
	var trg = c.stack.peek()
	var addr = *trg

	offset := int(addr.Uint64())
	c.memory.EnsureCapacity(offset, 32, c)

	//fmt.Printf("MLOAD [%v]\n", addr)
	c.memory.CopyWord(offset, trg)
}

func opMsize(c *context) {
	c.stack.pushEmpty().SetUint64(uint64(c.memory.Len()))
}

func opSstore(c *context) {
	// Charge the gas price for this operation
	price, err := gasSStore(c)
	if err != nil || !c.UseGas(price) {
		return
	}

	var key = c.stack.pop()
	var value = c.stack.pop()

	// Perform storage update
	c.stateDB.SetState(c.contract.Address(), key.Bytes32(), value.Bytes32())
}

func opSload(c *context) {
	var top = c.stack.peek()
	top.SetBytes32(c.stateDB.GetState(c.contract.Address(), top.Bytes32()).Bytes())
}

func opCaller(c *context) {
	c.stack.pushEmpty().SetBytes20(c.contract.CallerAddress[:])
}

func opCallvalue(c *context) {
	//fmt.Printf("CALLVALUE\n")
	// Push a fake value on the stack.
	v, _ := uint256.FromBig(c.contract.Value())
	c.stack.push(v)
}

func opCallDatasize(c *context) {
	//fmt.Printf("CALLDATASIZE\n")
	c.stack.push(&c.callsize)
}

func opCallDataload(c *context) {
	//fmt.Printf("CALLDATALOAD\n")
	top := c.stack.peek()
	offset := top.Uint64()

	var value [32]byte
	for i := 0; i < 32; i++ {
		pos := i + int(offset)
		if pos < len(c.data) {
			value[i] = c.data[pos]
		} else {
			value[i] = 0
		}
	}
	top.SetBytes(value[:])
}

func opCallDataCopy(c *context) {
	var (
		memOffset  = c.stack.pop()
		dataOffset = c.stack.pop()
		length     = c.stack.pop()
	)
	dataOffset64, overflow := dataOffset.Uint64WithOverflow()
	if overflow {
		dataOffset64 = 0xffffffffffffffff
	}
	memOffset64 := memOffset.Uint64()
	length64 := length.Uint64()

	// Charge for the copy costs
	words := (length64 + 31) / 32
	price := 3 * words
	if !c.UseGas(price) {
		return
	}

	c.memory.EnsureCapacity(int(memOffset64), int(length64), c)
	c.memory.Set(memOffset64, length64, getData(c.data, dataOffset64, length64))
}

func opAnd(c *context) {
	//fmt.Printf("AND\n")
	a := c.stack.pop()
	b := c.stack.peek()
	b.And(a, b)
}

func opOr(c *context) {
	//fmt.Printf("OR\n")
	a := c.stack.pop()
	b := c.stack.peek()
	b.Or(a, b)
}

func opNot(c *context) {
	//fmt.Printf("OR\n")
	a := c.stack.peek()
	a.Not(a)
}

func opXor(c *context) {
	//fmt.Printf("OR\n")
	a := c.stack.pop()
	b := c.stack.peek()
	b.Xor(a, b)
}

func opIszero(c *context) {
	//fmt.Printf("ISZERO\n")
	top := c.stack.peek()
	if top.IsZero() {
		top.SetOne()
	} else {
		top.Clear()
	}
}

func opEq(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	//fmt.Printf("EQ %v %v\n", a, b)
	res := a.Cmp(b)
	for i := range b {
		b[i] = 0
	}
	if res == 0 {
		b[0] = 1
	} else {
		b[0] = 0
	}
}

func opLt(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	//fmt.Printf("LT %v %v\n", &a, b)
	if a.Lt(b) {
		b.SetOne()
	} else {
		b.Clear()
	}
}

func opGt(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	//fmt.Printf("GT %v %v\n", a, b)
	if a.Gt(b) {
		b.SetOne()
	} else {
		b.Clear()
	}
}

func opSlt(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	if a.Slt(b) {
		b.SetOne()
	} else {
		b.Clear()
	}
}
func opSgt(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	if a.Sgt(b) {
		b.SetOne()
	} else {
		b.Clear()
	}
}

func opShr(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	//fmt.Printf("SHR %02x=%d %v\n", a.ToByte(), a.ToByte(), b)
	// Note: this does not check for byte overflow!
	b.Rsh(b, uint(a.Uint64()))
}

func opShl(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	//fmt.Printf("SHR %02x=%d %v\n", a.ToByte(), a.ToByte(), b)
	// Note: this does not check for byte overflow!
	b.Lsh(b, uint(a.Uint64()))
}

func opSar(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	//fmt.Printf("SHR %02x=%d %v\n", a.ToByte(), a.ToByte(), b)
	// Note: this does not check for byte overflow!
	b.SRsh(b, uint(a.Uint64()))
}

func opSignExtend(c *context) {
	back, num := c.stack.pop(), c.stack.peek()
	num.ExtendSign(num, back)
}

func opByte(c *context) {
	th, val := c.stack.pop(), c.stack.peek()
	val.Byte(th)
}

func opAdd(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.Add(a, b)
}

func opSub(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.Sub(a, b)
}

func opMul(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.Mul(a, b)
}

func opMulMod(c *context) {
	a := c.stack.pop()
	b := c.stack.pop()
	n := c.stack.peek()
	n.MulMod(a, b, n)
}

func opDiv(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.Div(a, b)
}

func opSDiv(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.SDiv(a, b)
}

func opMod(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.Mod(a, b)
}

func opSMod(c *context) {
	a := c.stack.pop()
	b := c.stack.peek()
	b.SMod(a, b)
}

func opExp(c *context) {
	base, exponent := c.stack.pop(), c.stack.peek()
	if !c.UseGas(uint64(50 * exponent.ByteLen())) {
		return
	}
	exponent.Exp(base, exponent)
}

func opSha3(c *context) {
	offset, size := c.stack.pop(), c.stack.peek()

	c.memory.EnsureCapacity(int(offset.Uint64()), int(size.Uint64()), c)
	data := c.memory.GetSlice(int64(offset.Uint64()), int64(size.Uint64()))

	// charge dynamic gas price
	minimum_word_size := uint64((size.Uint64() + 31) / 32)
	price := 6 * minimum_word_size
	if !c.UseGas(price) {
		return
	}

	if c.hasher == nil {
		c.hasher = sha3.NewLegacyKeccak256().(keccakState)
	} else {
		c.hasher.Reset()
	}
	c.hasher.Write(data)
	c.hasher.Read(c.hasherBuf[:])
	size.SetBytes(c.hasherBuf[:])
}

func opGas(c *context) {
	c.stack.pushEmpty().SetUint64(c.contract.Gas)
}

func opDifficulty(c *context) {
	v, _ := uint256.FromBig(c.evm.Context.Difficulty)
	c.stack.push(v)
}

func opTimestamp(c *context) {
	v, _ := uint256.FromBig(c.evm.Context.Time)
	c.stack.push(v)
}

func opNumber(c *context) {
	v, _ := uint256.FromBig(c.evm.Context.BlockNumber)
	c.stack.push(v)
}

func opCoinbase(c *context) {
	c.stack.pushEmpty().SetBytes(c.evm.Context.Coinbase[:])
}

func opGasLimit(c *context) {
	c.stack.pushEmpty().SetUint64(c.evm.Context.GasLimit)
}

func opGasPrice(c *context) {
	v, _ := uint256.FromBig(c.evm.GasPrice)
	c.stack.push(v)
}

func opBalance(c *context) {
	slot := c.stack.peek()
	address := common.Address(slot.Bytes20())
	slot.SetFromBig(c.evm.StateDB.GetBalance(address))
}

func opSelfbalance(c *context) {
	c.stack.pushEmpty().SetFromBig(c.evm.StateDB.GetBalance(c.contract.Address()))
}

func opSelfdestruct(c *context) {
	// even death is not for free
	if !c.UseGas(gasSelfdestruct(c)) {
		return
	}
	beneficiary := c.stack.pop()
	balance := c.stateDB.GetBalance(c.contract.Address())
	c.stateDB.AddBalance(beneficiary.Bytes20(), balance)
	c.stateDB.Suicide(c.contract.Address())
	c.status = SUICIDED
}

func opChainId(c *context) {
	c.stack.pushEmpty().SetFromBig(c.evm.ChainConfig().ChainID)
}

func opBlockhash(c *context) {
	num := c.stack.peek()
	num64, overflow := num.Uint64WithOverflow()

	if overflow {
		num.Clear()
		return
	}
	var upper, lower uint64
	upper = c.evm.Context.BlockNumber.Uint64()
	if upper < 257 {
		lower = 0
	} else {
		lower = upper - 256
	}
	if num64 >= lower && num64 < upper {
		num.SetBytes(c.evm.Context.GetHash(num64).Bytes())
	} else {
		num.Clear()
	}
}

func opAddress(c *context) {
	c.stack.pushEmpty().SetBytes20(c.contract.Address().Bytes())
}

func opOrigin(c *context) {
	c.stack.pushEmpty().SetBytes20(c.evm.Origin.Bytes())
}

func opCodeSize(c *context) {
	c.stack.pushEmpty().SetUint64(uint64(len(c.contract.Code)))
}

func opCodeCopy(c *context) {
	var (
		memOffset  = c.stack.pop()
		codeOffset = c.stack.pop()
		length     = c.stack.pop()
	)
	uint64CodeOffset, overflow := codeOffset.Uint64WithOverflow()
	if overflow {
		uint64CodeOffset = 0xffffffffffffffff
	}

	// Charge for length of copied code
	words := (length.Uint64() + 31) / 32
	if !c.UseGas(3 * words) {
		return
	}

	codeCopy := getData(c.contract.Code, uint64CodeOffset, length.Uint64())
	c.memory.EnsureCapacity(int(memOffset.Uint64()), int(length.Uint64()), c)
	c.memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)
}

func opExtcodesize(c *context) {
	top := c.stack.peek()
	addr := top.Bytes20()
	top.SetUint64(uint64(c.stateDB.GetCodeSize(addr)))
	// Charge extra for cold locations.
	/*
		if !c.evm.StateDB.AddressInAccessList(addr) {
			fmt.Printf("Reducing gas from %v to %v\n", c.contract.Gas, c.contract.Gas-2500)
			c.contract.Gas -= 2500
				if c.interpreter == nil {
					c.evm.StateDB.AddAddressToAccessList(addr)
				}
		}
	*/
}

func opExtcodehash(c *context) {
	slot := c.stack.peek()
	address := common.Address(slot.Bytes20())
	if c.evm.StateDB.Empty(address) {
		slot.Clear()
	} else {
		slot.SetBytes32(c.evm.StateDB.GetCodeHash(address).Bytes())
	}
}

func opCreate(c *context) {
	var (
		value        = c.stack.pop()
		offset, size = c.stack.pop(), c.stack.pop()
		input        = c.memory.GetSlice(int64(offset.Uint64()), int64(size.Uint64()))
		gas          = c.contract.Gas
	)
	if true /*c.evm.chainRules.IsEIP150*/ {
		gas -= gas / 64
	}

	c.contract.UseGas(gas)
	//TODO: use uint256.Int instead of converting with toBig()
	var bigVal = big0
	if !value.IsZero() {
		bigVal = value.ToBig()
	}

	res, addr, returnGas, suberr := c.evm.Create(c.contract, input, gas, bigVal)

	success := c.stack.pushEmpty()
	if suberr != nil && suberr != vm.ErrCodeStoreOutOfGas {
		success.Clear()
	} else {
		success.SetBytes(addr.Bytes())
	}
	c.contract.Gas += returnGas

	if suberr == vm.ErrExecutionReverted {
		c.return_data = res
	} else {
		c.return_data = nil
	}
}

func opCreate2(c *context) {
	var (
		endowment    = c.stack.pop()
		offset, size = c.stack.pop(), c.stack.pop()
		salt         = c.stack.pop()
	)

	c.memory.EnsureCapacity(int(offset.Uint64()), int(size.Uint64()), c)

	input := c.memory.GetSlice(int64(offset.Uint64()), int64(size.Uint64()))

	// Charge for the code size
	words := (size.Uint64() + 31) / 32
	if !c.UseGas(6 * words) {
		return
	}

	// Apply EIP150
	gas := c.contract.Gas
	gas -= gas / 64
	if !c.UseGas(gas) {
		return
	}

	//TODO: use uint256.Int instead of converting with toBig()
	bigEndowment := big0
	if !endowment.IsZero() {
		bigEndowment = endowment.ToBig()
	}

	res, addr, returnGas, suberr := c.evm.Create2(c.contract, input, gas, bigEndowment, salt)

	// Push item on the stack based on the returned error.
	success := c.stack.pushEmpty()
	if suberr != nil {
		success.Clear()
	} else {
		success.SetBytes(addr.Bytes())
	}
	c.contract.Gas += returnGas

	if suberr == vm.ErrExecutionReverted {
		c.return_data = res
	} else {
		c.return_data = nil
	}
}

func getData(data []byte, start uint64, size uint64) []byte {
	length := uint64(len(data))
	if start > length {
		start = length
	}
	end := start + size
	if end > length {
		end = length
	}
	return common.RightPadBytes(data[start:end], int(size))
}

func opExtCodeCopy(c *context) {
	var (
		stack      = c.stack
		a          = stack.pop()
		memOffset  = stack.pop()
		codeOffset = stack.pop()
		length     = stack.pop()
	)
	uint64CodeOffset, overflow := codeOffset.Uint64WithOverflow()
	if overflow {
		uint64CodeOffset = 0xffffffffffffffff
	}
	addr := common.Address(a.Bytes20())
	codeCopy := getData(c.evm.StateDB.GetCode(addr), uint64CodeOffset, length.Uint64())
	c.memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

}

func neededMemorySize(offset, size *uint256.Int) uint64 {
	if size.IsZero() {
		return 0
	}
	return offset.Uint64() + size.Uint64()
}

func opCall(c *context) {
	stack := c.stack
	// Pop call parameters.
	provided_gas, addr, value, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()

	// Compute and charge gas price for call
	arg_memory_size := neededMemorySize(inOffset, inSize)
	ret_memory_size := neededMemorySize(retOffset, retSize)
	needed_memory_size := arg_memory_size
	if ret_memory_size > arg_memory_size {
		needed_memory_size = ret_memory_size
	}
	base_gas := c.memory.ExpansionCosts(int(needed_memory_size))

	// Consider value transfere fees for base costs
	if !value.IsZero() {
		base_gas += params.CallValueTransferGas
	}

	gas := callGas(c.contract.Gas, base_gas, provided_gas)

	// We need to check the existence of the target account before removing
	// the gas price for the other cost factors to make sure that the read
	// in the state DB is always happening. This is the current EVM behaviour,
	// and not doing it would be identified by the replay tool as an error.
	toAddr := common.Address(addr.Bytes20())

	// Charge for transfering value to a new address
	if !value.IsZero() && !c.stateDB.Exist(toAddr) && !c.UseGas(params.CallNewAccountGas) {
		return
	}

	// Charge value-transfere fees
	if !value.IsZero() && !c.UseGas(params.CallValueTransferGas) {
		return
	}

	if !c.UseGas(gas) {
		return
	}

	// Get the arguments from the memory.
	args := c.memory.GetSlice(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	var bigVal = big0
	//TODO: use uint256.Int instead of converting with toBig()
	// By using big0 here, we save an alloc for the most common case (non-ether-transferring contract calls),
	// but it would make more sense to extend the usage of uint256.Int
	if !value.IsZero() {
		gas += params.CallStipend
		bigVal = value.ToBig()
	}

	ret, returnGas, err := c.evm.Call(c.contract, toAddr, args, gas, bigVal)

	if err == nil || err == vm.ErrExecutionReverted {
		c.memory.EnsureCapacity(int(retOffset.Uint64()), int(retSize.Uint64()), c)
		c.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}

	success := stack.pushEmpty()
	if err != nil {
		success.Clear()
	} else {
		success.SetOne()
	}
	c.contract.Gas += returnGas
	c.return_data = ret
}

func opStaticCall(c *context) {
	stack := c.stack
	// Pop call parameters.
	provided_gas, addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()

	// Compute and charge gas price for call
	arg_memory_size := neededMemorySize(inOffset, inSize)
	ret_memory_size := neededMemorySize(retOffset, retSize)
	needed_memory_size := arg_memory_size
	if ret_memory_size > arg_memory_size {
		needed_memory_size = ret_memory_size
	}
	base_gas := c.memory.ExpansionCosts(int(needed_memory_size))
	gas := callGas(c.contract.Gas, base_gas, provided_gas)
	if !c.UseGas(gas) {
		return
	}

	toAddr := common.Address(addr.Bytes20())
	// Get arguments from the memory.
	args := c.memory.GetSlice(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	ret, returnGas, err := c.evm.StaticCall(c.contract, toAddr, args, gas)

	if err == nil || err == vm.ErrExecutionReverted {
		c.memory.EnsureCapacity(int(retOffset.Uint64()), int(retSize.Uint64()), c)
		c.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}

	success := stack.pushEmpty()
	if err != nil {
		success.Clear()
	} else {
		success.SetOne()
	}
	c.contract.Gas += returnGas
	c.return_data = ret
}

func opDelegateCall(c *context) {
	stack := c.stack
	// Pop call parameters.
	provided_gas, addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()

	// Compute and charge gas price for call
	arg_memory_size := neededMemorySize(inOffset, inSize)
	ret_memory_size := neededMemorySize(retOffset, retSize)
	needed_memory_size := arg_memory_size
	if ret_memory_size > arg_memory_size {
		needed_memory_size = ret_memory_size
	}
	base_gas := c.memory.ExpansionCosts(int(needed_memory_size))
	gas := callGas(c.contract.Gas, base_gas, provided_gas)
	if !c.UseGas(gas) {
		return
	}

	toAddr := common.Address(addr.Bytes20())
	// Get arguments from the memory.
	args := c.memory.GetSlice(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	ret, returnGas, err := c.evm.DelegateCall(c.contract, toAddr, args, gas)

	if err == nil || err == vm.ErrExecutionReverted {
		c.memory.EnsureCapacity(int(retOffset.Uint64()), int(retSize.Uint64()), c)
		c.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}

	success := stack.pushEmpty()
	if err != nil {
		success.Clear()
	} else {
		success.SetOne()
	}
	c.contract.Gas += returnGas
	c.return_data = ret
}

func opReturnDataSize(c *context) {
	c.stack.pushEmpty().SetUint64(uint64(len(c.return_data)))
}

func opReturnDataCopy(c *context) {
	var (
		memOffset  = c.stack.pop()
		dataOffset = c.stack.pop()
		length     = c.stack.pop()
	)

	offset64, overflow := dataOffset.Uint64WithOverflow()
	if overflow {
		c.SignalError(vm.ErrReturnDataOutOfBounds)
		return
	}
	// we can reuse dataOffset now (aliasing it for clarity)
	var end = dataOffset
	end.Add(dataOffset, length)
	end64, overflow := end.Uint64WithOverflow()
	if overflow || uint64(len(c.return_data)) < end64 {
		c.SignalError(vm.ErrReturnDataOutOfBounds)
		return
	}

	c.memory.EnsureCapacity(int(memOffset.Uint64()), int(length.Uint64()), c)

	words := (length.Uint64() + 31) / 32
	if !c.UseGas(3 * words) {
		return
	}

	c.memory.Set(memOffset.Uint64(), length.Uint64(), c.return_data[offset64:end64])
}

func opLog(c *context, size int) {
	topics := make([]common.Hash, size)
	stack := c.stack
	mStart, mSize := stack.pop(), stack.pop()
	for i := 0; i < size; i++ {
		addr := stack.pop()
		topics[i] = addr.Bytes32()
	}

	// charge for log size
	if !c.UseGas(8 * mSize.Uint64()) {
		return
	}

	// Expand memory if needed
	start := int(mStart.Uint64())
	log_size := int(mSize.Uint64())
	c.memory.EnsureCapacity(start, log_size, c)
	if c.status != RUNNING {
		return
	}
	d := c.memory.GetSlice(int64(start), int64(log_size))

	// make a copy of the data to disconnect from memory
	log_data := common.CopyBytes(d)

	c.evm.StateDB.AddLog(&types.Log{
		Address: c.contract.Address(),
		Topics:  topics,
		Data:    log_data,
		// This is a non-consensus field, but assigned here because
		// core/state doesn't know the current block number.
		BlockNumber: c.evm.Context.BlockNumber.Uint64(),
	})
}

// ----------------------------- Super Instructions -----------------------------

func opSwap1_Pop(c *context) {
	a1 := c.stack.pop()
	a2 := c.stack.peek()
	*a2 = *a1
}

func opSwap2_Pop(c *context) {
	a1 := c.stack.pop()
	*c.stack.Back(1) = *a1
}

func opPush1_Push1(c *context) {
	arg := c.code[c.pc].arg
	c.stack.stack_ptr += 2
	c.stack.Back(0).SetUint64(uint64(arg & 0xFF))
	c.stack.Back(1).SetUint64(uint64(arg >> 8))
}

func opPush1_Add(c *context) {
	arg := c.code[c.pc].arg
	trg := c.stack.peek()
	var carry uint64
	trg[0], carry = bits.Add64(trg[0], uint64(arg), 0)
	trg[1], carry = bits.Add64(trg[1], 0, carry)
	trg[2], carry = bits.Add64(trg[2], 0, carry)
	trg[3], _ = bits.Add64(trg[3], 0, carry)
}

func opPush1_Shl(c *context) {
	arg := c.code[c.pc].arg
	trg := c.stack.peek()
	trg.Lsh(trg, uint(arg))
}

func opPush1_Dup1(c *context) {
	arg := c.code[c.pc].arg
	c.stack.stack_ptr += 2
	c.stack.Back(0).SetUint64(uint64(arg))
	c.stack.Back(1).SetUint64(uint64(arg))
}

func opPush2_Jump(c *context) {
	// Directly take pushed value and jump to destination.
	c.pc = int32(c.code[c.pc].arg) - 1
	checkJumpDest(c)
}

func opPush2_Jumpi(c *context) {
	// Directly take pushed value and jump to destination.
	condition := c.stack.pop()
	if !condition.IsZero() {
		c.pc = int32(c.code[c.pc].arg) - 1
		checkJumpDest(c)
	}
}

func opSwap2_Swap1(c *context) {
	a1 := c.stack.Back(0)
	a2 := c.stack.Back(1)
	a3 := c.stack.Back(2)
	*a1, *a2, *a3 = *a2, *a3, *a1
}

func opDup2_Mstore(c *context) {
	var value = c.stack.pop()
	var addr = c.stack.peek()

	offset := int(addr.Uint64())
	c.memory.EnsureCapacity(offset, 32, c)
	if c.status == RUNNING {
		c.memory.SetWord(offset, value)
	}
}

func opDup2_Lt(c *context) {
	b := c.stack.Back(0)
	a := c.stack.Back(1)
	if a.Lt(b) {
		b.SetOne()
	} else {
		b.Clear()
	}
}

func opPopPop(c *context) {
	c.stack.stack_ptr -= 2
}

func opPop_Jump(c *context) {
	opPop(c)
	opJump(c)
}

func opIsZero_Push2_Jumpi(c *context) {
	condition := c.stack.pop()
	if condition.IsZero() {
		c.pc = int32(c.code[c.pc].arg) - 1
		checkJumpDest(c)
	}
}

func opSwap2_Swap1_Pop_Jump(c *context) {
	top := c.stack.pop()
	c.stack.pop()
	trg := c.stack.peek()
	c.pc = int32(trg.Uint64()) - 1
	*trg = *top
}

func opSwap1_Pop_Swap2_Swap1(c *context) {
	a1 := c.stack.pop()
	a2 := c.stack.Back(0)
	a3 := c.stack.Back(1)
	a4 := c.stack.Back(2)
	*a2, *a3, *a4 = *a3, *a4, *a1
}

func opPop_Swap2_Swap1_Pop(c *context) {
	c.stack.pop()
	a2 := c.stack.pop()
	a3 := c.stack.Back(0)
	a4 := c.stack.Back(1)
	*a3, *a4 = *a4, *a2
}

func opPush1_Push4_Dup3(c *context) {
	opPush1(c)
	c.pc++
	opPush4(c)
	opDup(c, 3)
}

func opAnd_Swap1_Pop_Swap2_Swap1(c *context) {
	opAnd(c)
	opSwap1_Pop_Swap2_Swap1(c)
}

func opPush1_Push1_Push1_Shl_Sub(c *context) {
	arg1 := c.code[c.pc].arg
	arg2 := c.code[c.pc+1].arg
	shift := uint8(arg2)
	value := uint8(arg1 & 0xFF)
	delta := uint8(arg1 >> 8)
	trg := c.stack.pushEmpty()
	trg.SetUint64(uint64(value))
	trg.Lsh(trg, uint(shift))
	trg.Sub(trg, uint256.NewInt(uint64(delta)))
	c.pc++
}
