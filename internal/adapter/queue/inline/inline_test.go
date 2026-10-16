// SPDX-License-Identifier: AGPL-3.0-only

package inline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/queue/inline"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/pkg/testkit"
)

func factory(maxDelay time.Duration) testkit.QueueFactory {
	return func(_ *testing.T, clock *testkit.Clock) testkit.QueueHarness {
		q := inline.New(inline.Options{Clock: clock, IDs: testkit.NewIDGenerator(clock), MaxDelay: maxDelay})
		return testkit.QueueHarness{Queue: q, Source: q, DeadJobs: q, MaxDelay: maxDelay}
	}
}

func TestQueueContract(t *testing.T) {
	testkit.QueueContract(t, factory(0))
}

// TestQueueContractWithDelayLimit emulates the 15-minute delay limit of SQS, which exercises
// the re-enqueue chain.
func TestQueueContractWithDelayLimit(t *testing.T) {
	testkit.QueueContract(t, factory(15*time.Minute))
}

func TestRunSynchronously(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	q := inline.New(inline.Options{Clock: clock, IDs: testkit.NewIDGenerator(clock)})
	var reg queue.JobHandlerRegistry
	var ran []string
	reg.Register("parent", func(ctx context.Context, _ queue.Job) queue.Result {
		ran = append(ran, "parent")
		// Enqueuing from a handler joins the running drain instead of recursing.
		if err := q.Enqueue(ctx, queue.NewJob{Type: "child"}); err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
	reg.Register("child", func(context.Context, queue.Job) queue.Result {
		ran = append(ran, "child")
		return queue.Done()
	})
	q.RunSynchronously(queue.NewPullRunner(q, &reg, queue.RunnerOptions{Clock: clock}))

	later := queue.NewJob{Type: "child", Payload: json.RawMessage(`{}`), RunAt: clock.Now().Add(time.Minute)}
	if err := q.Enqueue(t.Context(), queue.NewJob{Type: "parent"}, later); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(ran) != 2 || ran[0] != "parent" || ran[1] != "child" {
		t.Fatalf("Enqueue ran %v, want [parent child] before returning", ran)
	}
	clock.Advance(time.Minute)
	if err := q.Enqueue(t.Context()); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(ran) != 3 {
		t.Fatalf("ran %v, want the delayed child once it is due", ran)
	}
}
