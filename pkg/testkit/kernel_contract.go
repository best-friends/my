// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"testing"
	"time"
	"uuid"

	"github.com/best-friends/my/internal/app/kernel"
)

// ClockContract verifies that c satisfies the kernel.Clock contract: Now returns UTC times
// with millisecond precision and no monotonic clock reading.
func ClockContract(t *testing.T, c kernel.Clock) {
	t.Helper()
	for range 100 {
		now := c.Now()
		if now.Location() != time.UTC {
			t.Fatalf("Now() = %v, want location UTC", now)
		}
		if now.Nanosecond()%int(time.Millisecond) != 0 {
			t.Fatalf("Now() = %v, want millisecond precision", now)
		}
		if now != now.Round(0) {
			t.Fatalf("Now() = %v carries a monotonic clock reading", now)
		}
	}
}

// IDGeneratorContract verifies that g satisfies the kernel.IDGenerator contract: NewID returns
// canonical UUIDv7 strings that strictly increase from call to call, including calls within
// the same millisecond. It returns the generated IDs for further checks.
func IDGeneratorContract(t *testing.T, g kernel.IDGenerator) []string {
	t.Helper()
	const n = 10_000
	ids := make([]string, n)
	for i := range ids {
		ids[i] = g.NewID()
	}
	prev := ""
	for _, id := range ids {
		u, err := uuid.Parse(id)
		if err != nil {
			t.Fatalf("NewID() = %q is not a UUID: %v", id, err)
		}
		if u.String() != id {
			t.Fatalf("NewID() = %q is not canonical, want %q", id, u.String())
		}
		if version := u[6] >> 4; version != 7 {
			t.Fatalf("NewID() = %q has version %d, want 7", id, version)
		}
		if variant := u[8] >> 6; variant != 0b10 {
			t.Fatalf("NewID() = %q has variant %b, want 10", id, variant)
		}
		if id <= prev {
			t.Fatalf("NewID() = %q after %q, want strictly increasing IDs", id, prev)
		}
		prev = id
	}
	return ids
}

// UUIDv7Millis returns the unix_ts_ms field of a canonical UUIDv7 string.
// It returns false if id is not a UUID.
func UUIDv7Millis(id string) (int64, bool) {
	u, err := uuid.Parse(id)
	if err != nil {
		return 0, false
	}
	var ms int64
	for _, b := range u[:6] {
		ms = ms<<8 | int64(b)
	}
	return ms, true
}
