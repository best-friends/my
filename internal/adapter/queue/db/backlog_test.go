// SPDX-License-Identifier: AGPL-3.0-only

package db_test

import (
	"maps"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/pkg/testkit"
)

var backlogStart = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// testBacklog checks queue.BacklogReader: it counts the due jobs that no worker holds by
// type, with the earliest due time, and the dead jobs.
func testBacklog(t *testing.T, newHarness testkit.QueueFactory) {
	t.Helper()
	clock := testkit.NewClock(backlogStart)
	h := newHarness(t, clock)
	reader, ok := h.Queue.(queue.BacklogReader)
	if !ok {
		t.Fatalf("%T is not a queue.BacklogReader", h.Queue)
	}
	ctx := t.Context()

	b, err := reader.Backlog(ctx, clock.Now())
	if err != nil {
		t.Fatalf("Backlog: %v", err)
	}
	if len(b.Ready) != 0 || !b.OldestReadyAt.IsZero() || b.Dead != 0 {
		t.Fatalf("empty queue: Backlog = %+v", b)
	}

	if err := h.Queue.Enqueue(ctx,
		queue.NewJob{Type: "test.a"}, queue.NewJob{Type: "test.a"}, queue.NewJob{Type: "test.b"},
		queue.NewJob{Type: "test.c", RunAt: backlogStart.Add(time.Hour)},
	); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	clock.Advance(10 * time.Second)

	// One job dies, and a worker holds another.
	dead, ok, err := h.Source.Claim(ctx, "w1", clock.Now(), time.Minute)
	if err != nil || !ok {
		t.Fatalf("Claim = %v, %v", ok, err)
	}
	if err := h.Source.Bury(ctx, "w1", dead, "boom", clock.Now()); err != nil {
		t.Fatalf("Bury: %v", err)
	}
	held, ok, err := h.Source.Claim(ctx, "w2", clock.Now(), time.Minute)
	if err != nil || !ok {
		t.Fatalf("Claim = %v, %v", ok, err)
	}

	want := map[string]int64{"test.a": 2, "test.b": 1}
	want[dead.Type]--
	want[held.Type]--
	maps.DeleteFunc(want, func(_ string, n int64) bool { return n == 0 })
	b, err = reader.Backlog(ctx, clock.Now())
	if err != nil {
		t.Fatalf("Backlog: %v", err)
	}
	if !maps.Equal(b.Ready, want) || !b.OldestReadyAt.Equal(backlogStart) || b.Dead != 1 {
		t.Fatalf("Backlog = %+v, want Ready %v, OldestReadyAt %v, Dead 1", b, want, backlogStart)
	}

	// Once the lease expires, the held job is ready again.
	clock.Advance(2 * time.Minute)
	want[held.Type]++
	b, err = reader.Backlog(ctx, clock.Now())
	if err != nil {
		t.Fatalf("Backlog: %v", err)
	}
	if !maps.Equal(b.Ready, want) {
		t.Fatalf("after the lease: Ready = %v, want %v", b.Ready, want)
	}
}
