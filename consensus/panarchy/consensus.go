import (
	"sync"

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
		log.Println("LoadHashOnion error:", err)
	}
}
