// Copyright 2016 The go-ethereum Authors
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

package vm

import (
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func TestContractDictionarySimple1(t *testing.T) {
	encodedAddr := common.HexToAddress("0xdEcAf0562A19C9fFf21c9cEB476B2858E6f1F272")
	dict := NewContractDictionary()
	idx, err1 := dict.Encode(encodedAddr)
	decodedAddr, err2 := dict.Decode(idx)
	if encodedAddr != decodedAddr || err1 != nil || err2 != nil || idx != 0 {
		t.Fatalf("Encoding/Decoding is not symmetric")
	}
}

func TestContractDictionarySimple2(t *testing.T) {
	encodedAddr1 := common.HexToAddress("0xdEcAf0562A19C9fFf21c9cEB476B2858E6f1F272")
	encodedAddr2 := common.HexToAddress("0xdEcAf0562A19C9fFf21c9cEB476B2858E6f1F273")
	dict := NewContractDictionary()
	idx1, err1 := dict.Encode(encodedAddr1)
	idx2, err2 := dict.Encode(encodedAddr2)
	decodedAddr1, err3 := dict.Decode(idx1)
	decodedAddr2, err4 := dict.Decode(idx2)
	if encodedAddr1 != decodedAddr1 || err1 != nil || err3 != nil || idx1 != 0 {
		t.Fatalf("Encoding/Decoding is not symmetric")
	}
	if encodedAddr2 != decodedAddr2 || err2 != nil || err4 != nil || idx2 != 1 {
		t.Fatalf("Encoding/Decoding is not symmetric")
	}
}

func TestContractDictionaryOverflow(t *testint T)
{
}

