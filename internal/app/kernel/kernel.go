// SPDX-License-Identifier: AGPL-3.0-only

// Package kernel holds the ports and helpers shared by every module's application layer:
// the Clock and IDGenerator ports and UUIDv7 helpers.
// Use cases obtain time and IDs only through these ports and pass plain values
// (time.Time, ID strings) to the domain, so that tests stay deterministic (ADR-0015).
// The production implementations live in internal/adapter/system; deterministic fakes live in pkg/testkit.
package kernel

import "time"

// Clock tells the current time.
//
// Now returns the current time in UTC, truncated to millisecond precision
// (the precision every database dialect stores, ADR-0003), without a monotonic clock reading.
type Clock interface {
	Now() time.Time
}

// IDGenerator issues primary keys.
//
// NewID returns a new UUIDv7 in its canonical form (36 lowercase characters).
// IDs returned by one generator sort in increasing order, also within the same millisecond,
// unless the underlying clock moves backwards.
type IDGenerator interface {
	NewID() string
}

// Normalize converts t to the form Clock.Now returns: UTC, truncated to milliseconds,
// with the monotonic clock reading stripped.
func Normalize(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}
