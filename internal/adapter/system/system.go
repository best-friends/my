// SPDX-License-Identifier: AGPL-3.0-only

// Package system implements the Clock and IDGenerator ports of internal/app/kernel with
// the operating system's clock and the standard library's UUIDv7 generator.
package system

import (
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
// by a sub-millisecond counter.
type IDGenerator struct{}

var _ kernel.IDGenerator = IDGenerator{}

// NewID returns a new UUIDv7 in canonical form.
func (IDGenerator) NewID() string {
	return uuid.NewV7().String()
}
