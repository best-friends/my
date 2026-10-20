// SPDX-License-Identifier: AGPL-3.0-only

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/best-friends/my/internal/adapter/db/sqlite"
	"github.com/best-friends/my/internal/adapter/queue/db/internal/sqlitequeue"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
)

// sqliteStore stores timestamps as Unix milliseconds (data-model.md §1.2).
type sqliteStore struct{}

var _ store = sqliteStore{}

func sqliteQueries(tx uow.Tx) (*sqlitequeue.Queries, error) {
	sqlTx, err := sqlite.SQLTx(tx)
	if err != nil {
		return nil, err
	}
	return sqlitequeue.New(sqlTx), nil
}

func millis(t time.Time) int64 { return t.UnixMilli() }

func fromMillis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func (sqliteStore) insertJob(ctx context.Context, tx uow.Tx, j queue.Job) error {
	q, err := sqliteQueries(tx)
	if err != nil {
		return err
	}
	return q.InsertJob(ctx, sqlitequeue.InsertJobParams{
		ID:             j.ID,
		Type:           j.Type,
		Payload:        j.Payload,
		IdempotencyKey: nullString(j.IdempotencyKey),
		OrderingKey:    nullString(j.OrderingKey),
		RunAt:          millis(j.RunAt),
		CreatedAt:      millis(j.CreatedAt),
	})
}

func (sqliteStore) claim(ctx context.Context, tx uow.Tx, worker string, now, until time.Time) (queue.Job, bool, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return queue.Job{}, false, err
	}
	r, err := q.ClaimJob(ctx, sqlitequeue.ClaimJobParams{
		Worker:      nullString(worker),
		LockedUntil: sql.NullInt64{Int64: millis(until), Valid: true},
		Now:         millis(now),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return queue.Job{}, false, nil
	}
	if err != nil {
		return queue.Job{}, false, err
	}
	return queue.Job{
		ID:             r.ID,
		Type:           r.Type,
		Payload:        json.RawMessage(r.Payload),
		IdempotencyKey: r.IdempotencyKey.String,
		OrderingKey:    r.OrderingKey.String,
		Attempt:        int(r.Attempt),
		RunAt:          fromMillis(r.RunAt),
		LastError:      r.LastError.String,
		CreatedAt:      fromMillis(r.CreatedAt),
	}, true, nil
}

func (sqliteStore) deleteClaimed(ctx context.Context, tx uow.Tx, id, worker string, attempt int) (int64, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.DeleteClaimedJob(ctx, sqlitequeue.DeleteClaimedJobParams{ID: id, LockedBy: nullString(worker), Attempt: int64(attempt)})
}

func (sqliteStore) release(ctx context.Context, tx uow.Tx, id, worker string, attempt, newAttempt int, runAt time.Time, lastErr string) (int64, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.ReleaseJob(ctx, sqlitequeue.ReleaseJobParams{
		NewAttempt: int64(newAttempt),
		RunAt:      millis(runAt),
		LastError:  nullString(lastErr),
		ID:         id,
		Worker:     nullString(worker),
		Attempt:    int64(attempt),
	})
}

func (sqliteStore) insertDeadFromClaim(ctx context.Context, tx uow.Tx, id, worker string, attempt int, lastErr string, failedAt time.Time) (int64, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.InsertDeadJobFromClaim(ctx, sqlitequeue.InsertDeadJobFromClaimParams{
		LastError: lastErr,
		FailedAt:  millis(failedAt),
		ID:        id,
		Worker:    nullString(worker),
		Attempt:   int64(attempt),
	})
}

func (sqliteStore) listDead(ctx context.Context, tx uow.Tx, after queue.DeadJobPosition, limit int) ([]queue.DeadJob, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return nil, err
	}
	var afterFailedAt int64
	if after.ID != "" {
		afterFailedAt = millis(after.FailedAt)
	}
	rows, err := q.ListDeadJobs(ctx, sqlitequeue.ListDeadJobsParams{AfterID: after.ID, AfterFailedAt: afterFailedAt, BatchLimit: int64(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]queue.DeadJob, len(rows))
	for i, r := range rows {
		out[i] = queue.DeadJob{
			ID:             r.ID,
			Type:           r.Type,
			Payload:        json.RawMessage(r.Payload),
			IdempotencyKey: r.IdempotencyKey.String,
			OrderingKey:    r.OrderingKey.String,
			Attempt:        int(r.Attempt),
			LastError:      r.LastError,
			CreatedAt:      fromMillis(r.CreatedAt),
			FailedAt:       fromMillis(r.FailedAt),
		}
	}
	return out, nil
}

func (sqliteStore) requeueDead(ctx context.Context, tx uow.Tx, id string, runAt time.Time) error {
	q, err := sqliteQueries(tx)
	if err != nil {
		return err
	}
	_, err = q.RequeueDeadJob(ctx, sqlitequeue.RequeueDeadJobParams{RunAt: millis(runAt), ID: id})
	return err
}

func (sqliteStore) deleteDead(ctx context.Context, tx uow.Tx, id string) (int64, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return 0, err
	}
	return q.DeleteDeadJob(ctx, id)
}

func (sqliteStore) insertOutbox(ctx context.Context, tx uow.Tx, j queue.Job) error {
	q, err := sqliteQueries(tx)
	if err != nil {
		return err
	}
	return q.InsertOutboxEvent(ctx, sqlitequeue.InsertOutboxEventParams{
		ID:             j.ID,
		JobType:        j.Type,
		Payload:        j.Payload,
		IdempotencyKey: nullString(j.IdempotencyKey),
		OrderingKey:    nullString(j.OrderingKey),
		RunAt:          millis(j.RunAt),
		CreatedAt:      millis(j.CreatedAt),
	})
}

func (sqliteStore) listOutbox(ctx context.Context, tx uow.Tx, limit int) ([]queue.Job, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListOutboxEvents(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]queue.Job, len(rows))
	for i, r := range rows {
		out[i] = queue.Job{
			ID:             r.ID,
			Type:           r.JobType,
			Payload:        json.RawMessage(r.Payload),
			IdempotencyKey: r.IdempotencyKey.String,
			OrderingKey:    r.OrderingKey.String,
			RunAt:          fromMillis(r.RunAt),
			CreatedAt:      fromMillis(r.CreatedAt),
		}
	}
	return out, nil
}

func (sqliteStore) deleteOutbox(ctx context.Context, tx uow.Tx, id string) error {
	q, err := sqliteQueries(tx)
	if err != nil {
		return err
	}
	return q.DeleteOutboxEvent(ctx, id)
}

func (sqliteStore) backlog(ctx context.Context, tx uow.Tx, now time.Time) (queue.Backlog, error) {
	q, err := sqliteQueries(tx)
	if err != nil {
		return queue.Backlog{}, err
	}
	rows, err := q.CountReadyJobs(ctx, millis(now))
	if err != nil {
		return queue.Backlog{}, err
	}
	b := queue.Backlog{Ready: make(map[string]int64, len(rows))}
	for _, r := range rows {
		b.Ready[r.Type] = r.Ready
		if at := fromMillis(r.OldestRunAt); b.OldestReadyAt.IsZero() || at.Before(b.OldestReadyAt) {
			b.OldestReadyAt = at
		}
	}
	b.Dead, err = q.CountDeadJobs(ctx)
	return b, err
}
