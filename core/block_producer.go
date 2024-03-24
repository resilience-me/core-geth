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

package core

import (
	"fmt"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/pow"
	"gopkg.in/fatih/set.v0"
)

var jsonlogger = logger.NewJsonLogger()

const (
	resultQueueSize  = 10
	miningLogAtDepth = 5
)

type uint64RingBuffer struct {
	ints []uint64 //array of all integers in buffer
	next int      //where is the next insertion? assert 0 <= next < len(ints)
}

// environment is the workers current environment and holds
// all of the current state information
type Work struct {
	state              *state.StateDB     // apply state changes here
	coinbase           *state.StateObject // the coinbase account
	ancestors          *set.Set           // ancestor set (used for checking uncle parent validity)
	family             *set.Set           // family set (used for checking uncle invalidity)
	uncles             *set.Set           // uncle set
	remove             *set.Set           // tx which will be removed
	tcount             int                // tx count in cycle
	ignoredTransactors *set.Set
	lowGasTransactors  *set.Set
	ownedAccounts      *set.Set
	lowGasTxs          types.Transactions
	localMinedBlocks   *uint64RingBuffer // the most recent block numbers that were mined locally (used to check block inclusion)

	Block *types.Block // the new block

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt

	createdAt time.Time
}

type hashonionFile struct {
	Root common.Hash `json:"root"`
	Layers int `json:"layers"`
}
type hashonion struct {
	buffer [][]byte
	file hashonionFile
	filepath string
}

type Result struct {
	Work  *Work
	Block *types.Block
}

// worker is the main object which takes care of applying messages to the new state
type blockProducer struct {
	mu sync.Mutex

	recv   chan *Result
	mux    *event.TypeMux
	quit   chan struct{}

	eth     Backend
	chain   *ChainManager
	proc    *BlockProcessor
	extraDb common.Database

	panarchy  *Panarchy
	hashonion hashonion
	
	validator common.Address
	gasPrice *big.Int
	extra    []byte

	currentMu sync.Mutex
	current   *Work

	uncleMu        sync.Mutex
	possibleUncles map[common.Hash]*types.Block

	txQueueMu sync.Mutex
	txQueue   map[common.Hash]*types.Transaction

	// atomic status counters
	mining int32

	fullValidation bool
}

func newBlockProducer(validator common.Address, eth Backend, panarchy *Panarchy) *worker {
	blockProducer := &blockProducer{
		eth:            eth,
		mux:            eth.EventMux(),
		extraDb:        eth.ExtraDb(),
		recv:           make(chan *Result, resultQueueSize),
		gasPrice:       new(big.Int),
		chain:          eth.ChainManager(),
		proc:           eth.BlockProcessor(),
		possibleUncles: make(map[common.Hash]*types.Block),
		validator:      validator,
		txQueue:        make(map[common.Hash]*types.Transaction),
		quit:           make(chan struct{}),
		fullValidation: false,
		panarchy: panarchy,
	}
	go blockProducer.update()
	go blockProducer.wait()

	blockProducer.startProduceBlock()

	return blockProducer
}

func (self *blockProducer) setEtherbase(addr common.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.validator = addr
}

func (self *blockProducer) pendingState() *state.StateDB {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()
	return self.current.state
}

func (self *blockProducer) pendingBlock() *types.Block {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	if atomic.LoadInt32(&self.mining) == 0 {
		return types.NewBlock(
			self.current.header,
			self.current.txs,
			nil,
			self.current.receipts,
		)
	}
	return self.current.Block
}

func (self *blockProducer) start() {
	self.mu.Lock()
	defer self.mu.Unlock()

	atomic.StoreInt32(&self.mining, 1)
	self.loadHashonion()
	self.startProduceBlock()
}

func (self *blockProducer) stop() {
	self.mu.Lock()
	defer self.mu.Unlock()

	atomic.StoreInt32(&self.mining, 0)
}

