// SPDX-License-Identifier: AGPL-3.0-only

package sqlitedb_test

// Evidence for the report (docs/spikes/17-sqlc-dialects.md). These tests log
// observations; they assert only what the adapter relies on.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/adapter/sqlitedb"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/app"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/internal/txretry"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

var drivers = []sqlitedb.Driver{sqlitedb.Modernc, sqlitedb.Ncruces}

// TestPragmasApplied checks that the DSN pragmas reach every pooled connection.
func TestPragmasApplied(t *testing.T) {
	for _, drv := range drivers {
		db, err := sqlitedb.Open(drv, filepath.Join(t.TempDir(), "p.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = db.SQL().Close() }()
		var mode, version string
		var fk, busy int
		row := db.SQL().QueryRowContext(t.Context(),
			"SELECT (SELECT journal_mode FROM pragma_journal_mode), (SELECT foreign_keys FROM pragma_foreign_keys), (SELECT timeout FROM pragma_busy_timeout), sqlite_version()")
		if err := row.Scan(&mode, &fk, &busy, &version); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: sqlite %s journal_mode=%s foreign_keys=%d busy_timeout=%d", drv, version, mode, fk, busy)
		if mode != "wal" || fk != 1 || busy != 5000 {
			t.Errorf("%s: pragmas not applied", drv)
		}
	}
}

// TestDeferredTxNeedsRetry shows why the adapter uses BEGIN IMMEDIATE: with a
// deferred transaction, the read->write lock upgrade fails with SQLITE_BUSY
// without waiting for busy_timeout, so only the retry loop saves us.
func TestDeferredTxNeedsRetry(t *testing.T) {
	for _, drv := range drivers {
		for _, lock := range []string{"deferred", "immediate"} {
			t.Run(string(drv)+"/"+lock, func(t *testing.T) {
				db, err := sqlitedb.OpenTxLock(drv, filepath.Join(t.TempDir(), "d.db"), lock)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = db.SQL().Close() }()
				if err := db.Migrate(t.Context()); err != nil {
					t.Fatal(err)
				}
				var retries atomic.Int64
				db.Policy = txretry.Policy{MaxAttempts: 100, OnRetry: func(int, error) { retries.Add(1) }}
				target := upsert(t, db, "target")
				svc := &app.Follows{UoW: db}
				const n = 16
				fans := make([]store.Account, n)
				for i := range fans {
					fans[i] = upsert(t, db, fmt.Sprintf("fan%d", i))
				}
				var wg sync.WaitGroup
				var failed atomic.Int64
				for i := range n {
					wg.Go(func() {
						if _, err := svc.Follow(context.Background(), fans[i].ID, target.ID); err != nil {
							failed.Add(1)
							t.Log(err)
						}
					})
				}
				wg.Wait()
				t.Logf("%s _txlock=%s: %d concurrent follows, retries=%d, failures=%d", drv, lock, n, retries.Load(), failed.Load())
				if failed.Load() != 0 {
					t.Fail()
				}
			})
		}
	}
}

func upsert(t *testing.T, u store.UnitOfWork, name string) store.Account {
	t.Helper()
	now := store.Timestamp(time.Now())
	a := store.Account{ID: uuid.NewV7().String(), Username: name, URI: "https://x/" + name, CreatedAt: now, UpdatedAt: now}
	var got store.Account
	if err := u.Tx(t.Context(), func(tx store.Tx) error {
		var err error
		got, err = tx.Accounts().Upsert(t.Context(), a)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

// TestTimeBindingBehaviour records what each driver does when a time.Time is
// bound directly (without the adapter's unix-ms conversion).
func TestTimeBindingBehaviour(t *testing.T) {
	jst := time.FixedZone("JST", 9*3600)
	values := []time.Time{
		time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 16, 12, 0, 0, 500_000_000, time.UTC),
		time.Date(2026, 10, 16, 20, 0, 0, 100_000_000, jst), // 11:00:00.1Z, earlier than both above
	}
	for _, drv := range drivers {
		db, err := sqlitedb.Open(drv, ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = db.SQL().Close() }()
		ctx := t.Context()
		if _, err := db.SQL().ExecContext(ctx, `CREATE TABLE tt (n INTEGER, t_text TEXT, t_dt DATETIME, t_int INTEGER)`); err != nil {
			t.Fatal(err)
		}
		for i, v := range values {
			if _, err := db.SQL().ExecContext(ctx, `INSERT INTO tt VALUES (?, ?, ?, ?)`, i, v, v, v); err != nil {
				t.Fatalf("%s: insert: %v", drv, err)
			}
		}
		rows, err := db.SQL().QueryContext(ctx, `SELECT n, typeof(t_text), t_text, typeof(t_int), t_int, t_dt FROM tt ORDER BY n`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var n int
			var ty1, ty2 string
			var text, intv any
			var dt any
			if err := rows.Scan(&n, &ty1, &text, &ty2, &intv, &dt); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: row%d TEXT col: %s %q | INTEGER col: %s %v | DATETIME col scans as %T", drv, n, ty1, fmt.Sprint(text), ty2, intv, dt)
		}
		_ = rows.Close()
		var order string
		if err := db.SQL().QueryRowContext(ctx, `SELECT group_concat(n, ',') FROM (SELECT n FROM tt ORDER BY t_text)`).Scan(&order); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: ORDER BY t_text => %s (chronological is 2,0,1)", drv, order)

		var back time.Time
		err = db.SQL().QueryRowContext(ctx, `SELECT t_text FROM tt WHERE n = 2`).Scan(&back)
		t.Logf("%s: scan TEXT column into time.Time: %v err=%v", drv, back, errOrNil(err))

		if _, err := db.SQL().ExecContext(ctx, `CREATE TABLE ts (t INTEGER) STRICT`); err != nil {
			t.Fatal(err)
		}
		_, err = db.SQL().ExecContext(ctx, `INSERT INTO ts VALUES (?)`, values[0])
		t.Logf("%s: bind time.Time into STRICT INTEGER column: err=%v", drv, err)
		if err == nil {
			t.Errorf("%s: STRICT table accepted a time.Time as INTEGER", drv)
		}
	}
}

func errOrNil(err error) any {
	if err == nil {
		return "<nil>"
	}
	return errors.Unwrap(err)
}

// TestUUIDv7 checks the stdlib uuid package (Go 1.27): monotonic within a
// millisecond and lexicographically sortable as a string.
func TestUUIDv7(t *testing.T) {
	const n = 200_000
	prev := uuid.NewV7().String()
	start := time.Now()
	for range n {
		cur := uuid.NewV7().String()
		if cur <= prev {
			t.Fatalf("not increasing: %s <= %s", cur, prev)
		}
		prev = cur
	}
	t.Logf("stdlib uuid.NewV7: %d strictly increasing IDs in %v (%v/op)", n, time.Since(start), time.Since(start)/n)
}

// BenchmarkFollow measures one Follow use case (2 reads + 1 insert + 1-row
// updates in one IMMEDIATE transaction) on a WAL file database.
func BenchmarkFollow(b *testing.B) {
	for _, drv := range drivers {
		b.Run(string(drv), func(b *testing.B) {
			db, err := sqlitedb.Open(drv, filepath.Join(b.TempDir(), "b.db"))
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = db.SQL().Close() }()
			if err := db.Migrate(b.Context()); err != nil {
				b.Fatal(err)
			}
			mk := func(name string) store.Account {
				now := store.Timestamp(time.Now())
				a := store.Account{ID: uuid.NewV7().String(), Username: name, URI: "https://x/" + uuid.NewV4().String(), CreatedAt: now, UpdatedAt: now}
				var got store.Account
				if err := db.Tx(b.Context(), func(tx store.Tx) error {
					var err error
					got, err = tx.Accounts().Upsert(b.Context(), a)
					return err
				}); err != nil {
					b.Fatal(err)
				}
				return got
			}
			target := mk("target")
			fans := make([]store.Account, b.N)
			for i := range fans {
				fans[i] = mk("fan")
			}
			svc := &app.Follows{UoW: db}
			b.ResetTimer()
			for i := range b.N {
				if _, err := svc.Follow(b.Context(), fans[i].ID, target.ID); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
