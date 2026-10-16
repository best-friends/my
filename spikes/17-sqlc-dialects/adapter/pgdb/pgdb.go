// SPDX-License-Identifier: AGPL-3.0-only

// Package pgdb implements the store ports on PostgreSQL via pgx/v5 and
// sqlc-generated queries.
package pgdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/adapter/pgdb/pggen"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/internal/txretry"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/schema"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

// DB is a store.UnitOfWork backed by PostgreSQL.
type DB struct {
	pool *pgxpool.Pool
	// IsoLevel defaults to Serializable so that the retry path is exercised
	// the same way Aurora DSQL (optimistic concurrency) would exercise it.
	IsoLevel pgx.TxIsoLevel
	Policy   txretry.Policy
}

func New(pool *pgxpool.Pool) *DB { return &DB{pool: pool, IsoLevel: pgx.Serializable} }

func (d *DB) Pool() *pgxpool.Pool { return d.pool }

func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.pool.Exec(ctx, schema.Postgres)
	return err
}

// IsRetryable reports serialization_failure (40001) and deadlock_detected (40P01).
func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	return false
}

func (d *DB) Tx(ctx context.Context, fn func(tx store.Tx) error) error {
	return txretry.Run(ctx, d.Policy, func() error {
		return txretry.Mark(d.once(ctx, fn), IsRetryable)
	})
}

func (d *DB) once(ctx context.Context, fn func(tx store.Tx) error) (err error) {
	pgTx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: d.IsoLevel})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = pgTx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err = fn(txRepos{pggen.New(pgTx)}); err != nil {
		return err
	}
	if err = pgTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type txRepos struct{ q *pggen.Queries }

func (t txRepos) Accounts() store.AccountRepository { return accounts(t) }
func (t txRepos) Follows() store.FollowRepository   { return follows(t) }

// timestamptz has microsecond precision; the store contract is milliseconds.
func ts(t time.Time) time.Time { return store.Timestamp(t) }

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

// --- AccountRepository ---

type accounts struct{ q *pggen.Queries }

func toAccount(a pggen.Account) store.Account {
	return store.Account{
		ID: a.ID, Username: a.Username, Domain: a.Domain, URI: a.Uri, DisplayName: a.DisplayName,
		FollowersCount: a.FollowersCount, FollowingCount: a.FollowingCount,
		CreatedAt: ts(a.CreatedAt), UpdatedAt: ts(a.UpdatedAt),
	}
}

func (r accounts) Upsert(ctx context.Context, a store.Account) (store.Account, error) {
	row, err := r.q.UpsertAccount(ctx, pggen.UpsertAccountParams{
		ID: a.ID, Username: a.Username, Domain: a.Domain, Uri: a.URI, DisplayName: a.DisplayName,
		CreatedAt: ts(a.CreatedAt), UpdatedAt: ts(a.UpdatedAt),
	})
	if err != nil {
		return store.Account{}, err
	}
	return toAccount(row), nil
}

func (r accounts) Get(ctx context.Context, id string) (store.Account, error) {
	row, err := r.q.GetAccount(ctx, id)
	if err != nil {
		return store.Account{}, notFound(err)
	}
	return toAccount(row), nil
}

func (r accounts) AddFollowCounts(ctx context.Context, id string, followers, following int64) error {
	n, err := r.q.AddFollowCounts(ctx, pggen.AddFollowCountsParams{
		FollowersDelta: followers, FollowingDelta: following, ID: id,
	})
	if err == nil && n == 0 {
		err = store.ErrNotFound
	}
	return err
}

func (r accounts) RecountFollowCounts(ctx context.Context, id string) error {
	n, err := r.q.RecountFollowCounts(ctx, id)
	if err == nil && n == 0 {
		err = store.ErrNotFound
	}
	return err
}

// --- FollowRepository ---

type follows struct{ q *pggen.Queries }

func toFollow(f pggen.Follow) store.Follow {
	return store.Follow{ID: f.ID, FollowerID: f.FollowerID, TargetID: f.TargetID, CreatedAt: ts(f.CreatedAt)}
}

func (r follows) Insert(ctx context.Context, f store.Follow) (bool, error) {
	n, err := r.q.InsertFollow(ctx, pggen.InsertFollowParams{
		ID: f.ID, FollowerID: f.FollowerID, TargetID: f.TargetID, CreatedAt: ts(f.CreatedAt),
	})
	return n == 1, err
}

func (r follows) Delete(ctx context.Context, followerID, targetID string) (bool, error) {
	n, err := r.q.DeleteFollow(ctx, pggen.DeleteFollowParams{FollowerID: followerID, TargetID: targetID})
	return n == 1, err
}

func (r follows) Get(ctx context.Context, followerID, targetID string) (store.Follow, error) {
	row, err := r.q.GetFollow(ctx, pggen.GetFollowParams{FollowerID: followerID, TargetID: targetID})
	if err != nil {
		return store.Follow{}, notFound(err)
	}
	return toFollow(row), nil
}

func (r follows) ListFollowers(ctx context.Context, targetID, maxID string, limit int) ([]store.Follow, error) {
	if maxID == "" {
		maxID = store.MaxIDUpperBound
	}
	rows, err := r.q.ListFollowers(ctx, pggen.ListFollowersParams{TargetID: targetID, MaxID: maxID, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]store.Follow, len(rows))
	for i, row := range rows {
		out[i] = toFollow(row)
	}
	return out, nil
}
