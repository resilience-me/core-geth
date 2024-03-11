package panarchy

import (
	"sync"
	"errors"
	"fmt"
	"time"
	"encoding/json"
	"os"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/vars"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/crypto"

	"golang.org/x/crypto/sha3"
)

const errorPrefix = "consensus engine - "

var (
	errInvalidTimestamp = errors.New("invalid timestamp")
	errMissingExtraData = errors.New("extra-data length is wrong")
)

type StorageSlots struct {
	election []byte
	hashOnion []byte
	validSince []byte
}
type Schedule struct {
	genesis uint64
	period uint64
}
type ValidatorContract struct {
	slots StorageSlots
	addr common.Address
	schedule Schedule
}

type HashOnion struct {
	Root common.Hash `json:"root"`
	Layers int `json:"layers"`
}

type Panarchy struct {
	config	*ctypes.PanarchyConfig
	trie state.Trie
	contract ValidatorContract
	hashOnion HashOnion
	lock sync.RWMutex
	signer common.Address
	signFn SignerFn
}

func pad(val []byte) []byte {
	return common.LeftPadBytes(val, 32)
}
func weeksToSeconds(weeks uint64) uint64 {
	return weeks*7*24*60*60
}

type SignerFn func(signer accounts.Account, mimeType string, message []byte) ([]byte, error)

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
		},
	}
}

func (p *Panarchy) LoadHashOnion() error {
	filePath := p.config.HashOnionFilePath
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v, hashOnionFilePath: %v", err, filePath)
	}
	defer file.Close()
	
	err = json.NewDecoder(file).Decode(&p.hashOnion)
	if err != nil {
		return fmt.Errorf("error decoding JSON: %v", err)
	}
	return nil
}

func (p *Panarchy) Authorize(signer common.Address, signFn SignerFn) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.signer = signer
	p.signFn = signFn
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
		for _, header := range headers {
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

	return p.finalizeAndAssemble(chain, header, state)
}

func (p *Panarchy) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return nil
}
func (p *Panarchy) SealHash(header *types.Header) (hash common.Hash) {
	return sealHash(header, true)
}

func sealHash(header *types.Header, earlySealHash bool) (hash common.Hash) {
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
	if earlySealHash == false {
		enc = append(enc, header.Difficulty)
		enc = append(enc, header.Extra[:len(header.Extra)-crypto.SignatureLength])
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

func (p *Panarchy) Author(header *types.Header) (common.Address, error) {
	return ecrecover(header)
}

func extraDataLength(header *types.Header) error {
	if len(header.Extra) != 129 {
	        return errMissingExtraData
	}
	return nil
}

func ecrecover(header *types.Header) (common.Address, error) {

	if err := extraDataLength(header); err != nil {
		return common.Address{}, err
	}
	if len(header.Extra) != 129 {
		return common.Address{}, errMissingExtraData
	}
	signature := header.Extra[:65]

	pubkey, err := crypto.Ecrecover(sealHash(header, false).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	return signer, nil
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

type hashOnion struct {
    hash	common.Hash
    validSince	*big.Int
}

func (p *Panarchy) getHashOnionFromContract(state *state.StateDB) common.Hash {
	addrAndSlot := append(pad(p.signer.Bytes()), p.contract.slots.hashOnion...)
	key := crypto.Keccak256Hash(addrAndSlot)
	return state.GetState(p.contract.addr, key)
}

func (p *Panarchy) finalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) (*types.Block, error) {

	addrAndSlot := append(pad(p.signer.Bytes()), p.contract.slots.validSince...)
	key := crypto.Keccak256Hash(addrAndSlot)
	data := state.GetState(p.contract.addr, key)
	validSince := new(big.Int).SetBytes(data.Bytes())

	parentHeader := chain.GetHeaderByHash(header.ParentHash)
	
	trieRoot, err := getTrieRoot(parentHeader)
	if err != nil {
		return nil, err
	}
	if err := p.openTrie(trieRoot, state); err != nil {
		return nil, err
	}
	onion, err := p.getHashOnion(p.signer.Bytes())
	if err != nil {
		return nil, err
	}

	hashOnion := onion.hash
	 
	if header.Number.Cmp(validSince) >= 0 {
		
		if onion.validSince == nil || onion.validSince.Cmp(validSince) < 0 {
			hashOnion = p.getHashOnionFromContract(state)
		}
	}
	log.Info(p.getHashOnionFromContract(state).Hex())
	if p.hashOnion.Layers <= 0 {
		return nil, fmt.Errorf("Validator hash onion is empty")
	}
	root := p.hashOnion.Root.Bytes()
	for i := 0; i < p.hashOnion.Layers; i++ {
		root = crypto.Keccak256(root)
	}
	hashOnionLocal := common.BytesToHash(root)
	
	if hashOnion == common.BytesToHash(root) {
	}

	return nil, nil
}

func (p *Panarchy) openTrie(trieRoot common.Hash, state *state.StateDB) error {
	var err error
	p.trie, err = state.Database().OpenTrie(trieRoot)
	if err != nil {
		return fmt.Errorf(errorPrefix + "open trie failed: %w", err)
	}
	return nil
}

func getTrieRoot(header *types.Header) (common.Hash, error) {
	if header.Number.Cmp(common.Big0) == 0 {
		return common.Hash{}, nil
	}
	if err := extraDataLength(header); err != nil {
		return common.Hash{}, err
	}
	return common.BytesToHash(header.Extra[97: 129]), nil
}

func (p *Panarchy) getStorage(key []byte) ([]byte, error) {
	value, err := p.trie.GetStorage(common.Address{}, key)
	if err != nil {
		return nil, fmt.Errorf(errorPrefix + "get storage failed: %w", err)
	}
	return value, nil
}

func (p *Panarchy) getHashOnion(key []byte) (hashOnion, error) {
	value, err := p.getStorage(key)
	if err != nil {
		return hashOnion{}, fmt.Errorf(errorPrefix + "get hash onion failed: %w", err)
	}
	if len(value) == 0 {
		return hashOnion{}, nil
	}
	var hash hashOnion
	if err := rlp.DecodeBytes(value, &hash); err != nil {
		return hashOnion{}, fmt.Errorf(errorPrefix + "decode hash onion failed: %w", err)
	}
	return hash, nil
}
