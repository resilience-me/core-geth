import (

	"github.com/ethereum/go-ethereum/common"
)

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
	config *ctypes.PanarchyConfig
  trie Trie
  contract ValidatorContract
  hashOnion HashOnion
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
