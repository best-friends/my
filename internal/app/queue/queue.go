// SPDX-License-Identifier: AGPL-3.0-only

// Package queue defines the job queue ports (ADR-0004) and the parts every queue adapter
// shares: the job model, the handler registry, the retry policy (exponential backoff with
// jitter), and the pull runner.
//
// Delivery is at-least-once: a job may run more than once (a worker can stop after running a
// handler and before recording the result), so handlers must be idempotent.
//
// The adapters live in internal/adapter/queue/*; their shared contract test suite is
// testkit.QueueContract in pkg/testkit.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
)

// Job is a unit of asynchronous work, as a handler receives it.
type Job struct {
	// ID is a UUIDv7.
	ID string
	// Type selects the handler, for example "federation.deliver".
	Type string
	// Payload is a small JSON document; large data stays in the database or blob storage and
	// the payload refers to it by ID. Adapters may reformat it (whitespace, key order) but
	// keep its meaning.
	Payload json.RawMessage
	// IdempotencyKey, when not empty, is unique among the jobs not yet completed: enqueuing a
	// job with the key of such a job does nothing.
	IdempotencyKey string
	// OrderingKey groups jobs that need to run one at a time (for example the inbox of one
	// actor). Adapters store and return it; the pull runner does not serialize by it yet.
	OrderingKey string
	// Attempt is the number of this run, from 1. A run counts as soon as a worker claims the
	// job, so runs cut short by a crash count too.
	Attempt int
	// RunAt is the earliest time the job may run.
	RunAt time.Time
	// LastError describes why the previous attempt did not finish, or is empty.
	LastError string
	CreatedAt time.Time
}

// NewJob describes a job to enqueue.
type NewJob struct {
	Type           string
	Payload        json.RawMessage
	IdempotencyKey string
	OrderingKey    string
	// RunAt delays the job; the zero value means now. The job runs no earlier than RunAt;
	// adapters do not guarantee how much later.
	RunAt time.Time
}

// Build validates n and turns it into the Job that adapters store, with the given ID, the
// creation time now, and RunAt defaulting to now. A nil payload becomes JSON null.
func (n NewJob) Build(id string, now time.Time) (Job, error) {
	if n.Type == "" {
		return Job{}, errors.New("queue: job type is empty")
	}
	payload := n.Payload
	if payload == nil {
		payload = json.RawMessage("null")
	}
	if !json.Valid(payload) {
		return Job{}, fmt.Errorf("queue: payload of %s job is not valid JSON", n.Type)
	}
	now = kernel.Normalize(now)
	runAt := now
	if !n.RunAt.IsZero() {
		runAt = kernel.Normalize(n.RunAt)
	}
	return Job{
		ID:             id,
		Type:           n.Type,
		Payload:        payload,
		IdempotencyKey: n.IdempotencyKey,
		OrderingKey:    n.OrderingKey,
		RunAt:          runAt,
		CreatedAt:      now,
	}, nil
}

// JobQueue enqueues jobs. Enqueue is external I/O: never call it inside UnitOfWork.Tx; use
// Outbox there instead (ADR-0003).
type JobQueue interface {
	// Enqueue adds jobs atomically: either all of them are enqueued or, on error, none. Jobs
	// whose IdempotencyKey matches a job not yet completed, or an earlier job of the same
	// call, are skipped.
	Enqueue(ctx context.Context, jobs ...NewJob) error
}

// Outbox enqueues jobs as part of a database transaction (Transactional Outbox, ADR-0004):
// the jobs exist exactly when the transaction commits, together with the domain writes made
// through the same tx.
//
// An implementation either writes the jobs table of the database queue directly or writes
// outbox_events, which a relay later moves into the configured JobQueue.
type Outbox interface {
	// EnqueueTx records jobs in tx, with the same deduplication as JobQueue.Enqueue (for
	// outbox_events, deduplication happens when the relay enqueues them). It is safe to
	// repeat when UnitOfWork.Tx retries the transaction.
	EnqueueTx(ctx context.Context, tx uow.Tx, jobs ...NewJob) error
}

// DeadJob is a job that failed for good: its handler returned Fail, or it used up its
// attempts. Dead jobs stay until an operator requeues or discards them.
type DeadJob struct {
	ID             string
	Type           string
	Payload        json.RawMessage
	IdempotencyKey string
	OrderingKey    string
	// Attempt is the number of runs the job had.
	Attempt   int
	LastError string
	CreatedAt time.Time
	FailedAt  time.Time
}

// DeadJobs gives the admin screen and the CLI access to dead jobs.
type DeadJobs interface {
	// ListDeadJobs returns up to limit dead jobs, most recently failed first; none when
	// limit <= 0.
	ListDeadJobs(ctx context.Context, limit int) ([]DeadJob, error)
	// RequeueDeadJob enqueues the dead job id again with a fresh attempt count, to run now,
	// and removes it from the dead jobs. When a job with the same IdempotencyKey is pending,
	// that job stands in for it. It returns ErrDeadJobNotFound when there is no such dead job.
	RequeueDeadJob(ctx context.Context, id string) error
}

var (
	// ErrDeadJobNotFound is returned by DeadJobs.RequeueDeadJob for an unknown ID.
	ErrDeadJobNotFound = errors.New("queue: dead job not found")
	// ErrLeaseLost is returned by Source methods when the worker no longer holds the job:
	// its lease expired and another worker claimed it. The other worker's result wins.
	ErrLeaseLost = errors.New("queue: job lease lost")
	// ErrNoHandler is recorded as the error of a job whose type has no registered handler.
	// Such a job is retried, in case a newer worker that knows the type picks it up.
	ErrNoHandler = errors.New("queue: no handler registered for job type")
)
