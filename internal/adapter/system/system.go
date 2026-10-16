// SPDX-License-Identifier: AGPL-3.0-only

// Package system implements the Clock and IDGenerator ports of internal/app/kernel with
// the operating system's clock and the standard library's UUIDv7 generator.
package system

import (
	"crypto/rand"
	"time"
	"uuid"

	"github.com/best-friends/my/internal/app/kernel"
)

// Clock reads the system clock.
type Clock struct{}

var _ kernel.Clock = Clock{}

// Now returns the current system time in UTC, truncated to milliseconds.
func (Clock) Now() time.Time {
	return kernel.Normalize(time.Now())
}

// IDGenerator issues UUIDv7s with uuid.NewV7, which orders IDs within the same millisecond
// by a sub-millisecond counter, and fills the random bits of IDs for a given time from crypto/rand.
type IDGenerator struct{}

var _ kernel.IDGenerator = IDGenerator{}

// NewID returns a new UUIDv7 in canonical form.
func (IDGenerator) NewID() string {
	return uuid.NewV7().String()
}

// NewIDAt returns a new UUIDv7 in canonical form whose timestamp is at's millisecond, or the
// current system time's when at is in the future, and whose 74 random bits come from crypto/rand.
func (IDGenerator) NewIDAt(at time.Time) string {
	if now := time.Now(); at.After(now) {
		at = now
	}
	u := uuid.MustParse(kernel.UUIDv7Floor(at))
	var r [10]byte
	_, _ = rand.Read(r[:]) // crypto/rand.Read never returns an error
	// The floor's rand_a and rand_b are zero: fill rand_a (the 12 bits after the version
	// nibble) and rand_b (the 62 bits after the variant bits).
	u[6] |= r[0] & 0x0f
	u[7] = r[1]
	u[8] |= r[2] & 0x3f
	copy(u[9:], r[3:])
	return u.String()
}
