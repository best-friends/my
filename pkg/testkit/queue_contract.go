// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
)

// QueueHarness is one job queue adapter under test, on its own empty storage.
type QueueHarness struct {
	// Queue, Source, and DeadJobs are the adapter's views of the same queue.
	Queue    queue.JobQueue
	Source   queue.Source
	DeadJobs queue.DeadJobs
	// UnitOfWork and Outbox, when both set, enable the transactional enqueue cases: jobs
	// enqueued through Outbox in a UnitOfWork transaction must exist exactly when it commits.
	UnitOfWork uow.UnitOfWork
	Outbox     queue.Outbox
	// Relay, when set, moves the jobs written through Outbox into Queue. The contract calls it
	// before running jobs.
	Relay func(ctx context.Context) error
	// Restart, when set, simulates a process restart: it drops every connection and in-memory
	// state of the adapter and returns a harness on the same storage. Durable adapters set it.
	Restart func(t *testing.T) QueueHarness
	// MaxDelay is the adapter's delay limit, or 0 for none. With a limit, the contract checks
	// the re-enqueue chain.
	MaxDelay time.Duration
}

// QueueFactory returns a harness on new, empty storage whose adapter reads time from clock.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type QueueFactory func(t *testing.T, clock *Clock) QueueHarness

// The contract's runner settings. With FixedRandom(0.5) and Jitter 0.5, the backoff after
// failed run n is BaseDelay * 2^(n-1) * 0.75.
var (
	contractStart  = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	contractLease  = time.Minute
	contractPolicy = queue.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Minute, MaxDelay: time.Hour, Jitter: 0.5}
)

// FixedRandom returns a random source for queue.RunnerOptions.Random that always returns v,
// which makes backoff jitter deterministic.
func FixedRandom(v float64) func() float64 { return func() float64 { return v } }

// QueueContract verifies that the adapter built by newHarness satisfies the job queue
// contract, driven by queue.PullRunner: due jobs run in RunAt order and delayed jobs not
// before RunAt; idempotency keys deduplicate pending jobs; failed runs are retried with
// exponential backoff with jitter or after the requested delay; jobs that fail for good or
// use up their attempts become dead jobs, which can be requeued; a job whose worker stopped
// is claimed again once its lease expires (at-least-once), and the stale worker's result is
// dropped; concurrent workers run each job once; and, where the harness enables them,
// transactional enqueue, survival of a restart, and the re-enqueue chain beyond a delay limit.
func QueueContract(t *testing.T, newHarness QueueFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, newHarness QueueFactory)
	}{
		{"RunsDueJobsInRunAtOrder", queueRunsDueJobsInOrder},
		{"RejectsInvalidJobsAtomically", queueRejectsInvalidJobs},
		{"DeduplicatesPendingJobsByIdempotencyKey", queueDeduplicates},
		{"RetriesWithExponentialBackoffAndJitter", queueRetriesWithBackoff},
		{"RetriesAfterTheRequestedDelay", queueRetriesAfterDelay},
		{"RetriesPanickingHandlers", queueRetriesPanics},
		{"MovesFailedJobsToDeadJobs", queueMovesFailedJobsToDead},
		{"MovesExhaustedJobsToDeadJobs", queueMovesExhaustedJobsToDead},
		{"RetriesJobsWithoutAHandler", queueRetriesUnknownTypes},
		{"RequeuesDeadJobs", queueRequeuesDeadJobs},
		{"ReclaimsJobsOfAStoppedWorker", queueReclaimsAfterLeaseExpiry},
		{"RunsEachJobOnceUnderConcurrentWorkers", queueConcurrentWorkers},
		{"EnqueuesAtomicallyWithTheTransaction", queueTransactionalEnqueue},
		{"KeepsJobsAcrossARestart", queueSurvivesRestart},
		{"ReenqueuesJobsBeyondTheDelayLimit", queueReenqueueChain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newHarness) })
	}
}

