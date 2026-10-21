// SPDX-License-Identifier: AGPL-3.0-only

package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
)

// Outbox keeps jobs in outbox_events until Relay hands them to a queue outside the database
// (Transactional Outbox, ADR-0004).
type Outbox struct {
	db   uow.UnitOfWork
	st   store
	opts Options
}

var _ queue.Outbox = (*Outbox)(nil)

// EnqueueTx implements queue.Outbox: it writes the jobs to outbox_events in tx. Idempotency
// keys take effect when Relay enqueues the jobs.
func (o *Outbox) EnqueueTx(ctx context.Context, tx uow.Tx, jobs ...queue.NewJob) error {
	built, err := build(o.opts, jobs)
	if err != nil {
		return err
	}
	for _, j := range built {
		if err := o.st.insertOutbox(ctx, tx, j); err != nil {
			return fmt.Errorf("queue/db: outbox %s: %w", j.Type, err)
		}
	}
	return nil
}

// DefaultRelayBatch is how many outbox events Relay moves per call when limit <= 0.
const DefaultRelayBatch = 100

// Relay moves up to limit outbox events, oldest first, into target and returns how many it
// moved. It enqueues outside any transaction and deletes the events afterwards, so a crash in
// between enqueues them again on the next call: delivery is at-least-once, and the jobs'
// idempotency keys and idempotent handlers absorb the duplicates.
func (o *Outbox) Relay(ctx context.Context, target queue.JobQueue, limit int) (int, error) {
	if limit <= 0 {
		limit = DefaultRelayBatch
	}
	var events []queue.Job
	err := o.db.Tx(ctx, func(tx uow.Tx) error {
		var err error
		events, err = o.st.listOutbox(ctx, tx, limit)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("queue/db: read outbox: %w", err)
	}
	if len(events) == 0 {
		return 0, nil
	}
	jobs := make([]queue.NewJob, len(events))
	for i, e := range events {
		jobs[i] = queue.NewJob{
			Type:           e.Type,
			Payload:        e.Payload,
			IdempotencyKey: e.IdempotencyKey,
			OrderingKey:    e.OrderingKey,
			RunAt:          e.RunAt,
		}
	}
	if err := target.Enqueue(ctx, jobs...); err != nil {
		return 0, fmt.Errorf("queue/db: relay outbox: %w", err)
	}
	err = o.db.Tx(ctx, func(tx uow.Tx) error {
		for _, e := range events {
			if err := o.st.deleteOutbox(ctx, tx, e.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("queue/db: delete relayed outbox events: %w", err)
	}
	return len(events), nil
}

// RunRelay relays outbox events into target until ctx ends, polling every interval while the
// outbox is empty. It logs errors and keeps going.
func (o *Outbox) RunRelay(ctx context.Context, target queue.JobQueue, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		interval = queue.DefaultPollInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	for {
		n, err := o.Relay(ctx, target, 0)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.ErrorContext(ctx, "outbox relay", slog.Any("error", err))
		}
		if n > 0 && err == nil {
			continue
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
