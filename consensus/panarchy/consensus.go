package panarchy

import (
	"bytes"
	"io"
	"sync"
	"errors"
	"fmt"
	"time"
	"encoding/json"
	"os"
	"math/big"
	"encoding/binary"

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
	"github.com/ethereum/go-ethereum/params/mutations"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/trie/triestate"

	"golang.org/x/crypto/sha3"
)

const (
	errorPrefix = "consensus engine - "
	compensateForPossibleReorg = 10
)

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
	state *state.StateDB
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
				genesis: 1709960400,
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
	mutations.AccumulateRewards(chain.Config(), state, header, uncles)
}

func (p *Panarchy) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	
	go func() {
		var validator common.Address
		header := block.Header()
		parentHeader := chain.GetHeaderByHash(header.ParentHash)
		delay := time.Unix(int64(header.Time), 0).Sub(time.Now())
		var i uint64
		loop:
		for {
			select {
			case <-stop:
				return
			case <-time.After(delay):
				validator = p.getValidator(parentHeader, i);
				if validator == p.signer {
					break loop
				}
				i++
				delay += time.Duration(p.config.Deadline) * time.Second
			}
		}
		header.Difficulty = new(big.Int).Sub(big.NewInt(1), big.NewInt(int64(i)))
		sighash, err := p.signFn(accounts.Account{Address: p.signer}, "", PanarchyRLP(header))
		if err != nil {
			log.Error("Failed to sign the header", "account", p.signer.Hex(), "error", err)
		}

		copy(header.Extra[64:], sighash)

		select {
			case results <- block.WithSeal(header):
			default:
				log.Warn("Sealing result is not read by miner", "sealhash", SealHash(header, false))
		}
	}()

	return nil
}

func uint64ToBytes(value uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, value)
	return buf
}

func (p *Panarchy) schedule(header *types.Header) []byte {
	t := ((header.Time - p.contract.schedule.genesis) / p.contract.schedule.period)
	return pad(uint64ToBytes(t))
}

func (p *Panarchy) electionLength(index []byte) []byte {
	slot := p.contract.slots.election
	key := crypto.Keccak256Hash(append(index, slot...))
	return p.state.GetState(p.contract.addr, key).Bytes()
}

func (p *Panarchy) getValidator(header *types.Header, skipped uint64) common.Address {
	index := p.schedule(header)
	
	trieRoot, err := getTrieRoot(header)
	if err != nil {
		log.Error("Error loading trie root in getValidator:", err)
	}
	random := new(big.Int).SetBytes(trieRoot.Bytes())
	random.Add(random, new(big.Int).SetUint64(skipped))
	modulus := new(big.Int).SetBytes(p.electionLength(index))
	random.Mod(random, modulus)
	
	slot := p.contract.slots.election
	key := new(big.Int).SetBytes(crypto.Keccak256(crypto.Keccak256(append(index, slot...))))
	key.Add(key, random)
	return common.BytesToAddress(p.state.GetState(p.contract.addr, common.BytesToHash(key.Bytes())).Bytes())
}

func (p *Panarchy) SealHash(header *types.Header) (hash common.Hash) {
	return SealHash(header, false)
}
func SealHash(header *types.Header, finalSealHash bool) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header, finalSealHash)
	hasher.(crypto.KeccakState).Read(hash[:])
	return hash
}

func PanarchyRLP(header *types.Header) []byte {
	b := new(bytes.Buffer)
	encodeSigHeader(b, header, true)
	return b.Bytes()
}

func encodeSigHeader(w io.Writer, header *types.Header, finalSealHash bool) {
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
		header.MixDigest,
		header.Nonce,
	}
	if finalSealHash == true {
		enc = append(enc, header.Difficulty)
		enc = append(enc, header.Extra)
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil {
		panic("unexpected withdrawal hash value in panarchy")
	}
	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
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

	pubkey, err := crypto.Ecrecover(SealHash(header, false).Bytes(), signature)
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

type Onion struct {
    Hash	common.Hash
    ValidSince	*big.Int
}

func (p *Panarchy) getHashOnionFromContract(state *state.StateDB) common.Hash {
	addrAndSlot := append(pad(p.signer.Bytes()), p.contract.slots.hashOnion...)
	key := crypto.Keccak256Hash(addrAndSlot)
	return state.GetState(p.contract.addr, key)
}

func (p *Panarchy) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt, withdrawals []*types.Withdrawal) (*types.Block, error) {

	if len(withdrawals) > 0 {
		return nil, errors.New("panarchy does not support withdrawals")
	}
	p.Finalize(chain, header, state, txs, uncles, nil)
	
	p.state = state
	
	if err := p.finalizeAndAssemble(chain, header); err != nil {
		return nil, err
	}
	
	return types.NewBlock(header, txs, uncles, receipts, trie.NewStackTrie(nil)), nil
}

