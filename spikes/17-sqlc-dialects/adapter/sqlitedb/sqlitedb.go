// SPDX-License-Identifier: AGPL-3.0-only

// Package sqlitedb implements the store ports on SQLite via database/sql and
// sqlc-generated queries. It is driver-agnostic: drivers.go wires
// modernc.org/sqlite or github.com/ncruces/go-sqlite3.
package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/adapter/sqlitedb/sqlitegen"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/internal/txretry"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/schema"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

// DB is a store.UnitOfWork backed by SQLite.
type DB struct {
	db          *sql.DB
	isRetryable func(error) bool
	Policy      txretry.Policy
}

// New wraps an opened *sql.DB. isRetryable classifies driver errors
// (SQLITE_BUSY / SQLITE_LOCKED) as retryable.
func New(db *sql.DB, isRetryable func(error) bool) *DB {
	return &DB{db: db, isRetryable: isRetryable}
}

func (d *DB) SQL() *sql.DB { return d.db }

// Migrate applies the schema (spike only; real code would use a migrator).
func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, schema.SQLite)
	return err
}

// Tx implements store.UnitOfWork. The driver is expected to open transactions
// with BEGIN IMMEDIATE (_txlock=immediate) so that the write lock is taken up
// front and busy_timeout applies; a deferred transaction that upgrades from
// read to write fails with SQLITE_BUSY immediately, ignoring busy_timeout.
func (d *DB) Tx(ctx context.Context, fn func(tx store.Tx) error) error {
	return txretry.Run(ctx, d.Policy, func() error {
		return txretry.Mark(d.once(ctx, fn), d.isRetryable)
	})
}

func (d *DB) once(ctx context.Context, fn func(tx store.Tx) error) (err error) {
	sqlTx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = sqlTx.Rollback()
		}
	}()
	q := sqlitegen.New(sqlTx)
	if err = fn(txRepos{q}); err != nil {
		return err
	}
	if err = sqlTx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type txRepos struct{ q *sqlitegen.Queries }

func (t txRepos) Accounts() store.AccountRepository { return accounts(t) }
func (t txRepos) Follows() store.FollowRepository   { return follows(t) }

// --- time mapping: INTEGER unix ms <-> time.Time (UTC) ---

func toMs(t time.Time) int64    { return t.UnixMilli() }
func fromMs(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

// --- AccountRepository ---

type accounts struct{ q *sqlitegen.Queries }

func toAccount(a sqlitegen.Account) store.Account {
	return store.Account{
		ID: a.ID, Username: a.Username, Domain: a.Domain, URI: a.Uri, DisplayName: a.DisplayName,
		FollowersCount: a.FollowersCount, FollowingCount: a.FollowingCount,
		CreatedAt: fromMs(a.CreatedAt), UpdatedAt: fromMs(a.UpdatedAt),
	}
}

func (r accounts) Upsert(ctx context.Context, a store.Account) (store.Account, error) {
	row, err := r.q.UpsertAccount(ctx, sqlitegen.UpsertAccountParams{
		ID: a.ID, Username: a.Username, Domain: a.Domain, Uri: a.URI, DisplayName: a.DisplayName,
		CreatedAt: toMs(a.CreatedAt), UpdatedAt: toMs(a.UpdatedAt),
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
	n, err := r.q.AddFollowCounts(ctx, sqlitegen.AddFollowCountsParams{
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

type follows struct{ q *sqlitegen.Queries }

func toFollow(f sqlitegen.Follow) store.Follow {
	return store.Follow{ID: f.ID, FollowerID: f.FollowerID, TargetID: f.TargetID, CreatedAt: fromMs(f.CreatedAt)}
}

func (r follows) Insert(ctx context.Context, f store.Follow) (bool, error) {
	n, err := r.q.InsertFollow(ctx, sqlitegen.InsertFollowParams{
		ID: f.ID, FollowerID: f.FollowerID, TargetID: f.TargetID, CreatedAt: toMs(f.CreatedAt),
	})
	return n == 1, err
}

func (r follows) Delete(ctx context.Context, followerID, targetID string) (bool, error) {
	n, err := r.q.DeleteFollow(ctx, sqlitegen.DeleteFollowParams{FollowerID: followerID, TargetID: targetID})
	return n == 1, err
}

func (r follows) Get(ctx context.Context, followerID, targetID string) (store.Follow, error) {
	row, err := r.q.GetFollow(ctx, sqlitegen.GetFollowParams{FollowerID: followerID, TargetID: targetID})
	if err != nil {
		return store.Follow{}, notFound(err)
	}
	return toFollow(row), nil
}

func (r follows) ListFollowers(ctx context.Context, targetID, maxID string, limit int) ([]store.Follow, error) {
	if maxID == "" {
		maxID = store.MaxIDUpperBound
	}
	rows, err := r.q.ListFollowers(ctx, sqlitegen.ListFollowersParams{TargetID: targetID, MaxID: maxID, Lim: int64(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]store.Follow, len(rows))
	for i, row := range rows {
		out[i] = toFollow(row)
	}
	return out, nil
}
