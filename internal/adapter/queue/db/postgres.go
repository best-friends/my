// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_postgres

package db

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/best-friends/my/internal/adapter/db/postgres"
	"github.com/best-friends/my/internal/adapter/queue/db/internal/postgresqueue"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
)

// NewPostgres returns the queue kept in the PostgreSQL database d.
func NewPostgres(d *postgres.DB, opts Options) *Queue {
	opts.check()
	return &Queue{db: d, st: postgresStore{}, opts: opts}
}

// NewPostgresOutbox returns the outbox kept in the PostgreSQL database d.
func NewPostgresOutbox(d *postgres.DB, opts Options) *Outbox {
	opts.check()
	return &Outbox{db: d, st: postgresStore{}, opts: opts}
}

type postgresStore struct{}

var _ store = postgresStore{}

func postgresQueries(tx uow.Tx) (*postgresqueue.Queries, error) {
	pgTx, err := postgres.PgxTx(tx)
	if err != nil {
		return nil, err
	}
	return postgresqueue.New(pgTx), nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// int32Of converts attempt counts and limits, which stay far below the int32 range.
func int32Of(n int) int32 {
	switch {
	case n > math.MaxInt32:
		return math.MaxInt32
	case n < math.MinInt32:
		return math.MinInt32
	}
	return int32(n)
}

func (postgresStore) insertJob(ctx context.Context, tx uow.Tx, j queue.Job) error {
	q, err := postgresQueries(tx)
	if err != nil {
		return err
	}
	return q.InsertJob(ctx, postgresqueue.InsertJobParams{
		ID:             j.ID,
		Type:           j.Type,
		Payload:        j.Payload,
		IdempotencyKey: optional(j.IdempotencyKey),
		OrderingKey:    optional(j.OrderingKey),
		RunAt:          j.RunAt,
		CreatedAt:      j.CreatedAt,
	})
}

func (postgresStore) claim(ctx context.Context, tx uow.Tx, worker string, now, until time.Time) (queue.Job, bool, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return queue.Job{}, false, err
	}
	// A candidate fails the second check only when a claim of its ordering key committed in
	// between, which the next pick sees; the bound keeps a claim short under contention.
	for range maxClaimPicks {
		c, err := q.PickJob(ctx, now)
		if errors.Is(err, pgx.ErrNoRows) {
			return queue.Job{}, false, nil
		}
		if err != nil {
			return queue.Job{}, false, err
		}
		if c.OrderingKey != nil {
			if err := q.LockOrderingKey(ctx, orderingLockPrefix+*c.OrderingKey); err != nil {
				return queue.Job{}, false, err
			}
		}
		r, err := q.ClaimJob(ctx, postgresqueue.ClaimJobParams{
			Worker:      &worker,
			LockedUntil: pgtype.Timestamptz{Time: until, Valid: true},
			ID:          c.ID,
			Now:         now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return queue.Job{}, false, err
		}
		return claimedJob(r), true, nil
	}
	return queue.Job{}, false, nil
}

const (
	// maxClaimPicks bounds the candidates one claim tries.
	maxClaimPicks = 4
	// orderingLockPrefix namespaces the advisory locks of ordering keys.
	orderingLockPrefix = "queue:ordering:"
)

func claimedJob(r postgresqueue.ClaimJobRow) queue.Job {
	return queue.Job{
		ID:             r.ID,
		Type:           r.Type,
		Payload:        json.RawMessage(r.Payload),
		IdempotencyKey: deref(r.IdempotencyKey),
		OrderingKey:    deref(r.OrderingKey),
		Attempt:        int(r.Attempt),
		RunAt:          kernel.Normalize(r.RunAt),
		LastError:      deref(r.LastError),
		CreatedAt:      kernel.Normalize(r.CreatedAt),
	}
}

func (postgresStore) deleteClaimed(ctx context.Context, tx uow.Tx, id, worker string, attempt int) (int64, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.DeleteClaimedJob(ctx, postgresqueue.DeleteClaimedJobParams{ID: id, LockedBy: &worker, Attempt: int32Of(attempt)})
}

func (postgresStore) release(ctx context.Context, tx uow.Tx, id, worker string, attempt, newAttempt int, runAt time.Time, lastErr string) (int64, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.ReleaseJob(ctx, postgresqueue.ReleaseJobParams{
		NewAttempt: int32Of(newAttempt),
		RunAt:      runAt,
		LastError:  optional(lastErr),
		ID:         id,
		Worker:     &worker,
		Attempt:    int32Of(attempt),
	})
}

func (postgresStore) insertDeadFromClaim(ctx context.Context, tx uow.Tx, id, worker string, attempt int, lastErr string, failedAt time.Time) (int64, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.InsertDeadJobFromClaim(ctx, postgresqueue.InsertDeadJobFromClaimParams{
		LastError: lastErr,
		FailedAt:  failedAt,
		ID:        id,
		Worker:    &worker,
		Attempt:   int32Of(attempt),
	})
}

func (postgresStore) listDead(ctx context.Context, tx uow.Tx, after queue.DeadJobPosition, limit int) ([]queue.DeadJob, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListDeadJobs(ctx, postgresqueue.ListDeadJobsParams{AfterID: after.ID, AfterFailedAt: after.FailedAt, BatchLimit: int32Of(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]queue.DeadJob, len(rows))
	for i, r := range rows {
		out[i] = queue.DeadJob{
			ID:             r.ID,
			Type:           r.Type,
			Payload:        json.RawMessage(r.Payload),
			IdempotencyKey: deref(r.IdempotencyKey),
			OrderingKey:    deref(r.OrderingKey),
			Attempt:        int(r.Attempt),
			LastError:      r.LastError,
			CreatedAt:      kernel.Normalize(r.CreatedAt),
			FailedAt:       kernel.Normalize(r.FailedAt),
		}
	}
	return out, nil
}

func (postgresStore) requeueDead(ctx context.Context, tx uow.Tx, id string, runAt time.Time) error {
	q, err := postgresQueries(tx)
	if err != nil {
		return err
	}
	_, err = q.RequeueDeadJob(ctx, postgresqueue.RequeueDeadJobParams{RunAt: runAt, ID: id})
	return err
}

func (postgresStore) deleteDead(ctx context.Context, tx uow.Tx, id string) (int64, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.DeleteDeadJob(ctx, id)
}

func (postgresStore) deleteDeadBefore(ctx context.Context, tx uow.Tx, before time.Time, limit int) (int64, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.DeleteDeadJobsBefore(ctx, postgresqueue.DeleteDeadJobsBeforeParams{FailedBefore: before, BatchLimit: int32Of(limit)})
}

func (postgresStore) insertOutbox(ctx context.Context, tx uow.Tx, j queue.Job) error {
	q, err := postgresQueries(tx)
	if err != nil {
		return err
	}
	return q.InsertOutboxEvent(ctx, postgresqueue.InsertOutboxEventParams{
		ID:             j.ID,
		JobType:        j.Type,
		Payload:        j.Payload,
		IdempotencyKey: optional(j.IdempotencyKey),
		OrderingKey:    optional(j.OrderingKey),
		RunAt:          j.RunAt,
		CreatedAt:      j.CreatedAt,
	})
}

func (postgresStore) listOutbox(ctx context.Context, tx uow.Tx, limit int) ([]queue.Job, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListOutboxEvents(ctx, int32Of(limit))
	if err != nil {
		return nil, err
	}
	out := make([]queue.Job, len(rows))
	for i, r := range rows {
		out[i] = queue.Job{
			ID:             r.ID,
			Type:           r.JobType,
			Payload:        json.RawMessage(r.Payload),
			IdempotencyKey: deref(r.IdempotencyKey),
			OrderingKey:    deref(r.OrderingKey),
			RunAt:          kernel.Normalize(r.RunAt),
			CreatedAt:      kernel.Normalize(r.CreatedAt),
		}
	}
	return out, nil
}

func (postgresStore) deleteOutbox(ctx context.Context, tx uow.Tx, id string) error {
	q, err := postgresQueries(tx)
	if err != nil {
		return err
	}
	return q.DeleteOutboxEvent(ctx, id)
}

func (postgresStore) backlog(ctx context.Context, tx uow.Tx, now time.Time) (queue.Backlog, error) {
	q, err := postgresQueries(tx)
	if err != nil {
		return queue.Backlog{}, err
	}
	rows, err := q.CountReadyJobs(ctx, now)
	if err != nil {
		return queue.Backlog{}, err
	}
	b := queue.Backlog{Ready: make(map[string]int64, len(rows))}
	for _, r := range rows {
		b.Ready[r.Type] = r.Ready
		if b.OldestReadyAt.IsZero() || r.OldestRunAt.Before(b.OldestReadyAt) {
			b.OldestReadyAt = r.OldestRunAt.UTC()
		}
	}
	b.Dead, err = q.CountDeadJobs(ctx)
	return b, err
}
