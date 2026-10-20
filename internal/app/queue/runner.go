// SPDX-License-Identifier: AGPL-3.0-only

package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	randv2 "math/rand/v2"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
)

// JobRunner runs jobs until ctx ends. It has two forms (ADR-0004): a pull runner, which polls
// a queue (PullRunner, for the database queue and long-running SQS workers), and a push
// receiver, which handles the messages a platform delivers (Lambda SQS events, Cloud Tasks
// HTTP targets).
type JobRunner interface {
	// Run processes jobs until ctx ends, then returns nil. It returns an error only when it
	// cannot run at all.
	Run(ctx context.Context) error
}

// Source is the storage side of a pull queue, implemented by queue adapters and driven by
// PullRunner. Claims are leases: a claimed job is hidden from other workers until the lease
// expires, after which another worker may claim it again. That is how jobs survive a worker
// that stops while running them.
//
// A claim is identified by the job's ID, the worker, and the claimed Attempt. Complete,
// Release, and Bury act only while that claim still holds and return ErrLeaseLost otherwise.
type Source interface {
	// Claim leases the due job (RunAt <= now, not leased, not held back by its ordering key;
	// see "Ordering keys" in the package documentation) with the earliest RunAt to worker
	// until now+lease, increments its Attempt, and returns it. ok is false when no job is due.
	Claim(ctx context.Context, worker string, now time.Time, lease time.Duration) (j Job, ok bool, err error)
	// Complete removes the claimed job j.
	Complete(ctx context.Context, worker string, j Job) error
	// Release ends the claim of j and makes the job due again at runAt, with its Attempt set to
	// attempt and its LastError to lastErr. An adapter whose delays are limited makes the job
	// visible at NextVisible(now, runAt, limit) and keeps runAt as the job's RunAt.
	Release(ctx context.Context, worker string, j Job, attempt int, runAt time.Time, lastErr string) error
	// Bury moves the claimed job j to the dead jobs with lastErr, failed at failedAt.
	Bury(ctx context.Context, worker string, j Job, lastErr string, failedAt time.Time) error
}

// Defaults applied to the zero fields of RunnerOptions.
const (
	DefaultLease        = 5 * time.Minute
	DefaultPollInterval = time.Second
)

// RunnerOptions configure a PullRunner. Clock is required; the other zero fields take defaults.
type RunnerOptions struct {
	Clock kernel.Clock
	// Worker names this runner in claims. It must be unique among the runners of one queue;
	// the default is random.
	Worker string
	// Lease is how long a claim hides a job from other workers. A handler gets a context that
	// ends at the lease, so keep it longer than the slowest handler.
	Lease time.Duration
	// PollInterval is how long Run waits after finding no due job.
	PollInterval time.Duration
	// Retry is the default policy of handlers registered without one.
	Retry RetryPolicy
	// Random returns numbers in [0, 1) for backoff jitter; the default is math/rand/v2.
	// Tests pass a fixed function for deterministic delays.
	Random func() float64
	// Logger receives errors that Run survives; the default is slog.Default().
	Logger *slog.Logger
	// Meter records my.job.duration and my.job.wait (ADR-0055 §3); nil records nothing.
	Meter kernel.Meter
}

// PullRunner claims jobs from a Source, runs their handlers, and records the results. It
// implements JobRunner. Several PullRunners (in one or several processes) may share a Source.
type PullRunner struct {
	src     Source
	reg     *JobHandlerRegistry
	opts    RunnerOptions
	metrics jobMetrics
}

var _ JobRunner = (*PullRunner)(nil)

