// SPDX-License-Identifier: AGPL-3.0-only

// Package bobsketch compares bob's query builder with sqlc for one query
// (InsertFollow). It compiles but is not wired into the contract suite.
//
// Observations:
//   - The builder is per dialect (dialect/psql vs dialect/sqlite); code that
//     must run on both still needs two functions or a generic wrapper.
//   - Column names are strings; typos surface only at runtime unless bobgen
//     is used, and bobgen generates models from a *live* database per dialect.
//   - Executes through database/sql (bob.NewDB); pgx native API is not used.
package bobsketch

import (
	"context"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	pim "github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/sqlite"
	sim "github.com/stephenafamo/bob/dialect/sqlite/im"
)

func InsertFollowSQLite(ctx context.Context, db bob.Executor, id, follower, target string, at time.Time) (bool, error) {
	q := sqlite.Insert(
		sim.Into("follows", "id", "follower_id", "target_id", "created_at"),
		sim.Values(sqlite.Arg(id, follower, target, at.UnixMilli())),
		sim.OnConflict("follower_id", "target_id").DoNothing(),
	)
	res, err := bob.Exec(ctx, db, q)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func InsertFollowPostgres(ctx context.Context, db bob.Executor, id, follower, target string, at time.Time) (bool, error) {
	q := psql.Insert(
		pim.Into("follows", "id", "follower_id", "target_id", "created_at"),
		pim.Values(psql.Arg(id, follower, target, at)),
		pim.OnConflict("follower_id", "target_id").DoNothing(),
	)
	res, err := bob.Exec(ctx, db, q)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// BuildBoth returns the SQL each dialect renders, for the report.
func BuildBoth() (sqliteSQL, pgSQL string, err error) {
	at := time.Unix(0, 0)
	s, _, err := sqlite.Insert(
		sim.Into("follows", "id", "follower_id", "target_id", "created_at"),
		sim.Values(sqlite.Arg("a", "b", "c", at.UnixMilli())),
		sim.OnConflict("follower_id", "target_id").DoNothing(),
	).Build(context.Background())
	if err != nil {
		return "", "", err
	}
	p, _, err := psql.Insert(
		pim.Into("follows", "id", "follower_id", "target_id", "created_at"),
		pim.Values(psql.Arg("a", "b", "c", at)),
		pim.OnConflict("follower_id", "target_id").DoNothing(),
	).Build(context.Background())
	return s, p, err
}
