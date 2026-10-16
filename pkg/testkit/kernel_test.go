// SPDX-License-Identifier: AGPL-3.0-only

package testkit_test

import (
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/pkg/testkit"
)

var start = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func TestClockContract(t *testing.T) {
	testkit.ClockContract(t, testkit.NewClock(start))
}

func TestKeyManagerContract(t *testing.T) {
	testkit.KeyManagerContract(t, testkit.NewKeyManager("test"))
}

func TestClockStandsStillUntilMoved(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	c := testkit.NewClock(start.In(jst).Add(123_456 * time.Nanosecond))

	if got := c.Now(); got != start {
		t.Fatalf("NewClock: Now() = %v, want %v", got, start)
	}
	if got := c.Now(); got != start {
		t.Fatalf("Now() moved on its own to %v", got)
	}
	if got, want := c.Advance(1500*time.Microsecond), start.Add(time.Millisecond); got != want || c.Now() != want {
		t.Fatalf("Advance: got %v, Now() = %v, want %v", got, c.Now(), want)
	}
	if got, want := c.Advance(-time.Hour), start.Add(time.Millisecond-time.Hour); got != want {
		t.Fatalf("Advance backwards: got %v, want %v", got, want)
	}
	c.Set(start.Add(24 * time.Hour).In(jst))
	if got, want := c.Now(), start.Add(24*time.Hour); got != want {
		t.Fatalf("Set: Now() = %v, want %v", got, want)
	}
}

func TestIDGeneratorContract(t *testing.T) {
	// A stopped clock puts every ID in the same millisecond.
	ids := testkit.IDGeneratorContract(t, testkit.NewIDGenerator(testkit.NewClock(start)))
	for _, id := range ids {
		if ms, _ := testkit.UUIDv7Millis(id); ms != start.UnixMilli() {
			t.Fatalf("ID %q has timestamp %d, want %d", id, ms, start.UnixMilli())
		}
	}
}

func TestIDGeneratorIsDeterministic(t *testing.T) {
	run := func() []string {
		c := testkit.NewClock(start)
		g := testkit.NewIDGenerator(c)
		ids := []string{g.NewID(), g.NewID()}
		c.Advance(time.Second)
		return append(ids, g.NewID())
	}
	want := []string{
		"01a14495-5600-7000-8000-000000000001",
		"01a14495-5600-7000-8000-000000000002",
		"01a14495-59e8-7000-8000-000000000003",
	}
	for range 2 {
		got := run()
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("IDs = %q, want %q", got, want)
			}
		}
	}
}

func TestIDGeneratorStampsClockTime(t *testing.T) {
	c := testkit.NewClock(start)
	g := testkit.NewIDGenerator(c)
	c.Advance(90 * time.Minute)
	id := g.NewID()
	if ms, _ := testkit.UUIDv7Millis(id); ms != c.Now().UnixMilli() {
		t.Errorf("ID %q has timestamp %d, want %d", id, ms, c.Now().UnixMilli())
	}
	if floor := kernel.UUIDv7Floor(c.Now()); id <= floor {
		t.Errorf("ID %q does not sort above its millisecond's floor %q", id, floor)
	}
}

func TestIDGeneratorConcurrentIDsAreUnique(t *testing.T) {
	g := testkit.NewIDGenerator(testkit.NewClock(start))
	const workers, perWorker = 8, 1000
	results := make(chan string, workers*perWorker)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				results <- g.NewID()
			}
		})
	}
	wg.Wait()
	close(results)
	seen := make(map[string]bool, workers*perWorker)
	for id := range results {
		if seen[id] {
			t.Fatalf("duplicate ID %q", id)
		}
		seen[id] = true
	}
}
