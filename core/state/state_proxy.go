// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type stateProxyDB struct {
	db       *StateDB // state db
}

func NewStateProxyDB(db *StateDB) {
	p := new(stateProxyDB)
	p.db = db
	return p
}

func (s *stateProxyDB) CreateAccount(addr common.Address) bool {
	flag:= s.db.CreateAccount(addr)
	return flag
}

func (s *stateProxyDB) SubBalance(addr common.Address, amount *big.Int) {
	s.db.SubBalance(addr, amount)
}

func (s *stateProxyDB) AddBalance(addr common.Address, amount *big.Int) {
	s.db.AddBalance(addr, amount)
}

func (s *stateProxyDB) GetBalance(addr common.Address) *big.Int {
	balance:= s.db.GetBalance(addr)
	return balance
}

func (s *stateProxyDB) GetNonce(addr common.Address) uint64 {
	nonce:= s.db.GetNonce(addr)
	return nonce
}

func (s *stateProxyDB) SetNonce(addr common.Address, nonce uint64) {
	s.db.SetNonce(addr, nonce)
}

func (s *stateProxyDB) GetCodeHash(addr common.Address) common.Hash {
	hash := s.db.GetCodeHash(addr) 
	return hash
}

func (s *stateProxyDB) GetCode(addr common.Address) []byte {
	code := s.db.GetCode(addr)
	return code
}

func (s *stateProxyDB) SetCode(addr common.Address, code []byte) {
	s.db.SetCode(addr,code)
}

func (s *stateProxyDB) GetCodeSize(addr common.Address) int {
	size := s.db.GetCodeSize(addr)
	return size
}

func (s *stateProxyDB) AddRefund(gas uint64) {
	s.db.AddRefund(gas)
}

func (s *stateProxyDB) SubRefund(gas uint64) {
	s.db.SubRefund(gas)
}

func (s *stateProxyDB) GetRefund() uint64 {
	gas := s.db.GetRefund() 
	return gas
}

func (s *stateProxyDB) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	value := s.db.GetCommittedState(addr, key)
	return value 
}

func (s *stateProxyDB) GetState(addr common.Address, key common.Hash) common.Hash {
	value := s.db.GetState(addr,key) 
	return value
}

func (s *stateProxyDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	s.db.SetState(addr, key, value)
}

func (s *stateProxyDB) Suicide(addr common.Address) bool {
	ok := s.db.Suicide(addr)
	return ok
}

func (s *stateProxyDB) HasSuicided(addr common.Address) bool {
	hasSuicided := s.db.HasSuicided(addr)
	return hasSuicided
}

func (s *stateProxyDB) Exist(addr common.Address) bool {
	exists := s.db.Exist(addr)
	return exists
}

func (s *stateProxyDB) Empty(addr common.Address) bool {
	empty := s.db.Empty(addr)
	return empty
}

func (s *stateProxyDB) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	s.db.PrepareAccessList(sender, dest, precompile, txAccesss)
}

func (s *stateProxyDB) AddressInAccessList(addr common.Address) bool {
	ok := s.db.AddressInAccessList(addr)
	return ok
}

func (s *stateProxyDB) SlotInAccessList(addr common.Address, slot common.Hash) (bool, bool) {
	addressOk, slotOk := s.db.SlotInAccessList(addr, slot) 
	return addressOk, slotOk
}

func (s *stateProxyDB) AddAddressToAccessList(addr common.Address) {
	s.db.AddAddressToAccessList(addr)
}

func (s *stateProxyDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.db.AddSlotToAccessList(addr, slot)
}

func (s *stateProxyDB) RevertToSnapshot(snapshot int) {
	s.db.RevertToSnapshot(snapshot)
}

func (s *stateProxyDB) Snapshot() int {
	snapshot := s.db.Snapshot()
}

func (s *stateProxyDB) AddLog(log *types.Log) {
	s.db.AddLog(log)
}

func (s *stateProxyDB) AddPreimage(addr common.Hash, image []byte) {
	s.db.AddPreimage(addr, image)
}

func (s *stateProxyDB) ForEachStorage(addr common.Address, fn func(common.Hash, common.Hash) bool) error {
	err:= s.db.ForEachStorage(addr, fn) 
	return err
}

