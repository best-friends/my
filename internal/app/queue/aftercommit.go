// SPDX-License-Identifier: AGPL-3.0-only

package queue

import (
	"context"
	"log/slog"

	"github.com/best-friends/my/internal/app/uow"
)

// eventAfterCommit is the event type through which AfterCommit outboxes hand their jobs to the
// commit hooks.
const eventAfterCommit = "queue.enqueue_after_commit"

// AfterCommit returns an Outbox for a queue that has no transactional enqueue (the inline
// queue, for tests and development): EnqueueTx emits an event in tx, and a commit hook
// registered on projectors enqueues its jobs through jobs once the transaction commits. An
// attempt that rolls back or is retried enqueues nothing.
//
// Unlike a real outbox, the jobs are lost when the process stops between the commit and the
// enqueue, or when Enqueue fails; the failure is logged. Use a database queue in production.
func AfterCommit(projectors *uow.Projectors, jobs JobQueue, logger *slog.Logger) Outbox {
	if logger == nil {
		logger = slog.Default()
	}
	o := &afterCommit{jobs: jobs, logger: logger}
	projectors.OnCommit(eventAfterCommit, o.committed)
	return o
}

type afterCommit struct {
	jobs   JobQueue
	logger *slog.Logger
}

type enqueueAfterCommit struct {
	owner *afterCommit
	jobs  []NewJob
}

func (enqueueAfterCommit) EventType() string { return eventAfterCommit }

// EnqueueTx implements Outbox.
func (o *afterCommit) EnqueueTx(ctx context.Context, tx uow.Tx, jobs ...NewJob) error {
	if len(jobs) == 0 {
		return nil
	}
	return tx.Emit(ctx, enqueueAfterCommit{owner: o, jobs: jobs})
}

func (o *afterCommit) committed(ctx context.Context, e uow.Event) {
	ev, ok := e.(enqueueAfterCommit)
	if !ok || ev.owner != o {
		return
	}
	if err := o.jobs.Enqueue(context.WithoutCancel(ctx), ev.jobs...); err != nil {
		o.logger.ErrorContext(ctx, "jobs not enqueued after commit", slog.String("type", ev.jobs[0].Type), slog.Any("error", err))
	}
}
