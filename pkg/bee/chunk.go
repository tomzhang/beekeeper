package bee

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"math/rand"

	"github.com/ethersphere/bee/pkg/cac"
	"github.com/ethersphere/bee/pkg/swarm"
	bmtlegacy "github.com/ethersphere/bmt/legacy"
	"golang.org/x/crypto/sha3"
)

const (
	// MaxChunkSize represents max chunk size in bytes
	MaxChunkSize = 4096
	spanInfoSize = 8
)

// Chunk represents Bee chunk
type Chunk struct {
	address swarm.Address
	data    []byte
	span    int
}

// NewRandomChunk returns new pseudorandom chunk
func NewRandomChunk(r *rand.Rand) (Chunk, error) {
	data := make([]byte, r.Intn(MaxChunkSize))
	if _, err := r.Read(data); err != nil {
		return Chunk{}, fmt.Errorf("create random chunk: %w", err)
	}

	span := len(data)
	b := make([]byte, spanInfoSize)
	binary.LittleEndian.PutUint64(b, uint64(span))
	data = append(b, data...)

	c := Chunk{data: data, span: span}
	err := c.SetAddress()
	return c, err
}

// Address returns chunk's address
func (c *Chunk) Address() swarm.Address {
	return c.address
}

// Data returns chunk's data
func (c *Chunk) Data() []byte {
	return c.data
}

// Size returns chunk size
func (c *Chunk) Size() int {
	return len(c.data)
}

// Span returns chunk span
func (c *Chunk) Span() int {
	return c.span
}

// SetAddress calculates the address of a chunk and assign's it to address field
func (c *Chunk) SetAddress() error {
	p := bmtlegacy.NewTreePool(chunkHahser, swarm.Branches, bmtlegacy.PoolSize)
	hasher := bmtlegacy.New(p)
	err := hasher.SetSpanBytes(c.Data()[:8])
	if err != nil {
		return err
	}
	_, err = hasher.Write(c.Data()[8:])
	if err != nil {
		return err
	}
	c.address = swarm.NewAddress(hasher.Sum(nil))
	return nil
}

// ClosestNode returns chunk's closest node of a given set of nodes
func (c *Chunk) ClosestNode(nodes []swarm.Address) (closest swarm.Address, err error) {
	closest = nodes[0]
	for _, a := range nodes[1:] {
		dcmp, err := swarm.DistanceCmp(c.Address().Bytes(), closest.Bytes(), a.Bytes())
		if err != nil {
			return swarm.Address{}, fmt.Errorf("find closest node: %w", err)
		}
		switch dcmp {
		case 0:
			// do nothing
		case -1:
			// current node is closer
			closest = a
		case 1:
			// closest is already closer to chunk
			// do nothing
		}
	}

	return
}

// ClosestNodeFromMap returns chunk's closest node of a given map of nodes
func (c *Chunk) ClosestNodeFromMap(nodes map[string]swarm.Address, skipNodes ...swarm.Address) (closestName string, closestAddress swarm.Address, err error) {
	for k, v := range nodes {
		fmt.Println(k, v)
	}
	names := make([]string, 0, len(nodes))
	addresses := make([]swarm.Address, 0, len(nodes))
	for k, v := range nodes {
		names = append(names, k)
		addresses = append(addresses, v)
	}

next:
	for i, a := range addresses {

		for _, skip := range skipNodes {
			if a.Equal(skip) {
				continue next
			}
		}

		if closestAddress.IsZero() {
			closestName = names[i]
			closestAddress = a
			continue
		}

		dcmp, err := swarm.DistanceCmp(c.Address().Bytes(), a.Bytes(), closestAddress.Bytes())
		if err != nil {
			return "", swarm.Address{}, fmt.Errorf("find closest node: %w", err)
		}
		if dcmp == 1 {
			closestName = names[i]
			closestAddress = a
		}
	}

	if closestAddress.IsZero() {
		return "", swarm.Address{}, errors.New("closest node not found")
	}

	return
}

func chunkHahser() hash.Hash {
	return sha3.NewLegacyKeccak256()
}

// GenerateRandomChunkAt generates a chunk with address of proximity order po wrt target.
func GenerateRandomChunkAt(rnd *rand.Rand, target swarm.Address, po uint8) swarm.Chunk {
	data := make([]byte, swarm.ChunkSize)
	for {
		_, _ = rnd.Read(data)
		chunk, _ := cac.New(data)
		if swarm.Proximity(chunk.Address().Bytes(), target.Bytes()) == po {
			return chunk
		}
	}
}