// recorder is a handler that records every run and answers its n-th run with results[n-1],
// or with Done when results runs out.
type recorder struct {
	mu      sync.Mutex
	runs    []queue.Job
	results []queue.Result
}

func (r *recorder) handle(_ context.Context, j queue.Job) queue.Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, j)
	if n := len(r.runs); n <= len(r.results) {
		return r.results[n-1]
	}
	return queue.Done()
}

func (r *recorder) snapshot() []queue.Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.runs)
}

type queueFixture struct {
	t     *testing.T
	h     QueueHarness
	clock *Clock
	reg   *queue.JobHandlerRegistry
	rec   *recorder
}

const contractJobType = "testkit.probe"

func newQueueFixture(t *testing.T, newHarness QueueFactory, results ...queue.Result) *queueFixture {
	t.Helper()
	clock := NewClock(contractStart)
	f := &queueFixture{t: t, h: newHarness(t, clock), clock: clock, reg: &queue.JobHandlerRegistry{}, rec: &recorder{results: results}}
	f.reg.Register(contractJobType, f.rec.handle)
	return f
}

func (f *queueFixture) runner(worker string) *queue.PullRunner {
	return queue.NewPullRunner(f.h.Source, f.reg, queue.RunnerOptions{
		Clock:  f.clock,
		Worker: worker,
		Lease:  contractLease,
		Retry:  contractPolicy,
		Random: FixedRandom(0.5),
	})
}

func (f *queueFixture) enqueue(jobs ...queue.NewJob) {
	f.t.Helper()
	if err := f.h.Queue.Enqueue(f.t.Context(), jobs...); err != nil {
		f.t.Fatalf("Enqueue: %v", err)
	}
}

// drain relays the outbox, when the harness has one, and runs every due job with a runner
// named "w1". It returns the number of jobs claimed.
func (f *queueFixture) drain() int {
	f.t.Helper()
	if f.h.Relay != nil {
		if err := f.h.Relay(f.t.Context()); err != nil {
			f.t.Fatalf("Relay: %v", err)
		}
	}
	n, err := f.runner("w1").Drain(f.t.Context())
	if err != nil {
		f.t.Fatalf("Drain: %v", err)
	}
	return n
}

func (f *queueFixture) wantRuns(n int) []queue.Job {
	f.t.Helper()
	runs := f.rec.snapshot()
	if len(runs) != n {
		f.t.Fatalf("handler ran %d times, want %d: %v", len(runs), n, runs)
	}
	return runs
}

func (f *queueFixture) dead() []queue.DeadJob {
	f.t.Helper()
	d, err := f.h.DeadJobs.ListDeadJobs(f.t.Context(), 100)
	if err != nil {
		f.t.Fatalf("ListDeadJobs: %v", err)
	}
	return d
}

func probe(payload string) queue.NewJob {
	return queue.NewJob{Type: contractJobType, Payload: json.RawMessage(payload)}
}

func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func payloadOf(t *testing.T, j queue.Job) string {
	t.Helper()
	var v struct{ N string }
	if err := json.Unmarshal(j.Payload, &v); err != nil {
		t.Fatalf("payload %s: %v", j.Payload, err)
	}
	return v.N
}