func (self *blockProducer) update() {
	events := self.mux.Subscribe(ChainHeadEvent{}, ChainSideEvent{}, TxPreEvent{})

out:
	for {
		select {
		case event := <-events.Chan():
			switch ev := event.(type) {
			case ChainHeadEvent:
				self.startProduceBlock()
			case ChainSideEvent:
				self.uncleMu.Lock()
				self.possibleUncles[ev.Block.Hash()] = ev.Block
				self.uncleMu.Unlock()
			case TxPreEvent:
				// Apply transaction to the pending state if we're not mining
				if atomic.LoadInt32(&self.mining) == 0 {
					self.currentMu.Lock()
					self.current.commitTransactions(types.Transactions{ev.Tx}, self.gasPrice, self.proc)
					self.currentMu.Unlock()
				}
			}
		case <-self.quit:
			break out
		}
	}

	events.Unsubscribe()
}

func newLocalMinedBlock(blockNumber uint64, prevMinedBlocks *uint64RingBuffer) (minedBlocks *uint64RingBuffer) {
	if prevMinedBlocks == nil {
		minedBlocks = &uint64RingBuffer{next: 0, ints: make([]uint64, miningLogAtDepth+1)}
	} else {
		minedBlocks = prevMinedBlocks
	}

	minedBlocks.ints[minedBlocks.next] = blockNumber
	minedBlocks.next = (minedBlocks.next + 1) % len(minedBlocks.ints)
	return minedBlocks
}

func (self *blockProducer) wait() {
	for {
		for result := range self.recv {

			if result == nil {
				continue
			}
			block := result.Block

			self.current.state.Sync()
			if self.fullValidation {
				if _, err := self.chain.InsertChain(types.Blocks{block}); err != nil {
					glog.V(logger.Error).Infoln("mining err", err)
					continue
				}
				go self.mux.Post(NewMinedBlockEvent{block})
			} else {
				parent := self.chain.GetBlock(block.ParentHash())
				if parent == nil {
					glog.V(logger.Error).Infoln("Invalid block found during mining")
					continue
				}
				if err := ValidateHeader(block.Header(), parent); err != nil && err != BlockFutureErr {
					glog.V(logger.Error).Infoln("Invalid header on mined block:", err)
					continue
				}

				stat, err := self.chain.WriteBlock(block, false)
				if err != nil {
					glog.V(logger.Error).Infoln("error writing block to chain", err)
					continue
				}
				// check if canon block and write transactions
				if stat == CanonStatTy {
					// This puts transactions in a extra db for rpc
					PutTransactions(self.extraDb, block, block.Transactions())
					// store the receipts
					PutReceipts(self.extraDb, self.current.receipts)
				}

				// broadcast before waiting for validation
				go func(block *types.Block, logs state.Logs) {
					self.mux.Post(NewMinedBlockEvent{block})
					self.mux.Post(ChainEvent{block, block.Hash(), logs})
					if stat == CanonStatTy {
						self.mux.Post(ChainHeadEvent{block})
						self.mux.Post(logs)
					}
				}(block, self.current.state.Logs())
			}

			// check staleness and display confirmation
			var stale, confirm string
			canonBlock := self.chain.GetBlockByNumber(block.NumberU64())
			if canonBlock != nil && canonBlock.Hash() != block.Hash() {
				stale = "stale "
			} else {
				confirm = "Wait 5 blocks for confirmation"
				self.current.localMinedBlocks = newLocalMinedBlock(block.Number().Uint64(), self.current.localMinedBlocks)
			}
			glog.V(logger.Info).Infof("🔨  Mined %sblock (#%v / %x). %s", stale, block.Number(), block.Hash().Bytes()[:4], confirm)
		}
	}
}

// makeCurrent creates a new environment for the current cycle.
func (self *blockProducer) makeCurrent(parent *types.Block, header *types.Header) {
	state := state.New(parent.Root(), self.eth.StateDb())
	current := &Work{
		state:     state,
		ancestors: set.New(),
		family:    set.New(),
		uncles:    set.New(),
		header:    header,
		coinbase:  state.GetOrNewStateObject(self.coinbase),
		createdAt: time.Now(),
	}

	// when 08 is processed ancestors contain 07 (quick block)
	for _, ancestor := range self.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			current.family.Add(uncle.Hash())
		}
		current.family.Add(ancestor.Hash())
		current.ancestors.Add(ancestor.Hash())
	}
	accounts, _ := self.eth.AccountManager().Accounts()

	// Keep track of transactions which return errors so they can be removed
	current.remove = set.New()
	current.tcount = 0
	current.ignoredTransactors = set.New()
	current.lowGasTransactors = set.New()
	current.ownedAccounts = accountAddressesSet(accounts)
	if self.current != nil {
		current.localMinedBlocks = self.current.localMinedBlocks
	}
	self.current = current
}

