// SPDX-License-Identifier: AGPL-3.0-only

// Package handwritten sketches the "database/sql + hand-written SQL" option
// for one query. It compiles but is not wired into the contract suite.
//
// Observations:
//   - One SQL string can serve both dialects only for the common subset; the
//     placeholder style (? vs $1) and value encoding (unix ms vs timestamptz)
//     still differ, so a Dialect value leaks into every repository method.
//   - Scanning is manual (column order must match by hand) and untested by
//     the compiler; sqlc generates exactly this code from the SQL instead.
//   - PostgreSQL must go through pgx/v5/stdlib (database/sql), losing pgx's
//     native API (batch, COPY, richer error/type handling).
package handwritten

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Dialect int

const (
	SQLite Dialect = iota
	Postgres
)

// rebind turns ? placeholders into $1..$n for PostgreSQL.
func (d Dialect) rebind(q string) string {
	if d != Postgres {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (d Dialect) timeArg(t time.Time) any {
	if d == SQLite {
		return t.UnixMilli()
	}
	return t
}

type Follows struct {
	DB      *sql.DB // or *sql.Tx via an interface in real code
	Dialect Dialect
}

const insertFollow = `INSERT INTO follows (id, follower_id, target_id, created_at)
VALUES (?, ?, ?, ?) ON CONFLICT (follower_id, target_id) DO NOTHING`

func (r Follows) Insert(ctx context.Context, id, follower, target string, at time.Time) (bool, error) {
	res, err := r.DB.ExecContext(ctx, r.Dialect.rebind(insertFollow), id, follower, target, r.Dialect.timeArg(at))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