func queueRunsDueJobsInOrder(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	later := probe(`{"N": "later", "nested": {"b": [1, 2], "a": null}}`)
	later.RunAt = contractStart.Add(10 * time.Minute)
	later.IdempotencyKey = "key-later"
	later.OrderingKey = "order-1"
	first := probe(`{"N":"first"}`)
	first.RunAt = contractStart.Add(-time.Minute)
	f.enqueue(later, probe(`{"N":"now"}`), first)

	if n := f.drain(); n != 2 {
		t.Fatalf("first drain claimed %d jobs, want 2", n)
	}
	runs := f.wantRuns(2)
	if got := []string{payloadOf(t, runs[0]), payloadOf(t, runs[1])}; !slices.Equal(got, []string{"first", "now"}) {
		t.Fatalf("ran %v, want [first now] (RunAt order)", got)
	}
	if !runs[1].RunAt.Equal(contractStart) || runs[1].Attempt != 1 || runs[1].Type != contractJobType || runs[1].ID == "" {
		t.Errorf("job = %+v, want RunAt %v, Attempt 1, Type %s, and an ID", runs[1], contractStart, contractJobType)
	}

	f.clock.Advance(10*time.Minute - time.Millisecond)
	if n := f.drain(); n != 0 {
		t.Fatalf("drain before RunAt claimed %d jobs, want 0", n)
	}
	f.clock.Advance(time.Millisecond)
	if n := f.drain(); n != 1 {
		t.Fatalf("drain at RunAt claimed %d jobs, want 1", n)
	}
	j := f.wantRuns(3)[2]
	if !sameJSON(j.Payload, later.Payload) {
		t.Errorf("payload = %s, want %s", j.Payload, later.Payload)
	}
	if j.IdempotencyKey != "key-later" || j.OrderingKey != "order-1" || !j.RunAt.Equal(later.RunAt) || !j.CreatedAt.Equal(contractStart) {
		t.Errorf("job = %+v, want the enqueued keys, RunAt %v, and CreatedAt %v", j, later.RunAt, contractStart)
	}
	if n := f.drain(); n != 0 {
		t.Fatalf("completed jobs ran again: drain claimed %d", n)
	}
}

func queueRejectsInvalidJobs(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	for _, bad := range []queue.NewJob{
		{Type: "", Payload: json.RawMessage(`{}`)},
		{Type: contractJobType, Payload: json.RawMessage(`{"unterminated"`)},
	} {
		if err := f.h.Queue.Enqueue(t.Context(), probe(`{"N":"ok"}`), bad); err == nil {
			t.Errorf("Enqueue(%+v) succeeded, want an error", bad)
		}
	}
	f.drain()
	f.wantRuns(0)

	f.enqueue(queue.NewJob{Type: contractJobType})
	f.drain()
	if j := f.wantRuns(1)[0]; string(j.Payload) != "null" {
		t.Errorf("nil payload stored as %s, want null", j.Payload)
	}
}

func queueDeduplicates(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	keyed := func(n, key string) queue.NewJob {
		j := probe(fmt.Sprintf(`{"N":%q}`, n))
		j.IdempotencyKey = key
		return j
	}
	f.enqueue(keyed("a1", "a"), keyed("a2", "a"), keyed("b1", "b"))
	f.enqueue(keyed("a3", "a"), probe(`{"N":"plain"}`), probe(`{"N":"plain"}`))
	f.drain()
	var got []string
	for _, j := range f.wantRuns(4) {
		got = append(got, payloadOf(t, j))
	}
	slices.Sort(got)
	if want := []string{"a1", "b1", "plain", "plain"}; !slices.Equal(got, want) {
		t.Fatalf("ran %v, want %v (one job per pending key, every unkeyed job)", got, want)
	}

	// The key is unique among pending jobs only: once its job finished, it may be used again.
	f.enqueue(keyed("a4", "a"))
	f.drain()
	if j := f.wantRuns(5)[4]; payloadOf(t, j) != "a4" {
		t.Fatalf("ran %s, want a4", j.Payload)
	}
}

func queueRetriesWithBackoff(t *testing.T, newHarness QueueFactory) {
	boom := errors.New("temporarily unavailable")
	f := newQueueFixture(t, newHarness, queue.Retry(0, boom), queue.Retry(0, boom))
	f.enqueue(probe(`{"N":"x"}`))

	// Backoffs: 1m * 0.75 = 45s after run 1, 2m * 0.75 = 90s after run 2.
	for i, wait := range []time.Duration{0, 45 * time.Second, 90 * time.Second} {
		if wait > 0 {
			f.clock.Advance(wait - time.Millisecond)
			if n := f.drain(); n != 0 {
				t.Fatalf("run %d started %v early", i+1, time.Millisecond)
			}
			f.clock.Advance(time.Millisecond)
		}
		if n := f.drain(); n != 1 {
			t.Fatalf("run %d: drain claimed %d jobs, want 1", i+1, n)
		}
	}
	runs := f.wantRuns(3)
	for i, j := range runs {
		if j.Attempt != i+1 {
			t.Errorf("run %d has Attempt %d", i+1, j.Attempt)
		}
	}
	if runs[0].LastError != "" || runs[1].LastError != boom.Error() {
		t.Errorf("LastError = %q, %q, want empty, then %q", runs[0].LastError, runs[1].LastError, boom)
	}
	if d := f.dead(); len(d) != 0 {
		t.Errorf("dead jobs = %v, want none", d)
	}
}

