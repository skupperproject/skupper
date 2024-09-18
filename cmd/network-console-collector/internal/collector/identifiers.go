package collector

import (
	"bytes"
	"encoding/hex"
	"hash"
	"hash/fnv"
	"sync"
)

type idProvider interface {
	ID(prefix string, part string, parts ...string) string
}

type hashIDer struct {
	mu   sync.Mutex
	hash hash.Hash
	buff []byte
}

func newStableIdentityProvider() idProvider {
	h := fnv.New64()
	return &hashIDer{
		hash: h,
		buff: make([]byte, 0, h.Size()),
	}
}

func (c *hashIDer) ID(prefix string, part string, parts ...string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hash.Reset()
	c.hash.Write([]byte(prefix))
	c.hash.Write([]byte(part))
	for _, p := range parts {
		c.hash.Write([]byte(p))
	}
	sum := c.hash.Sum(c.buff)
	out := bytes.NewBuffer([]byte(prefix + "-"))
	hex.NewEncoder(out).Write(sum)
	return out.String()
}
