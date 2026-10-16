// SPDX-License-Identifier: AGPL-3.0-only

package kernel_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/best-friends/my/internal/app/kernel"
)

func TestUUIDv7Floor(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"epoch", time.UnixMilli(0), "00000000-0000-7000-8000-000000000000"},
		{"before epoch", time.UnixMilli(-1), "00000000-0000-7000-8000-000000000000"},
		{"known", time.UnixMilli(0x0123_4567_89ab), "01234567-89ab-7000-8000-000000000000"},
		{"sub-millisecond is truncated", time.UnixMilli(0x0123_4567_89ab).Add(999 * time.Microsecond), "01234567-89ab-7000-8000-000000000000"},
		{"beyond 48 bits", time.UnixMilli(1 << 50), "ffffffff-ffff-7000-8000-000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kernel.UUIDv7Floor(tt.t)
			if got != tt.want {
				t.Errorf("UUIDv7Floor(%v) = %q, want %q", tt.t, got, tt.want)
			}
			if _, err := uuid.Parse(got); err != nil {
				t.Errorf("UUIDv7Floor(%v) = %q is not a UUID: %v", tt.t, got, err)
			}
		})
	}
}

func TestUUIDv7FloorBoundsGeneratedIDs(t *testing.T) {
	before := time.Now()
	id := uuid.NewV7().String()
	after := time.Now()

	if floor := kernel.UUIDv7Floor(before); id < floor {
		t.Errorf("id %q generated after %v sorts below its floor %q", id, before, floor)
	}
	if floor := kernel.UUIDv7Floor(after.Add(time.Millisecond)); id >= floor {
		t.Errorf("id %q generated before %v sorts at or above the next millisecond's floor %q", id, after, floor)
	}
}

func TestNormalize(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	in := time.Date(2026, 10, 16, 9, 0, 0, 123_456_789, jst)
	got := kernel.Normalize(in)
	want := time.Date(2026, 10, 16, 0, 0, 0, 123_000_000, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Normalize(%v) = %v, want %v", in, got, want)
	}
	// A normalized time carries no monotonic reading, so == agrees with Equal.
	if n := kernel.Normalize(time.Now()); n != kernel.Normalize(n) {
		t.Errorf("Normalize is not idempotent for %v", n)
	}
}