func queueRetriesAfterDelay(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness, queue.Retry(7*time.Hour, nil))
	f.enqueue(probe(`{"N":"x"}`))
	f.drain()
	f.clock.Advance(7*time.Hour - time.Millisecond)
	f.drain()
	f.wantRuns(1)
	f.clock.Advance(time.Millisecond)
	f.drain()
	if j := f.wantRuns(2)[1]; j.Attempt != 2 {
		t.Errorf("retry has Attempt %d, want 2", j.Attempt)
	}
}

func queueRetriesPanics(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	calls := 0
	f.reg.Register("testkit.panic", func(context.Context, queue.Job) queue.Result {
		calls++
		if calls == 1 {
			panic("handler bug")
		}
		return queue.Done()
	})
	f.enqueue(queue.NewJob{Type: "testkit.panic"})
	f.drain()
	f.clock.Advance(contractPolicy.BaseDelay)
	f.drain()
	if calls != 2 {
		t.Fatalf("handler ran %d times, want 2 (a panic retries)", calls)
	}
}

func queueMovesFailedJobsToDead(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness, queue.Fail(errors.New("malformed activity")))
	j := probe(`{"N":"x"}`)
	j.IdempotencyKey = "k"
	f.enqueue(j)
	f.clock.Advance(time.Second)
	f.drain()
	f.wantRuns(1)
	dead := f.dead()
	if len(dead) != 1 {
		t.Fatalf("dead jobs = %v, want 1", dead)
	}
	d := dead[0]
	if d.Type != contractJobType || d.Attempt != 1 || d.LastError != "malformed activity" || d.IdempotencyKey != "k" ||
		!sameJSON(d.Payload, j.Payload) || !d.CreatedAt.Equal(contractStart) || !d.FailedAt.Equal(contractStart.Add(time.Second)) {
		t.Errorf("dead job = %+v", d)
	}
	f.clock.Advance(24 * time.Hour)
	if n := f.drain(); n != 0 {
		t.Fatal("a failed job ran again")
	}
	// A dead job no longer holds its idempotency key.
	f.enqueue(j)
	f.drain()
	f.wantRuns(2)
}

func queueMovesExhaustedJobsToDead(t *testing.T, newHarness QueueFactory) {
	boom := errors.New("remote is down")
	f := newQueueFixture(t, newHarness, queue.Retry(0, boom), queue.Retry(0, boom), queue.Retry(0, boom))
	f.enqueue(probe(`{"N":"x"}`))
	for range contractPolicy.MaxAttempts {
		f.drain()
		f.clock.Advance(contractPolicy.MaxDelay)
	}
	f.wantRuns(contractPolicy.MaxAttempts)
	dead := f.dead()
	if len(dead) != 1 || dead[0].Attempt != contractPolicy.MaxAttempts || !strings.Contains(dead[0].LastError, boom.Error()) {
		t.Fatalf("dead jobs = %+v, want one after %d attempts with error %q", dead, contractPolicy.MaxAttempts, boom)
	}
	if n := f.drain(); n != 0 {
		t.Fatal("an exhausted job ran again")
	}
}