func (p *Panarchy) finalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header) error {

	addrAndSlot := append(pad(p.signer.Bytes()), p.contract.slots.validSince...)
	key := crypto.Keccak256Hash(addrAndSlot)
	data := p.state.GetState(p.contract.addr, key)
	validSince := new(big.Int).SetBytes(data.Bytes())

	parentHeader := chain.GetHeaderByHash(header.ParentHash)
	
	trieRoot, err := getTrieRoot(parentHeader)
	if err != nil {
		return err
	}

	if err := p.openTrie(trieRoot, p.state); err != nil {
		fmt.Printf("Error: %s\n", err)
		return err
	}

	onion, err := p.getHashOnion(p.signer.Bytes())
	if err != nil {
		return err
	}
	 
	if header.Number.Cmp(validSince) >= 0 {
		
		if onion.ValidSince == nil || onion.ValidSince.Cmp(validSince) < 0 {
			onion.Hash = p.getHashOnionFromContract(p.state)
			onion.ValidSince = validSince
		}
	}

	if p.hashOnion.Layers <= 0 {
		return fmt.Errorf("Validator hash onion is empty")
	}
	
	p.getHashOnionPreimage(&onion)
	
	if err := p.updateHashOnion(p.signer.Bytes(), onion); err != nil {
		return err
	}

	header.Extra = make([]byte, 129)
	copy(header.Extra[32:64], onion.Hash.Bytes())

	newTrieRoot, nodes, err := p.trie.Commit(false)

	if err != nil {
		return fmt.Errorf("Commit trie failed", err)
	}

	p.state.Database().TrieDB().Update(newTrieRoot, trieRoot, header.Number.Uint64(), trienode.NewWithNodeSet(nodes), &triestate.Set{})

	copy(header.Extra[:32], newTrieRoot.Bytes())

	header.Root = p.state.IntermediateRoot(true)

	return nil
}

func (p *Panarchy) getHashOnionPreimage(onion *Onion) error {

	preimage := p.hashOnion.Root.Bytes()
	for i := 0; i < p.hashOnion.Layers-1; i++ {
		preimage = crypto.Keccak256(preimage)
	}
	var hash []byte

	for i := 0; i < compensateForPossibleReorg; i++ {
		hash = crypto.Keccak256(preimage)

		if onion.Hash == common.BytesToHash(hash) {
			onion.Hash = common.BytesToHash(preimage)
			if i == 0 {
				p.hashOnion.Layers--
				if err := p.WriteHashOnion(); err != nil {
					return fmt.Errorf("Unable to update %s", p.config.HashOnionFilePath)
				}
			}
			return nil
		}
		preimage = hash
	}
	return fmt.Errorf("Validator hash onion cannot be verified")
}

func (p *Panarchy) WriteHashOnion() error {
    filePath := p.config.HashOnionFilePath
    file, err := os.Create(filePath)
    if err != nil {
        return fmt.Errorf("error creating file: %v, hashOnionFilePath: %v", err, filePath)
    }
    defer file.Close()

    err = json.NewEncoder(file).Encode(p.hashOnion)
    if err != nil {
        return fmt.Errorf("error encoding hashOnion to JSON: %v", err)
    }

    return nil
}


func (p *Panarchy) updateStorage(key, value []byte) error {
	if err := p.trie.UpdateStorage(common.Address{}, key, value); err != nil {
		return fmt.Errorf(errorPrefix + "update storage failed: %w", err)
	}
	return nil
}

func (p *Panarchy) updateHashOnion(key []byte, onion Onion) error {

	encoded, err := rlp.EncodeToBytes(onion)
	if err != nil {
		return fmt.Errorf("Error encoding hashOnion:", err)
	}
	
	if err := p.updateStorage(key, encoded); err != nil {
		return fmt.Errorf(errorPrefix + "get hash onion failed: %w", err)
	}
	return nil
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
	return common.BytesToHash(header.Extra[: 32]), nil
}

func (p *Panarchy) getStorage(key []byte) ([]byte, error) {
	value, err := p.trie.GetStorage(common.Address{}, key)
	if err != nil {
		return nil, fmt.Errorf(errorPrefix + "get storage failed: %w", err)
	}
	return value, nil
}

func (p *Panarchy) getHashOnion(key []byte) (Onion, error) {
	value, err := p.getStorage(key)
	if err != nil {
		return Onion{}, fmt.Errorf(errorPrefix + "get hash onion failed: %w", err)
	}
	if len(value) == 0 {
		return Onion{}, nil
	}
	var hash Onion
	if err := rlp.DecodeBytes(value, &hash); err != nil {
		return Onion{}, fmt.Errorf(errorPrefix + "decode hash onion failed: %w", err)
	}
	return hash, nil
}
