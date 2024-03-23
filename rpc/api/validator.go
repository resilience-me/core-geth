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
	"fmt"

	"github.com/ethereum/ethash"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc/codec"
	"github.com/ethereum/go-ethereum/rpc/shared"
)

const (
	MinerApiVersion = "1.0"
)

var (
	// mapping between methods and handlers
	ValidatorMapping = map[string]minerhandler{
		"miner_hashrate":     (*minerApi).Hashrate,
		"miner_makeDAG":      (*minerApi).MakeDAG,
		"miner_setExtra":     (*minerApi).SetExtra,
		"miner_setGasPrice":  (*minerApi).SetGasPrice,
		"miner_setEtherbase": (*minerApi).SetEtherbase,
		"miner_startAutoDAG": (*minerApi).StartAutoDAG,
		"miner_start":        (*minerApi).StartValidator,
		"miner_stopAutoDAG":  (*minerApi).StopAutoDAG,
		"miner_stop":         (*minerApi).StopValidator,
		"miner_hashonion":    (*minerApi).SetHashonionFilepath,
	}
)

// miner callback handler
type validatorhandler func(*validatorApi, *shared.Request) (interface{}, error)

// miner api provider
type validatorApi struct {
	ethereum *eth.Ethereum
	methods  map[string]validatorhandler
	codec    codec.ApiCoder
}

// create a new miner api instance
func NewValidatorApi(ethereum *eth.Ethereum, coder codec.Codec) *validatorApi {
	return &validatorApi{
		ethereum: ethereum,
		methods:  ValidatorMapping,
		codec:    coder.New(nil),
	}
}

// Execute given request
func (self *validatorApi) Execute(req *shared.Request) (interface{}, error) {
	if callback, ok := self.methods[req.Method]; ok {
		return callback(self, req)
	}

	return nil, &shared.NotImplementedError{req.Method}
}

// collection with supported methods
func (self *validatorApi) Methods() []string {
	methods := make([]string, len(self.methods))
	i := 0
	for k := range self.methods {
		methods[i] = k
		i++
	}
	return methods
}

func (self *validatorApi) Name() string {
	return shared.MinerApiName
}

func (self *validatorApi) ApiVersion() string {
	return MinerApiVersion
}

func (self *validatorApi) StartValidator(req *shared.Request) (interface{}, error) {
	args := new(StartMinerArgs)
	if err := self.codec.Decode(req.Params, &args); err != nil {
		return nil, err
	}

	self.ethereum.StartAutoDAG()
	err := self.ethereum.StartMining(args.Threads)
	if err == nil {
		return true, nil
	}

	return false, err
}

func (self *validatorApi) StopValidator(req *shared.Request) (interface{}, error) {
	self.ethereum.StopMining()
	return true, nil
}

func (self *validatorApi) SetGasPrice(req *shared.Request) (interface{}, error) {
	args := new(GasPriceArgs)
	if err := self.codec.Decode(req.Params, &args); err != nil {
		return false, err
	}

	self.ethereum.Miner().SetGasPrice(common.String2Big(args.Price))
	return true, nil
}

func (self *validatorApi) SetEtherbase(req *shared.Request) (interface{}, error) {
	args := new(SetEtherbaseArgs)
	if err := self.codec.Decode(req.Params, &args); err != nil {
		return false, err
	}
	self.ethereum.SetEtherbase(args.Etherbase)
	return nil, nil
}

func (self *validatorApi) SetHashonionFilepath(req *shared.Request) (interface{}, error) {
	args := new(SetHashonionFilepathArgs)
	if err := self.codec.Decode(req.Params, &args); err != nil {
		return false, err
	}
	self.ethereum.Miner().SetHashonionFilepath(args.Filepath)
	return nil, nil
}

func (self *validatorApi) StartAutoDAG(req *shared.Request) (interface{}, error) {
	self.ethereum.StartAutoDAG()
	return true, nil
}

func (self *validatorApi) StopAutoDAG(req *shared.Request) (interface{}, error) {
	self.ethereum.StopAutoDAG()
	return true, nil
}

func (self *validatorApi) MakeDAG(req *shared.Request) (interface{}, error) {
	args := new(MakeDAGArgs)
	if err := self.codec.Decode(req.Params, &args); err != nil {
		return nil, err
	}

	if args.BlockNumber < 0 {
		return false, shared.NewValidationError("BlockNumber", "BlockNumber must be positive")
	}

	err := ethash.MakeDAG(uint64(args.BlockNumber), "")
	if err == nil {
		return true, nil
	}
	return false, err
}