func queueRetriesUnknownTypes(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	f.enqueue(queue.NewJob{Type: "testkit.unknown"})
	for range contractPolicy.MaxAttempts {
		if n := f.drain(); n != 1 {
			t.Fatalf("drain claimed %d jobs, want 1", n)
		}
		f.clock.Advance(contractPolicy.MaxDelay)
	}
	dead := f.dead()
	if len(dead) != 1 || !strings.Contains(dead[0].LastError, "testkit.unknown") {
		t.Fatalf("dead jobs = %+v, want the unknown job, naming its type", dead)
	}
}

func queueRequeuesDeadJobs(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness, queue.Fail(errors.New("bug")))
	f.enqueue(probe(`{"N":"x"}`))
	f.drain()
	dead := f.dead()
	if len(dead) != 1 {
		t.Fatalf("dead jobs = %v, want 1", dead)
	}
	if err := f.h.DeadJobs.RequeueDeadJob(t.Context(), dead[0].ID); err != nil {
		t.Fatalf("RequeueDeadJob: %v", err)
	}
	if d := f.dead(); len(d) != 0 {
		t.Fatalf("dead jobs after requeue = %v, want none", d)
	}
	f.drain()
	if j := f.wantRuns(2)[1]; j.Attempt != 1 || !sameJSON(j.Payload, dead[0].Payload) {
		t.Errorf("requeued job = %+v, want Attempt 1 and the same payload", j)
	}
	if err := f.h.DeadJobs.RequeueDeadJob(t.Context(), dead[0].ID); !errors.Is(err, queue.ErrDeadJobNotFound) {
		t.Errorf("RequeueDeadJob of a requeued job = %v, want ErrDeadJobNotFound", err)
	}
}

func queueReclaimsAfterLeaseExpiry(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	started, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var attempts []int
	f.reg.Register("testkit.slow", func(_ context.Context, j queue.Job) queue.Result {
		mu.Lock()
		attempts = append(attempts, j.Attempt)
		mu.Unlock()
		if j.Attempt == 1 {
			close(started)
			<-release // the first worker hangs until the test lets it go
		}
		return queue.Done()
	})
	f.enqueue(queue.NewJob{Type: "testkit.slow"})

	stale := make(chan error, 1)
	go func() {
		_, err := f.runner("stale").RunOnce(t.Context())
		stale <- err
	}()
	<-started

	other := f.runner("other")
	if ran, err := other.RunOnce(t.Context()); ran || err != nil {
		t.Fatalf("RunOnce during the lease = %v, %v, want no job", ran, err)
	}
	f.clock.Advance(contractLease)
	if ran, err := other.RunOnce(t.Context()); !ran || err != nil {
		t.Fatalf("RunOnce after the lease expired = %v, %v, want the job", ran, err)
	}
	close(release)
	if err := <-stale; !errors.Is(err, queue.ErrLeaseLost) {
		t.Errorf("stale worker's RunOnce = %v, want ErrLeaseLost", err)
	}
	if !slices.Equal(attempts, []int{1, 2}) {
		t.Errorf("attempts = %v, want [1 2]", attempts)
	}
	f.clock.Advance(time.Hour)
	if n := f.drain(); n != 0 {
		t.Errorf("drain claimed %d jobs after completion, want 0", n)
	}
	if d := f.dead(); len(d) != 0 {
		t.Errorf("dead jobs = %v, want none", d)
	}
}

