// Copyright 2015 The go-ethereum Authors
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

package api

import (
	"encoding/json"

	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc/shared"
)

type GasPriceArgs struct {
	Price string
}

func (args *GasPriceArgs) UnmarshalJSON(b []byte) (err error) {
	var obj []interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return shared.NewDecodeParamError(err.Error())
	}

	if len(obj) < 1 {
		return shared.NewInsufficientParamsError(len(obj), 1)
	}

	if pricestr, ok := obj[0].(string); ok {
		args.Price = pricestr
		return nil
	}

	return shared.NewInvalidTypeError("Price", "not a string")
}

type SetEtherbaseArgs struct {
	Etherbase common.Address
}

func (args *SetEtherbaseArgs) UnmarshalJSON(b []byte) (err error) {
	var obj []interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return shared.NewDecodeParamError(err.Error())
	}

	if len(obj) < 1 {
		return shared.NewInsufficientParamsError(len(obj), 1)
	}

	if addr, ok := obj[0].(string); ok {
		args.Etherbase = common.HexToAddress(addr)
		if (args.Etherbase == common.Address{}) {
			return shared.NewInvalidTypeError("Etherbase", "not a valid address")
		}
		return nil
	}

	return shared.NewInvalidTypeError("Etherbase", "not a string")
}


type SetHashonionFilepathArgs struct {
	Filepath string
}

func (args *SetHashonionFilepathArgs) UnmarshalJSON(b []byte) (err error) {
	var obj []interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return shared.NewDecodeParamError(err.Error())
	}

	if len(obj) < 1 {
		return shared.NewInsufficientParamsError(len(obj), 1)
	}

	if filepath, ok := obj[0].(string); ok {
		args.Filepath = filepath
		return nil
	}

	return shared.NewInvalidTypeError("Filepath", "not a string")
}
