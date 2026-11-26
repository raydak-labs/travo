package api

import (
	"encoding/binary"
	"io"
)

// counterEntropy is deterministic and unique per read (an 8-byte counter, zero padded).
type counterEntropy struct{ n uint64 }

func (c *counterEntropy) Read(p []byte) (int, error) {
	c.n++
	clear(p)
	binary.BigEndian.PutUint64(p, c.n)
	return len(p), nil
}

func zeroEntropy() io.Reader { return &counterEntropy{} }