func queueConcurrentWorkers(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	const jobs, workers = 40, 4
	batch := make([]queue.NewJob, jobs)
	for i := range batch {
		batch[i] = probe(fmt.Sprintf(`{"N":"j%02d"}`, i))
	}
	f.enqueue(batch...)
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := range workers {
		r := f.runner(fmt.Sprintf("w%d", w))
		wg.Go(func() {
			_, err := r.Drain(t.Context())
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Drain: %v", err)
		}
	}
	seen := map[string]int{}
	for _, j := range f.rec.snapshot() {
		seen[payloadOf(t, j)]++
	}
	if len(seen) != jobs {
		t.Errorf("ran %d distinct jobs, want %d", len(seen), jobs)
	}
	for n, c := range seen {
		if c != 1 {
			t.Errorf("job %s ran %d times, want once", n, c)
		}
	}
}

func queueTransactionalEnqueue(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	if f.h.UnitOfWork == nil || f.h.Outbox == nil {
		t.Skip("the adapter does not enqueue inside database transactions")
	}
	boom := errors.New("domain write failed")
	err := f.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := f.h.Outbox.EnqueueTx(t.Context(), tx, probe(`{"N":"rolled back"}`)); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx = %v, want %v", err, boom)
	}
	f.drain()
	f.wantRuns(0)

	attempts := 0
	err = f.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		attempts++
		if err := f.h.Outbox.EnqueueTx(t.Context(), tx, probe(`{"N":"committed"}`)); err != nil {
			return err
		}
		if attempts == 1 {
			return uow.MarkRetryable(errors.New("conflict"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	f.drain()
	if j := f.wantRuns(1)[0]; payloadOf(t, j) != "committed" {
		t.Fatalf("ran %s, want the committed job once", j.Payload)
	}

	keyed := probe(`{"N":"keyed"}`)
	keyed.IdempotencyKey = "tx-key"
	for range 2 {
		if err := f.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
			return f.h.Outbox.EnqueueTx(t.Context(), tx, keyed)
		}); err != nil {
			t.Fatalf("Tx = %v", err)
		}
	}
	f.drain()
	f.wantRuns(2)
}

func queueSurvivesRestart(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	if f.h.Restart == nil {
		t.Skip("the harness does not simulate a restart")
	}
	f.enqueue(probe(`{"N":"claimed"}`))
	// A worker claims the job and the process stops before it records a result.
	if _, ok, err := f.h.Source.Claim(t.Context(), "crashed", f.clock.Now(), contractLease); !ok || err != nil {
		t.Fatalf("Claim = %v, %v, want a job", ok, err)
	}
	f.enqueue(probe(`{"N":"waiting"}`))

	f.h = f.h.Restart(t)
	if n := f.drain(); n != 1 {
		t.Fatalf("after the restart, drain claimed %d jobs, want only the unclaimed one", n)
	}
	f.clock.Advance(contractLease)
	if n := f.drain(); n != 1 {
		t.Fatalf("after the lease expired, drain claimed %d jobs, want the crashed worker's job", n)
	}
	runs := f.wantRuns(2)
	if payloadOf(t, runs[0]) != "waiting" || payloadOf(t, runs[1]) != "claimed" || runs[1].Attempt != 2 {
		t.Errorf("runs = %+v, want waiting, then claimed with Attempt 2", runs)
	}
}

func queueReenqueueChain(t *testing.T, newHarness QueueFactory) {
	f := newQueueFixture(t, newHarness)
	limit := f.h.MaxDelay
	if limit <= 0 {
		t.Skip("the adapter has no delay limit")
	}
	j := probe(`{"N":"far"}`)
	j.RunAt = contractStart.Add(2*limit + limit/2)
	f.enqueue(j)

	for hop := 1; hop <= 2; hop++ {
		f.clock.Advance(limit - time.Millisecond)
		if n := f.drain(); n != 0 {
			t.Fatalf("hop %d: the job became visible before the delay limit", hop)
		}
		f.clock.Advance(time.Millisecond)
		if n := f.drain(); n != 1 {
			t.Fatalf("hop %d: drain claimed %d jobs, want the job to be re-enqueued", hop, n)
		}
		f.wantRuns(0)
	}
	f.clock.Advance(limit/2 - time.Millisecond)
	f.drain()
	f.wantRuns(0)
	f.clock.Advance(time.Millisecond)
	f.drain()
	if run := f.wantRuns(1)[0]; run.Attempt != 1 || !run.RunAt.Equal(j.RunAt) {
		t.Errorf("job = %+v, want Attempt 1 (re-enqueues do not count) and RunAt %v", run, j.RunAt)
	}
}
