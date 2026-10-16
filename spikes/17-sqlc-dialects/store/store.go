// SPDX-License-Identifier: AGPL-3.0-only

// Package store holds the persistence ports the application layer depends on.
// It imports no driver; adapters (sqlitedb, pgdb) implement these interfaces.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound is returned when a row does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrRetryable marks an error that the UnitOfWork may retry
	// (serialization failure, deadlock, SQLITE_BUSY, ...). Adapters wrap driver
	// errors with it; application code may also return it explicitly.
	ErrRetryable = errors.New("store: retryable transaction conflict")
)

// Account is a local or remote actor. Times are UTC with millisecond precision.
type Account struct {
	ID             string // UUIDv7
	Username       string
	Domain         string // "" for local
	URI            string // unique; upsert key for remote accounts
	DisplayName    string
	FollowersCount int64
	FollowingCount int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Follow is an accepted follow relationship.
type Follow struct {
	ID         string // UUIDv7, also used as the pagination cursor
	FollowerID string
	TargetID   string
	CreatedAt  time.Time
}

// AccountRepository is the aggregate repository for accounts.
type AccountRepository interface {
	// Upsert inserts the account or, if one with the same URI exists, updates
	// its mutable fields. It returns the stored row (whose ID may differ from a.ID).
	Upsert(ctx context.Context, a Account) (Account, error)
	Get(ctx context.Context, id string) (Account, error)
	// AddFollowCounts adds deltas with `SET n = n + ?` (no read-modify-write).
	AddFollowCounts(ctx context.Context, id string, followersDelta, followingDelta int64) error
	// RecountFollowCounts recomputes the counters from follows (correction job).
	RecountFollowCounts(ctx context.Context, id string) error
}

// FollowRepository is the aggregate repository for follows.
type FollowRepository interface {
	// Insert creates the follow unless (FollowerID, TargetID) already exists.
	// created is false for a duplicate; that is not an error.
	Insert(ctx context.Context, f Follow) (created bool, err error)
	Delete(ctx context.Context, followerID, targetID string) (deleted bool, err error)
	Get(ctx context.Context, followerID, targetID string) (Follow, error)
	// ListFollowers returns follows of targetID with ID < maxID, newest first.
	// maxID "" means no upper bound.
	ListFollowers(ctx context.Context, targetID, maxID string, limit int) ([]Follow, error)
}

// Tx gives access to repositories bound to one transaction.
type Tx interface {
	Accounts() AccountRepository
	Follows() FollowRepository
}

// UnitOfWork runs fn inside a short transaction. fn may be called more than
// once (retry on ErrRetryable), so it must not perform external I/O and must
// reset any captured result variables at its start.
type UnitOfWork interface {
	Tx(ctx context.Context, fn func(tx Tx) error) error
}

// MaxIDUpperBound is used when the caller passes no max_id. UUID strings are
// lowercase hex, so "g" sorts after every valid ID.
const MaxIDUpperBound = "g"

// Timestamp normalizes t to the storage contract: UTC, millisecond precision.
func Timestamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }
