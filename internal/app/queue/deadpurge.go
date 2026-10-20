// SPDX-License-Identifier: AGPL-3.0-only

package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
)

// Retention of the dead jobs (FR-803).
const (
	// JobPurgeDeadJobs deletes the dead jobs past their retention; no payload.
	JobPurgeDeadJobs = "queue.purge_dead_jobs"
	// DeadJobPurgeInterval is the period of the dead job purge's schedule.
	DeadJobPurgeInterval = time.Hour
	// DefaultDeadJobRetention is how long a dead job is kept after it failed when the
	// configuration sets no period: long enough for an operator to notice and requeue it.
	DefaultDeadJobRetention = 30 * 24 * time.Hour
	// DeadJobPurgeBatch is the number of dead jobs one DeleteDeadJobsBefore call (one short
	// transaction) deletes; deadJobPurgeMaxBatches bounds the batches of one run, and the next
	// run continues.
	DeadJobPurgeBatch      = 1000
	deadJobPurgeMaxBatches = 20
)

// DeadJobPurgeDeps are the ports of the dead job purge.
type DeadJobPurgeDeps struct {
	DeadJobs DeadJobs
	Clock    kernel.Clock
	// Retention is how long a dead job is kept after it failed; zero means
	// DefaultDeadJobRetention.
	Retention time.Duration
	// Batch is the number of dead jobs per call; zero means DeadJobPurgeBatch.
	Batch int
}

// DeadJobPurger runs JobPurgeDeadJobs: it deletes the dead jobs that failed more than the
// retention ago, so that jobs nobody requeued do not pile up.
type DeadJobPurger struct{ d DeadJobPurgeDeps }

// NewDeadJobPurger returns the dead job purge on d.
func NewDeadJobPurger(d DeadJobPurgeDeps) *DeadJobPurger {
	if d.Retention <= 0 {
		d.Retention = DefaultDeadJobRetention
	}
	if d.Batch <= 0 {
		d.Batch = DeadJobPurgeBatch
	}
	return &DeadJobPurger{d: d}
}

// RegisterJobs registers the JobPurgeDeadJobs handler and its hourly schedule.
func (p *DeadJobPurger) RegisterJobs(h *JobHandlerRegistry, s *Scheduler) {
	h.Register(JobPurgeDeadJobs, func(ctx context.Context, _ Job) Result {
		if _, err := p.Run(ctx); err != nil {
			return Retry(0, err)
		}
		return Done()
	})
	s.Add(Schedule{Name: JobPurgeDeadJobs, Every: DeadJobPurgeInterval, Job: NewJob{Type: JobPurgeDeadJobs}})
}

// Run deletes the dead jobs past the retention, at most deadJobPurgeMaxBatches batches, and
// returns how many it deleted. It is safe to repeat.
func (p *DeadJobPurger) Run(ctx context.Context) (int64, error) {
	before := p.d.Clock.Now().Add(-p.d.Retention)
	var total int64
	for range deadJobPurgeMaxBatches {
		n, err := p.d.DeadJobs.DeleteDeadJobsBefore(ctx, before, p.d.Batch)
		if err != nil {
			return total, fmt.Errorf("queue: purge dead jobs: %w", err)
		}
		total += n
		if n < int64(p.d.Batch) {
			break
		}
	}
	return total, nil
}
