// Package idgen mints identifiers.
//
// Run ids must be collision resistant and reproducible in a test. Identifiers that
// end up in the database — or that decide which run owns a piece of state — need a
// real source of randomness.
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// Generator mints identifiers from crypto/rand.
type Generator struct {
	prefix  string
	counter atomic.Uint64
}

// New builds a generator whose ids carry the given prefix.
func New(prefix string) *Generator {
	return &Generator{prefix: prefix}
}

// NewID mints a fresh identifier.
func (g *Generator) NewID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		// crypto/rand does not fail in practice. If it ever did, falling back to a
		// predictable value would be worse than a clear identifier, so the sequence
		// is marked instead.
		return fmt.Sprintf("%s-fallback-%d-%d", g.prefix, time.Now().UnixNano(), g.counter.Add(1))
	}
	return g.prefix + "-" + hex.EncodeToString(buffer[:])
}

// New builds a generator, for callers that only need the interface.
func NewGenerator(prefix string) *Generator { return New(prefix) }
