// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
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

// reader returns the generated queries on the connection pool, for reads outside transactions.
func (d *DB) reader() *sqlitegen.Queries { return sqlitegen.New(d.db) }

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