// NewPullRunner returns a runner of the jobs in src with the handlers in reg. It panics when
// opts.Clock is nil.
func NewPullRunner(src Source, reg *JobHandlerRegistry, opts RunnerOptions) *PullRunner {
	if opts.Clock == nil {
		panic("queue: RunnerOptions.Clock is required")
	}
	if opts.Worker == "" {
		var b [8]byte
		_, _ = rand.Read(b[:])
		opts.Worker = "worker-" + hex.EncodeToString(b[:])
	}
	if opts.Lease <= 0 {
		opts.Lease = DefaultLease
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}
	opts.Retry = opts.Retry.withDefaults()
	if opts.Random == nil {
		opts.Random = randv2.Float64
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if reg == nil {
		reg = &JobHandlerRegistry{}
	}
	return &PullRunner{src: src, reg: reg, opts: opts, metrics: newJobMetrics(opts.Meter)}
}

// Worker returns the name this runner claims jobs under.
func (r *PullRunner) Worker() string { return r.opts.Worker }

// Run implements JobRunner: it runs due jobs one after another and polls every PollInterval
// while none is due. Errors of single jobs are logged, not returned.
func (r *PullRunner) Run(ctx context.Context) error {
	return r.RunUntil(ctx, ctx.Done())
}

// RunUntil runs jobs like Run until stop is closed or ctx ends, then returns nil. Closing stop
// is a graceful stop: the runner claims no more jobs, but the job in progress keeps ctx and
// runs to its end. Ending ctx also cancels the context of the job in progress.
func (r *PullRunner) RunUntil(ctx context.Context, stop <-chan struct{}) error {
	stopped := func() bool {
		select {
		case <-stop:
			return true
		default:
			return ctx.Err() != nil
		}
	}
	for {
		if stopped() {
			return nil
		}
		ran, err := r.RunOnce(ctx)
		if stopped() {
			return nil
		}
		if err != nil {
			r.opts.Logger.ErrorContext(ctx, "job runner", slog.String("worker", r.opts.Worker), slog.Any("error", err))
		}
		if ran && err == nil {
			continue
		}
		timer := time.NewTimer(r.opts.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-stop:
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// Drain runs due jobs until none is left and returns how many it claimed. Jobs that a handler
// retries become due later and are not run again by the same Drain unless the clock moves.
func (r *PullRunner) Drain(ctx context.Context) (int, error) {
	n := 0
	for {
		ran, err := r.RunOnce(ctx)
		if ran {
			n++
		}
		if err != nil || !ran {
			return n, err
		}
	}
}

// RunOnce claims at most one due job, runs it, and records the result. It reports whether it
// claimed a job. When the lease ran out while the handler ran and another worker claimed the
// job, the result is dropped and the error matches ErrLeaseLost.
func (r *PullRunner) RunOnce(ctx context.Context) (bool, error) {
	now := r.opts.Clock.Now()
	j, ok, err := r.src.Claim(ctx, r.opts.Worker, now, r.opts.Lease)
	if err != nil || !ok {
		return false, err
	}
	// The result must be recorded even when ctx ended during the handler; otherwise the job
	// would wait for its lease to expire.
	recordCtx := context.WithoutCancel(ctx)

	if j.RunAt.After(now) {
		// Delivered early by a queue with a delay limit: re-enqueue without running (the
		// re-enqueue chain) and give the attempt back.
		return true, r.src.Release(recordCtx, r.opts.Worker, j, j.Attempt-1, j.RunAt, j.LastError)
	}

	handler, policy, found := r.reg.Lookup(j.Type)
	policy = policy.Or(r.opts.Retry)
	jobType := j.Type
	if !found {
		jobType = JobTypeOther // a type no handler knows is not a finite label value
	}
	mctx, end := r.metrics.meter.Begin(ctx)
	defer end()
	r.metrics.waited(mctx, jobType, now.Sub(j.RunAt))
	started := time.Now() // the monotonic clock: Clock.Now has millisecond precision
	var res Result
	if found {
		res = r.call(mctx, handler, j)
	} else {
		res = Retry(0, fmt.Errorf("%w: %s", ErrNoHandler, j.Type))
	}

	done := r.opts.Clock.Now()
	switch {
	case res.IsDone():
		r.metrics.ran(mctx, jobType, OutcomeSuccess, time.Since(started))
		return true, r.src.Complete(recordCtx, r.opts.Worker, j)
	case res.IsFail():
		r.metrics.ran(mctx, jobType, OutcomeDead, time.Since(started))
		return true, r.src.Bury(recordCtx, r.opts.Worker, j, errorText(res.Err(), "failed"), done)
	case j.Attempt >= policy.MaxAttempts:
		r.metrics.ran(mctx, jobType, OutcomeDead, time.Since(started))
		return true, r.src.Bury(recordCtx, r.opts.Worker, j, errorText(res.Err(), "retried")+
			fmt.Sprintf(" (gave up after %d attempts)", j.Attempt), done)
	default:
		r.metrics.ran(mctx, jobType, OutcomeRetry, time.Since(started))
		after := res.After()
		if after <= 0 {
			after = policy.Backoff(j.Attempt, r.opts.Random)
		}
		return true, r.src.Release(recordCtx, r.opts.Worker, j, j.Attempt, done.Add(after), errorText(res.Err(), "retried"))
	}
}

// call runs h with a context bounded by the lease, turning a panic into a retry.
func (r *PullRunner) call(ctx context.Context, h Handler, j Job) (res Result) {
	ctx, cancel := context.WithTimeout(ctx, r.opts.Lease)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			res = Retry(0, fmt.Errorf("handler panicked: %v", p))
		}
	}()
	return h(ctx, j)
}

func errorText(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return err.Error()
}
