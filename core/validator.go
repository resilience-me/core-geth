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

// Package miner implements Ethereum block creation and mining.
package core

import (
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/pow"
)

type Validator struct {
	mux *event.TypeMux

	blockProducer *blockProducer

	MinAcceptedGasPrice *big.Int

	coinbase common.Address
	mining   int32
	eth      Backend

	canStart    int32 // can start indicates whether we can start the mining operation
	shouldStart int32 // should start indicates whether we should start after sync
}

func New(eth Backend, mux *event.TypeMux) *Miner {
	validator := &Validator{eth: eth, mux: mux, blockProducer: newBlockProducer(common.Address{}, eth), canStart: 1}
	go validator.update()

	return validator
}

// update keeps track of the downloader events. Please be aware that this is a one shot type of update loop.
// It's entered once and as soon as `Done` or `Failed` has been broadcasted the events are unregistered and
// the loop is exited. This to prevent a major security vuln where external parties can DOS you with blocks
// and halt your mining operation for as long as the DOS continues.
func (self *Validator) update() {
	events := self.mux.Subscribe(downloader.StartEvent{}, downloader.DoneEvent{}, downloader.FailedEvent{})
out:
	for ev := range events.Chan() {
		switch ev.(type) {
		case downloader.StartEvent:
			atomic.StoreInt32(&self.canStart, 0)
			if self.Running() {
				self.Stop()
				atomic.StoreInt32(&self.shouldStart, 1)
				glog.V(logger.Info).Infoln("Mining operation aborted due to sync operation")
			}
		case downloader.DoneEvent, downloader.FailedEvent:
			shouldStart := atomic.LoadInt32(&self.shouldStart) == 1

			atomic.StoreInt32(&self.canStart, 1)
			atomic.StoreInt32(&self.shouldStart, 0)
			if shouldStart {
				self.Start(self.coinbase, self.threads)
			}
			// unsubscribe. we're only interested in this event once
			events.Unsubscribe()
			// stop immediately and ignore all further pending events
			break out
		}
	}
}

func (m *Validator) SetGasPrice(price *big.Int) {
	// FIXME block tests set a nil gas price. Quick dirty fix
	if price == nil {
		return
	}

	m.worker.setGasPrice(price)
}

func (self *Validator) Start(coinbase common.Address) {
	atomic.StoreInt32(&self.shouldStart, 1)
	self.worker.coinbase = coinbase
	self.coinbase = coinbase

	if atomic.LoadInt32(&self.canStart) == 0 {
		glog.V(logger.Info).Infoln("Can not start mining operation due to network sync (starts when finished)")
		return
	}

	atomic.StoreInt32(&self.mining, 1)

	glog.V(logger.Info).Infof("Starting mining operation (CPU=%d TOT=%d)\n", threads, len(self.worker.agents))

	self.worker.start()
}

func (self *Validator) Stop() {
	self.worker.stop()
	atomic.StoreInt32(&self.mining, 0)
	atomic.StoreInt32(&self.shouldStart, 0)
}

func (self *Validator) Running() bool {
	return atomic.LoadInt32(&self.mining) > 0
}

func (self *Validator) PendingState() *state.StateDB {
	return self.worker.pendingState()
}

func (self *Validator) PendingBlock() *types.Block {
	return self.worker.pendingBlock()
}

func (self *Validator) SetEtherbase(addr common.Address) {
	self.coinbase = addr
	self.worker.setEtherbase(addr)
}