func (w *blockProducer) setGasPrice(p *big.Int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// calculate the minimal gas price the miner accepts when sorting out transactions.
	const pct = int64(90)
	w.gasPrice = gasprice(p, pct)

	w.mux.Post(GasPriceChanged{w.gasPrice})
}

func (self *blockProducer) isBlockLocallyMined(deepBlockNum uint64) bool {
	//Did this instance mine a block at {deepBlockNum} ?
	var isLocal = false
	for idx, blockNum := range self.current.localMinedBlocks.ints {
		if deepBlockNum == blockNum {
			isLocal = true
			self.current.localMinedBlocks.ints[idx] = 0 //prevent showing duplicate logs
			break
		}
	}
	//Short-circuit on false, because the previous and following tests must both be true
	if !isLocal {
		return false
	}

	//Does the block at {deepBlockNum} send earnings to my coinbase?
	var block = self.chain.GetBlockByNumber(deepBlockNum)
	return block != nil && block.Coinbase() == self.coinbase
}

func (self *blockProducer) logLocalMinedBlocks(previous *Work) {
	if previous != nil && self.current.localMinedBlocks != nil {
		nextBlockNum := self.current.Block.NumberU64()
		for checkBlockNum := previous.Block.NumberU64(); checkBlockNum < nextBlockNum; checkBlockNum++ {
			inspectBlockNum := checkBlockNum - miningLogAtDepth
			if self.isBlockLocallyMined(inspectBlockNum) {
				glog.V(logger.Info).Infof("🔨 🔗  Mined %d blocks back: block #%v", miningLogAtDepth, inspectBlockNum)
			}
		}
	}
}
func (self *blockProducer) startProduceBlock() {
    if self.stopCh != nil {
        close(self.stopCh)
    }
    self.stopCh = make(chan struct{})
    go self.produceBlock(self.stopCh)
}
func (self *blockProducer) produceBlock(stop <-chan struct{}) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.uncleMu.Lock()
	defer self.uncleMu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	parent := self.chain.CurrentBlock()

	tstamp := parent.Time().Int64() + self.panarchy.Period()

	now := time.Now().Unix()
	if tstamp < now {
		tstamp = now
	}
	i := big.NewInt(0)
	num := new(big.Int).Add(parent.Number(), common.Big1)
	
	if atomic.LoadInt32(&self.mining) == 1 {
		index := self.panarchy.schedule(parent.Time())
		electionLength := electionLength(index, self.current.state)
		delay := time.Unix(tstamp, 0).Sub(time.Now())
		loop:
		for {
			select {
			case <-stop:
				return
			case <-time.After(delay):
				validator := isValidator(index, electionLength, parent.Random(), i, self.current.state)

				if validator == self.validator {
					break loop
				}
				i.Add(i, common.Big1)
				delay = time.Duration(self.panarchy.Deadline()) * time.Second
			}
		}
		tstamp += int64(self.panarchy.Deadline())*i.Int64()
	}

	
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num,
		Skipped:    new(big.Int).Add(parent.Skipped(), i),
		GasLimit:   CalcGasLimit(parent),
		GasUsed:    new(big.Int),
		Time:       big.NewInt(tstamp),
		Random:     new(big.Int),
	}

	previous := self.current
	self.makeCurrent(parent, header)
	current := self.current

	if atomic.LoadInt32(&self.mining) == 1 {
		currentHash := hashonionFromStorageOrNew(self.validator, num, current.state)
		err, preimage := self.getHashonionPreimage(currentHash)
		if err != nil {
			return
		}
		writeHashToContract(preimage, self.validator, self.current.state)
		header.Random.Xor(parent.Random(), new(big.Int).SetBytes(preimage))
	}

	// commit transactions for this run.
	transactions := self.eth.TxPool().GetTransactions()
	sort.Sort(types.TxByNonce{transactions})
	current.coinbase.SetGasLimit(header.GasLimit)
	current.commitTransactions(transactions, self.gasPrice, self.proc)
	self.eth.TxPool().RemoveTransactions(current.lowGasTxs)

	// compute uncles for the new block.
	var (
		uncles    []*types.Header
		badUncles []common.Hash
	)
	for hash, uncle := range self.possibleUncles {
		if len(uncles) == 2 {
			break
		}
		if err := self.commitUncle(uncle.Header()); err != nil {
			if glog.V(logger.Ridiculousness) {
				glog.V(logger.Detail).Infof("Bad uncle found and will be removed (%x)\n", hash[:4])
				glog.V(logger.Detail).Infoln(uncle)
			}
			badUncles = append(badUncles, hash)
		} else {
			glog.V(logger.Debug).Infof("commiting %x as uncle\n", hash[:4])
			uncles = append(uncles, uncle.Header())
		}
	}
	for _, hash := range badUncles {
		delete(self.possibleUncles, hash)
	}

	if atomic.LoadInt32(&self.mining) == 1 {
		// commit state root after all state transitions.
		AccumulateRewards(current.coinbase, self.current.state)
		current.state.SyncObjects()
		header.Root = current.state.Root()
	}

	// create the new block whose nonce will be mined.
	current.Block = types.NewBlock(header, current.txs, uncles, current.receipts)

	// We only care about logging if we're actually mining.
	if atomic.LoadInt32(&self.mining) == 1 {
		glog.V(logger.Info).Infof("commit new work on block %v with %d txs & %d uncles. Took %v\n", current.Block.Number(), current.tcount, len(uncles), time.Since(tstart))
		self.logLocalMinedBlocks(previous)
	}

	if atomic.LoadInt32(&self.mining) == 1 {
		headerRlp, err := rlp.EncodeToBytes(self.current.header)
		if err != nil {
		}
		sig, err := self.eth.AccountManager().Sign(self.coinbase, crypto.Keccak256(headerRlp))
		if err != nil {
		}
		current.Block.SetSignature(sig)
		self.recv <- &Result{current, current.Block}
	}
}

