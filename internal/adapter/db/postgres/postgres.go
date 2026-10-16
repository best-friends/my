// SPDX-License-Identifier: AGPL-3.0-only

// Package postgres implements the persistence ports on PostgreSQL and compatible services
// with pgx/v5 (ADR-0003).
//
// Transactions run at READ COMMITTED. Serialization failures (SQLSTATE 40001) and deadlocks
// (40P01) are retryable: UnitOfWork.Tx runs the transaction again.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/best-friends/my/internal/app/uow"
)

// DB is a PostgreSQL connection pool. It implements uow.UnitOfWork.
type DB struct {
	pool *pgxpool.Pool
	opts uow.Options
}

var _ uow.UnitOfWork = (*DB)(nil)

// Open connects to the PostgreSQL database at url (the database_url setting for
// db = "postgres"), in any form pgx accepts: a postgres:// URL or a keyword/value string.
//
// Open does not apply the schema; Atlas does (ADR-0023).
func Open(ctx context.Context, url string, opts uow.Options) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// The error may quote the URL, which can hold a password; keep only its kind.
		return nil, errors.New("postgres: database_url is not a valid connection string")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	return &DB{pool: pool, opts: opts}, nil
}

// Close closes every connection of the pool.
func (d *DB) Close() { d.pool.Close() }

// Tx implements uow.UnitOfWork.
func (d *DB) Tx(ctx context.Context, fn func(tx uow.Tx) error) error {
	return uow.Retry(ctx, d.opts.Retry, func() error {
		err := d.attempt(ctx, fn)
		if isRetryable(err) {
			return uow.MarkRetryable(err)
		}
		return err
	})
}

func (d *DB) attempt(ctx context.Context, fn func(tx uow.Tx) error) error {
	pgTx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			// Roll back even when ctx has ended, so that the connection returns to the pool clean.
			_ = pgTx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err := fn(&Tx{tx: pgTx, projectors: d.opts.Projectors}); err != nil {
		return err
	}
	if err := pgTx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	committed = true
	return nil
}

// isRetryable reports serialization_failure (40001) and deadlock_detected (40P01).
func isRetryable(err error) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && (e.Code == "40001" || e.Code == "40P01")
}

// Tx is a PostgreSQL transaction. It implements uow.Tx.
type Tx struct {
	tx         pgx.Tx
	projectors *uow.Projectors
}

var _ uow.Tx = (*Tx)(nil)

// Emit implements uow.Tx.
func (t *Tx) Emit(ctx context.Context, e uow.Event) error {
	return t.projectors.Project(ctx, t, e)
}

// txOf recovers the PostgreSQL transaction behind tx, for repositories of this adapter.
func txOf(tx uow.Tx) (pgx.Tx, error) {
	t, ok := tx.(*Tx)
	if !ok {
		return nil, fmt.Errorf("postgres: transaction %T does not belong to this adapter", tx)
	}
	return t.tx, nil
}
