// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"encoding/binary"
	"sync"
	"uuid"

	"github.com/best-friends/my/internal/app/kernel"
)

// IDGenerator is a deterministic kernel.IDGenerator. Each ID is a valid UUIDv7 whose timestamp
// is the clock's current millisecond and whose random bits hold a sequence number starting at 1,
// so the same sequence of clock readings always yields the same IDs, in increasing order.
// It is safe for concurrent use.
type IDGenerator struct {
	clock kernel.Clock

	mu  sync.Mutex
	seq uint64
}

var _ kernel.IDGenerator = (*IDGenerator)(nil)

// seqMask keeps the sequence number within the 62 bits of rand_b, below the variant bits.
const seqMask = 1<<62 - 1

// NewIDGenerator returns an IDGenerator that stamps IDs with clock's time.
func NewIDGenerator(clock kernel.Clock) *IDGenerator {
	return &IDGenerator{clock: clock}
}

// NewID returns the next ID.
func (g *IDGenerator) NewID() string {
	g.mu.Lock()
	g.seq++
	seq := g.seq
	now := g.clock.Now()
	g.mu.Unlock()

	u := uuid.MustParse(kernel.UUIDv7Floor(now))
	// Keep the variant bits (0b10) of byte 8 and put the sequence number in rand_b.
	binary.BigEndian.PutUint64(u[8:16], 1<<63|seq&seqMask)
	return u.String()
}
