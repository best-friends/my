// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"sync"
	"time"

	"github.com/mycroblog/my/internal/app/kernel"
)

// Clock is a deterministic kernel.Clock. It stands still until the test moves it with
// Advance or Set. It is safe for concurrent use.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

var _ kernel.Clock = (*Clock)(nil)

// NewClock returns a Clock stopped at start, normalized to UTC milliseconds.
func NewClock(start time.Time) *Clock {
	return &Clock{now: kernel.Normalize(start)}
}

// Now returns the clock's current time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d (backward if d is negative) and returns the new time.
// The result is truncated to milliseconds, so durations below a millisecond are lost.
func (c *Clock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = kernel.Normalize(c.now.Add(d))
	return c.now
}

// Set moves the clock to t, normalized to UTC milliseconds.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = kernel.Normalize(t)
}
