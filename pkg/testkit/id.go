// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"encoding/binary"
	"sync"
	"time"
	"uuid"

	"github.com/best-friends/my/internal/app/kernel"
)

// IDGenerator is a deterministic kernel.IDGenerator. Each ID is a valid UUIDv7 whose timestamp
// is the clock's current millisecond and whose random bits hold a sequence number starting at 1,
// so the same sequence of clock readings always yields the same IDs, in increasing order.
// NewIDAt stamps the given time instead (rounded down to the clock's time when later) and draws
// from the same sequence, so its IDs are deterministic and unique too.
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

// NewID returns the next ID, stamped with the clock's time.
func (g *IDGenerator) NewID() string {
	seq, now := g.next()
	return stamp(now, seq)
}

// NewIDAt returns the next ID, stamped with at, or with the clock's time when at is later.
func (g *IDGenerator) NewIDAt(at time.Time) string {
	seq, now := g.next()
	if at.After(now) {
		at = now
	}
	return stamp(at, seq)
}

// next advances the sequence and reads the clock.
func (g *IDGenerator) next() (uint64, time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return g.seq, g.clock.Now()
}

// stamp returns the UUIDv7 with t's millisecond as its timestamp and seq in its random bits.
func stamp(t time.Time, seq uint64) string {
	u := uuid.MustParse(kernel.UUIDv7Floor(t))
	// Keep the variant bits (0b10) of byte 8 and put the sequence number in rand_b.
	binary.BigEndian.PutUint64(u[8:16], 1<<63|seq&seqMask)
	return u.String()
}
