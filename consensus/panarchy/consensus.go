package panarchy

import (
	"sync"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/vars"
)

var (
	errInvalidTimestamp = errors.New("invalid timestamp")
}

type StorageSlots {
	election []byte
	hashOnion []byte
	validSince []byte
}
type Schedule {
	genesis uint64
	period uint64
}
type ValidatorContract {
	slots StorageSlots
	addr common.Address
	schedule Schedule
}

type HashOnion struct {
	root common.Hash `json:"root"`
	layers int `json:"layers"`
}

type Panarchy struct {
	config	*ctypes.PanarchyConfig
	trie Trie
	contract ValidatorContract
	hashOnion HashOnion
	lock sync.RWMutex
}

func pad(val []byte) []byte {
	return common.LeftPadBytes(val, 32)
}
func weeksToSeconds(weeks uint64) uint64 {
	return weeks*7*24*60*60
}

func New(config *ctypes.PanarchyConfig, db ethdb.Database) *Panarchy {
	return &Panarchy{
		config: config,
		contract: ValidatorContract{
			slots: StorageSlots{
				election: pad([]byte{2}),
				hashOnion: pad([]byte{3}),
				validSince: pad([]byte{4}),
			},
			addr: common.HexToAddress("0x0000000000000000000000000000000000000020"),
			schedule: Schedule {
				period: weeksToSeconds(4),
			},
		}
	}
}

func (p *Panarchy) LoadHashOnion() error {
	filePath := p.config.HashOnionFilePath
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v, hashOnionFilePath: %v", err, configFile)
	}
	defer file.Close()
	
	err = json.NewDecoder(file).Decode(&p.hashOnion)
	if err != nil {
		return fmt.Errorf("error decoding JSON: %v", err)
	}
	return nil
}

func (p *Panarchy) Authorize(signer common.Address, signFn SignerFn) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.signer = signer
	c.signFn = signFn
	if err := p.LoadHashOnion(); err != nil {
		log.Error("LoadHashOnion error:", err)
	}
}

func (p *Panarchy) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return p.verifyHeader(chain, header)
}
func (p *Panarchy) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := p.verifyHeader(chain, header)

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

func (p *Panarchy) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	if header.Time > uint64(time.Now().Unix()) {
		return consensus.ErrFutureBlock
	}
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)

	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	if parent.Time+p.config.Period > header.Time {
		return errInvalidTimestamp
	}
	if header.GasLimit > vars.MaxGasLimit {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, vars.MaxGasLimit)
	}
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	return nil
}

func (p *Panarchy) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (p *Panarchy) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = parent.Time + p.config.Period
	if header.Time < uint64(time.Now().Unix()) {
		header.Time = uint64(time.Now().Unix())
	}
	return nil
}

func (p *Panarchy) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, withdrawals []*types.Withdrawal) {
}


func (p *Panarchy) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt, withdrawals []*types.Withdrawal) (*types.Block, error) {
	return nil
}

func (p *Panarchy) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return nil
}
func (p *Panarchy) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil {
		panic("unexpected withdrawal hash value in panarchy")
	}
	rlp.Encode(hasher, enc)
	hasher.Sum(hash[:0])
	return hash
}
func (p *Panarchy) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return nil
}
func (p *Panarchy) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{}
}
func (p *Panarchy) Close() error {
	return nil
}
