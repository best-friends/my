// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	msqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/best-friends/my/internal/adapter/db/sqlite/sqlitegen"
	"github.com/best-friends/my/internal/app/uow"
)

// queries returns the generated queries bound to the SQLite transaction behind tx.
func queries(tx uow.Tx) (*sqlitegen.Queries, error) {
	sqlTx, err := txOf(tx)
	if err != nil {
		return nil, err
	}
	return sqlitegen.New(sqlTx), nil
}

// reader returns the generated queries for reads outside transactions: on the connection pool,
// or in the transaction that the context of a query reads in (uow.ReadingIn).
func (d *DB) reader() *sqlitegen.Queries { return sqlitegen.New(readConn{d.db}) }

// readConn runs a query in the SQLite transaction that its context reads in, and on the pool
// otherwise.
type readConn struct{ db *sql.DB }

func (c readConn) conn(ctx context.Context) sqlitegen.DBTX {
	if tx, ok := uow.ReadingTx(ctx); ok {
		if t, ok := tx.(*Tx); ok {
			return t.tx
		}
	}
	return c.db
}

func (c readConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.conn(ctx).ExecContext(ctx, query, args...)
}

func (c readConn) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return c.conn(ctx).PrepareContext(ctx, query)
}

func (c readConn) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.conn(ctx).QueryContext(ctx, query, args...)
}

func (c readConn) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return c.conn(ctx).QueryRowContext(ctx, query, args...)
}

// Timestamps are stored as Unix milliseconds (data-model §1.2).
func toMillis(t time.Time) int64 { return t.UnixMilli() }

func fromMillis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func toNullMillis(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}

func fromNullMillis(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := fromMillis(n.Int64)
	return &t
}

// Optional references are NULL in the database and "" in the domain.
func toNullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func wrap(err error, what string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("sqlite: %s: %w", what, err)
}

// notFound maps sql.ErrNoRows to notFoundErr and wraps every other error with what.
func notFound(err error, notFoundErr error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", what, notFoundErr)
	}
	return fmt.Errorf("sqlite: %s: %w", what, err)
}

// isUniqueViolation reports whether err is a UNIQUE or PRIMARY KEY violation on the given
// columns, which SQLite names as "table.column, table.column" in the message.
func isUniqueViolation(err error, columns string) bool {
	var e *msqlite.Error
	if !errors.As(err, &e) {
		return false
	}
	switch e.Code() {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return strings.Contains(e.Error(), "constraint failed: "+columns)
	}
	return false
}