func (self *blockProducer) getHashonionPreimage(currentHash common.Hash) (error, []byte) {

	var preimage []byte
	if len(self.hashonion.buffer) > 0 {
		preimage = self.hashonion.buffer[len(self.hashonion.buffer)-1]
	}
	var hash []byte

	for i := 0; i < compensateForPossibleReorg; i++ {
		hash = crypto.Keccak256(preimage)

		if currentHash == common.BytesToHash(hash) {
			if i == 0 {
				self.hashonion.file.Layers--
				if err := self.writeHashonion(); err != nil {
					return nil
				}
				self.hashonion.buffer = self.hashOnion.buffer[:len(self.hashOnion.buffer)-1]
			}
			return nil, preimage
		}
		preimage = hash
	}
	return glog.V(logger.Error).Infoln("Hash onion does not fit the hash in validator contract"), nil
}

func (self *blockProducer) commitUncle(uncle *types.Header) error {
	hash := uncle.Hash()
	if self.current.uncles.Has(hash) {
		return UncleError("Uncle not unique")
	}
	if !self.current.ancestors.Has(uncle.ParentHash) {
		return UncleError(fmt.Sprintf("Uncle's parent unknown (%x)", uncle.ParentHash[0:4]))
	}
	if self.current.family.Has(hash) {
		return UncleError(fmt.Sprintf("Uncle already in family (%x)", hash))
	}
	self.current.uncles.Add(uncle.Hash())
	return nil
}

