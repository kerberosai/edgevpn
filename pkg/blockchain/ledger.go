package blockchain

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/pkg/errors"
)

type Ledger struct {
	sync.Mutex
	blockchain Blockchain

	maxChainSize int
	channel      io.Writer
}

// New returns a new ledger which writes to the writer
func New(w io.Writer, maxChainSize int) *Ledger {
	c := &Ledger{channel: w, maxChainSize: maxChainSize}
	c.newGenesis()
	return c
}

func (l *Ledger) newGenesis() {
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), map[string]map[string]Data{}, genesisBlock.Checksum(), ""}
	l.blockchain = append(l.blockchain, genesisBlock)
}

// Syncronizer starts a goroutine which
// writes the blockchain to the  periodically
func (l *Ledger) Syncronizer(ctx context.Context, t time.Duration) {
	go func() {
		t := time.NewTicker(t)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.Lock()
				bytes, err := json.Marshal(l.blockchain)
				if err != nil {
					log.Println(err)
				}
				l.channel.Write(bytes)

				// Reset blockchain if we exceed chainsize
				if l.maxChainSize != 0 && len(l.blockchain) > l.maxChainSize {
					l.blockchain = []Block{}
				}
				l.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// String returns the blockchain as string
func (l *Ledger) String() string {
	bytes, _ := json.MarshalIndent(l.blockchain, "", "  ")
	return string(bytes)
}

// Update the blockchain from a message
func (l *Ledger) Update(h *hub.Message) (err error) {
	chain := make(Blockchain, 0)

	err = json.Unmarshal([]byte(h.Message), &chain)
	if err != nil {
		err = errors.Wrap(err, "failed unmarshalling blockchain data")
		return
	}

	l.Lock()
	if chain.IsMoreRecent(l.blockchain) {
		l.blockchain = chain
	}
	l.Unlock()

	return
}

// Announce keeps updating async data to the blockchain.
// Sends a broadcast at the specified interval
// by making sure the async retrieved value is written to the
// blockchain
func (l *Ledger) Announce(ctx context.Context, t time.Duration, async func()) {
	go func() {
		t := time.NewTicker(t)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				async()

			case <-ctx.Done():
				return
			}
		}
	}()
}

func (l *Ledger) lastBlock() Block {
	return (l.blockchain[len(l.blockchain)-1])
}

// GetKey retrieve the current key from the blockchain
func (l *Ledger) GetKey(b, s string) (value Data, exists bool) {
	l.Lock()
	defer l.Unlock()

	if len(l.blockchain) > 0 {
		last := l.lastBlock()
		if _, exists = last.Storage[b]; !exists {
			return
		}
		value, exists = last.Storage[b][s]
		if exists {
			return
		}
	}
	return
}

// ExistsValue returns true if there is one element with a matching value
func (l *Ledger) Exists(b string, f func(Data) bool) (exists bool) {
	l.Lock()
	defer l.Unlock()
	if len(l.blockchain) > 0 {
		for _, bv := range l.lastBlock().Storage[b] {
			if f(bv) {
				exists = true
				return
			}
		}
	}

	return
}

func (l *Ledger) CurrentData() map[string]map[string]Data {
	l.Lock()
	defer l.Unlock()
	return l.lastBlock().Storage
}

func (l *Ledger) BlockChain() Blockchain {
	l.Lock()
	defer l.Unlock()
	return l.blockchain
}

// Add data to the blockchain
func (l *Ledger) Add(b string, s map[string]interface{}) {
	l.Lock()
	current := l.lastBlock().Storage
	for s, k := range s {
		if _, exists := current[b]; !exists {
			current[b] = make(map[string]Data)
		}
		dat, _ := json.Marshal(k)
		current[b][s] = Data(string(dat))
	}
	l.Unlock()
	l.writeData(current)
}

// Deletes data from the blockchain
func (l *Ledger) Delete(b string, k string) {
	l.Lock()
	new := make(map[string]map[string]Data)
	for bb, kk := range l.lastBlock().Storage {
		if _, exists := new[bb]; !exists {
			new[bb] = make(map[string]Data)
		}
		// Copy all keys/v except b/k
		for kkk, v := range kk {
			if !(bb == b && kkk == k) {
				new[bb][kkk] = v
			}
		}
	}
	l.Unlock()
	l.writeData(new)
}

// Deletes data from the blockchain
func (l *Ledger) DeleteBucket(b string) {
	l.Lock()
	new := make(map[string]map[string]Data)
	for bb, kk := range l.lastBlock().Storage {
		// Copy all except the specified bucket
		if bb == b {
			continue
		}
		if _, exists := new[bb]; !exists {
			new[bb] = make(map[string]Data)
		}
		for kkk, v := range kk {
			new[bb][kkk] = v
		}
	}
	l.Unlock()
	l.writeData(new)
}

func (l *Ledger) writeData(s map[string]map[string]Data) {
	newBlock := l.lastBlock().NewBlock(s)

	if newBlock.IsValid(l.lastBlock()) {
		l.Lock()
		l.blockchain = append(l.blockchain, newBlock)
		l.Unlock()
	}

	bytes, err := json.Marshal(l.blockchain)
	if err != nil {
		log.Println(err)
	}

	l.channel.Write(bytes)
}
