package core

import (
	"math/big"
	"encoding/binary"
	
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
    slotOne 		= []byte{31: 1}
    slotTwo 		= []byte{31: 2}
    addressOne		= common.Address{19: 1}
    addressTwo		= common.Address{19: 2}
)

const (
	compensateForPossibleReorg = 20
)

type Config struct {
	Period uint64 `json:"period"`
	Deadline  uint64 `json:"deadline"`
}

type schedule struct {
	genesis uint64
	period uint64
}

type Panarchy struct {
	config Config
	schedule schedule
}

func New() *Panarchy {
	return &Panarchy{
		config: Config{
			Period: 12,
			Deadline: 12,
		},
		schedule: Schedule {
			genesis: 1709960400,
			period: 4*7*24*60*60,
		},
	}
}

func (p *Panarchy) schedule(timestamp *big.Int) []byte {
	schedule := new(big.Int).Div(new(big.Int).Sub(timestamp, p.schedule.genesis), p.schedule.period)
	index := common.LeftPadBytes(schedule.Bytes(), 32)
}
func electionLength(index []byte, state *state.StateDB) *big.Int {
	lengthKey := crypto.Keccak256Hash(append(index, slotTwo...))
	electionLength := state.GetState(addressTwo, lengthKey)
	return new(big.Int).SetBytes(electionLength.Bytes())	
}
func isValidator(index []byte, electionLength *big.Int, random *big.Int, skipped *big.Int, state *state.StateDB) common.Address {
	randomVoter := new(big.Int).Add(random, skipped)
	randomVoter.Mod(randomVoter, electionLength)
	key := new(big.Int).SetBytes(crypto.Keccak256(crypto.Keccak256(append(index, slotTwo...))))
	key.Add(key, randomVoter)
	return state.GetState(addressTwo, common.BytesToHash(key.Bytes()))
}
func (p *Panarchy) getValidator(block *types.Block, state *state.StateDB) common.Address {
	index := p.schedule(block.Time())
	electionLength := electionLength(index, state)
	return isValidator(index, electionLength, block.Random(), block.Skipped(), state)
}

func writeHashToContract (preimage []byte, validator common.Address, state *state.StateDB) {
	validatorPadded := common.LeftPadBytes(validator.Bytes(), 32)
	hashOnion := crypto.Keccak256Hash(append(validatorPadded, slotOne...))
	self.current.state.SetState(addressOne, hashOnion, common.BytesToHash(preimage))
}

func hashOnionFromStorageOrNew(validator common.Address, blockNumber *big.Int, state *state.StateDB) common.Hash {
	validatorPadded := common.LeftPadBytes(validator.Bytes(), 32)
	pending := crypto.Keccak256(append(validatorPadded, slotTwo...))
	validSinceField := new(big.Int).Add(new(big.Int).SetBytes(pending), common.Big1)
	key := common.BytesToHash(validSinceField.Bytes())
	data := state.GetState(addressOne, key)
	validSince := new(big.Int).SetBytes(data.Bytes())
	if validSince.Cmp(common.Big0) == 0 || blockNumber.Cmp(validSince) < 0 {
		hashOnion := crypto.Keccak256Hash(append(validatorPadded, slotOne...))
		return state.GetState(addressOne, hashOnion)
	} else {
		hash := state.GetState(addressOne, common.BytesToHash(pending))
		state.SetState(addressOne, common.BytesToHash(pending), common.Hash{})
		state.SetState(addressOne, common.BytesToHash(validSinceField.Bytes()), common.Hash{})
		return hash
	}
}