func (env *Work) commitTransactions(transactions types.Transactions, gasPrice *big.Int, proc *BlockProcessor) {
	for _, tx := range transactions {
		// We can skip err. It has already been validated in the tx pool
		from, _ := tx.From()

		// Check if it falls within margin. Txs from owned accounts are always processed.
		if tx.GasPrice().Cmp(gasPrice) < 0 && !env.ownedAccounts.Has(from) {
			// ignore the transaction and transactor. We ignore the transactor
			// because nonce will fail after ignoring this transaction so there's
			// no point
			env.lowGasTransactors.Add(from)

			glog.V(logger.Info).Infof("transaction(%x) below gas price (tx=%v ask=%v). All sequential txs from this address(%x) will be ignored\n", tx.Hash().Bytes()[:4], common.CurrencyToString(tx.GasPrice()), common.CurrencyToString(gasPrice), from[:4])
		}

		// Continue with the next transaction if the transaction sender is included in
		// the low gas tx set. This will also remove the tx and all sequential transaction
		// from this transactor
		if env.lowGasTransactors.Has(from) {
			// add tx to the low gas set. This will be removed at the end of the run
			// owned accounts are ignored
			if !env.ownedAccounts.Has(from) {
				env.lowGasTxs = append(env.lowGasTxs, tx)
			}
			continue
		}

		// Move on to the next transaction when the transactor is in ignored transactions set
		// This may occur when a transaction hits the gas limit. When a gas limit is hit and
		// the transaction is processed (that could potentially be included in the block) it
		// will throw a nonce error because the previous transaction hasn't been processed.
		// Therefor we need to ignore any transaction after the ignored one.
		if env.ignoredTransactors.Has(from) {
			continue
		}

		env.state.StartRecord(tx.Hash(), common.Hash{}, 0)

		err := env.commitTransaction(tx, proc)
		switch {
		case state.IsGasLimitErr(err):
			// ignore the transactor so no nonce errors will be thrown for this account
			// next time the worker is run, they'll be picked up again.
			env.ignoredTransactors.Add(from)

			glog.V(logger.Detail).Infof("Gas limit reached for (%x) in this block. Continue to try smaller txs\n", from[:4])
		case err != nil:
			env.remove.Add(tx.Hash())

			if glog.V(logger.Detail) {
				glog.Infof("TX (%x) failed, will be removed: %v\n", tx.Hash().Bytes()[:4], err)
			}
		default:
			env.tcount++
		}
	}
}

func (env *Work) commitTransaction(tx *types.Transaction, proc *BlockProcessor) error {
	snap := env.state.Copy()
	receipt, _, err := proc.ApplyTransaction(env.coinbase, env.state, env.header, tx, env.header.GasUsed, true)
	if err != nil {
		env.state.Set(snap)
		return err
	}
	env.txs = append(env.txs, tx)
	env.receipts = append(env.receipts, receipt)
	return nil
}

// TODO: remove or use
func (self *worker) HashRate() int64 {
	return 0
}

// gasprice calculates a reduced gas price based on the pct
// XXX Use big.Rat?
func gasprice(price *big.Int, pct int64) *big.Int {
	p := new(big.Int).Set(price)
	p.Div(p, big.NewInt(100))
	p.Mul(p, big.NewInt(pct))
	return p
}

func accountAddressesSet(accounts []accounts.Account) *set.Set {
	accountSet := set.New()
	for _, account := range accounts {
		accountSet.Add(account.Address)
	}
	return accountSet
}

func (self *blockProducer) loadHashonion() error {
	filePath := self.hashonion.filepath
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v, hashonion.filepath: %v", err, filePath)
	}
	defer file.Close()
	
	err = json.NewDecoder(file).Decode(&self.hashonion.file)
	if err != nil {
		return fmt.Errorf("error decoding JSON: %v", err)
	}
	if self.hashonion.file.Layers <= 0 {
		return fmt.Errorf("Hash onion has no more layers: %v", err)
	}
	self.hashonion.buffer = append(self.hashonion.buffer, self.hashonion.file.Root.Bytes())
	for i := 0; i < self.hashonion.file.Layers-1; i++ {
		self.hashonion.buffer = append(self.hashonion.buffer, crypto.Keccak256(self.hashonion.buffer[i]))
	}
	return nil
}

func (self *blockProducer) writeHashonion() error {
    filePath := self.hashonion.filepath
    file, err := os.Create(filePath)
    if err != nil {
        return fmt.Errorf("error creating file: %v, hashonionFilePath: %v", err, filePath)
    }
    defer file.Close()

    err = json.NewEncoder(file).Encode(self.hashonion.file)
    if err != nil {
        return fmt.Errorf("error encoding hashonion to JSON: %v", err)
    }

    return nil
}

func (self *blockProducer) setHashonionFilepath(filepath string) {
	self.hashonion.filepath = filepath
}
