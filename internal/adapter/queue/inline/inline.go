// SPDX-License-Identifier: AGPL-3.0-only

package inline

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
)

// Options configure a Queue. Clock and IDs are required.
type Options struct {
	Clock kernel.Clock
	IDs   kernel.IDGenerator
	// MaxDelay limits how far in the future a job becomes visible (0 for no limit).
	MaxDelay time.Duration
}

type entry struct {
	job queue.Job
	// visibleAt is when the job may be claimed: its RunAt, or earlier under MaxDelay.
	visibleAt   time.Time
	lockedBy    string
	lockedUntil time.Time
}

// Queue is an in-memory job queue. It is safe for concurrent use.
type Queue struct {
	opts Options

	mu    sync.Mutex
	jobs  map[string]*entry
	byKey map[string]string // idempotency key -> job ID
	dead  []queue.DeadJob

	runner   *queue.PullRunner
	draining bool
	again    bool
}

var (
	_ queue.JobQueue = (*Queue)(nil)
	_ queue.Source   = (*Queue)(nil)
	_ queue.DeadJobs = (*Queue)(nil)
)

// New returns an empty queue.
func New(opts Options) *Queue {
	if opts.Clock == nil || opts.IDs == nil {
		panic("queue/inline: Options.Clock and Options.IDs are required")
	}
	return &Queue{opts: opts, jobs: map[string]*entry{}, byKey: map[string]string{}}
}

// RunSynchronously makes every later Enqueue run the due jobs with r (a runner of this queue)
// before it returns. Jobs enqueued by a handler during such a run join the same run.
func (q *Queue) RunSynchronously(r *queue.PullRunner) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.runner = r
}

// Enqueue implements queue.JobQueue.
func (q *Queue) Enqueue(ctx context.Context, jobs ...queue.NewJob) error {
	now := q.opts.Clock.Now()
	built := make([]queue.Job, len(jobs))
	for i, n := range jobs {
		j, err := n.Build(q.opts.IDs.NewID(), now)
		if err != nil {
			return err
		}
		j.Payload = slices.Clone(j.Payload)
		built[i] = j
	}

	q.mu.Lock()
	for _, j := range built {
		if j.IdempotencyKey != "" {
			if _, dup := q.byKey[j.IdempotencyKey]; dup {
				continue
			}
			q.byKey[j.IdempotencyKey] = j.ID
		}
		q.jobs[j.ID] = &entry{job: j, visibleAt: queue.NextVisible(now, j.RunAt, q.opts.MaxDelay)}
	}
	r := q.runner
	if r == nil {
		q.mu.Unlock()
		return nil
	}
	if q.draining {
		q.again = true
		q.mu.Unlock()
		return nil
	}
	q.draining = true
	q.mu.Unlock()
	return q.drain(ctx, r)
}

// drain runs due jobs until a pass ends with no new enqueue in between.
func (q *Queue) drain(ctx context.Context, r *queue.PullRunner) error {
	for {
		_, err := r.Drain(ctx)
		q.mu.Lock()
		if err != nil || !q.again {
			q.draining, q.again = false, false
			q.mu.Unlock()
			return err
		}
		q.again = false
		q.mu.Unlock()
	}
}

// Claim implements queue.Source.
func (q *Queue) Claim(_ context.Context, worker string, now time.Time, lease time.Duration) (queue.Job, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var next *entry
	for _, e := range q.jobs {
		if e.visibleAt.After(now) || (e.lockedBy != "" && e.lockedUntil.After(now)) {
			continue
		}
		if next == nil || e.visibleAt.Before(next.visibleAt) ||
			(e.visibleAt.Equal(next.visibleAt) && strings.Compare(e.job.ID, next.job.ID) < 0) {
			next = e
		}
	}
	if next == nil {
		return queue.Job{}, false, nil
	}
	next.lockedBy = worker
	next.lockedUntil = now.Add(lease)
	next.job.Attempt++
	j := next.job
	j.Payload = slices.Clone(j.Payload)
	return j, true, nil
}

// claimed returns the entry of j while worker still holds its claim.
func (q *Queue) claimed(worker string, j queue.Job) (*entry, error) {
	e, ok := q.jobs[j.ID]
	if !ok || e.lockedBy != worker || e.job.Attempt != j.Attempt {
		return nil, queue.ErrLeaseLost
	}
	return e, nil
}

func (q *Queue) remove(e *entry) {
	delete(q.jobs, e.job.ID)
	if k := e.job.IdempotencyKey; k != "" && q.byKey[k] == e.job.ID {
		delete(q.byKey, k)
	}
}

// Complete implements queue.Source.
func (q *Queue) Complete(_ context.Context, worker string, j queue.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	e, err := q.claimed(worker, j)
	if err != nil {
		return err
	}
	q.remove(e)
	return nil
}

// Release implements queue.Source.
func (q *Queue) Release(_ context.Context, worker string, j queue.Job, attempt int, runAt time.Time, lastErr string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	e, err := q.claimed(worker, j)
	if err != nil {
		return err
	}
	runAt = kernel.Normalize(runAt)
	e.job.Attempt = attempt
	e.job.RunAt = runAt
	e.job.LastError = lastErr
	e.visibleAt = queue.NextVisible(q.opts.Clock.Now(), runAt, q.opts.MaxDelay)
	e.lockedBy, e.lockedUntil = "", time.Time{}
	return nil
}

// Bury implements queue.Source.
func (q *Queue) Bury(_ context.Context, worker string, j queue.Job, lastErr string, failedAt time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	e, err := q.claimed(worker, j)
	if err != nil {
		return err
	}
	q.remove(e)
	q.dead = append(q.dead, queue.DeadJob{
		ID:             e.job.ID,
		Type:           e.job.Type,
		Payload:        e.job.Payload,
		IdempotencyKey: e.job.IdempotencyKey,
		OrderingKey:    e.job.OrderingKey,
		Attempt:        e.job.Attempt,
		LastError:      lastErr,
		CreatedAt:      e.job.CreatedAt,
		FailedAt:       kernel.Normalize(failedAt),
	})
	return nil
}

// ListDeadJobs implements queue.DeadJobs.
func (q *Queue) ListDeadJobs(_ context.Context, limit int) ([]queue.DeadJob, error) {
	if limit <= 0 {
		return nil, nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	out := slices.Clone(q.dead)
	slices.SortStableFunc(out, func(a, b queue.DeadJob) int {
		if c := b.FailedAt.Compare(a.FailedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// RequeueDeadJob implements queue.DeadJobs.
func (q *Queue) RequeueDeadJob(_ context.Context, id string) error {
	now := q.opts.Clock.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	i := slices.IndexFunc(q.dead, func(d queue.DeadJob) bool { return d.ID == id })
	if i < 0 {
		return queue.ErrDeadJobNotFound
	}
	d := q.dead[i]
	q.dead = slices.Delete(q.dead, i, i+1)
	if d.IdempotencyKey != "" {
		if _, dup := q.byKey[d.IdempotencyKey]; dup {
			return nil
		}
		q.byKey[d.IdempotencyKey] = d.ID
	}
	q.jobs[d.ID] = &entry{
		job: queue.Job{
			ID:             d.ID,
			Type:           d.Type,
			Payload:        d.Payload,
			IdempotencyKey: d.IdempotencyKey,
			OrderingKey:    d.OrderingKey,
			RunAt:          now,
			CreatedAt:      d.CreatedAt,
		},
		visibleAt: now,
	}
	return nil
}
