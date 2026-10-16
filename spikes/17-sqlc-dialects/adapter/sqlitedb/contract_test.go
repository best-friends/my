// SPDX-License-Identifier: AGPL-3.0-only

package sqlitedb_test

import (
	"path/filepath"
	"testing"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/adapter/sqlitedb"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/contract"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/internal/txretry"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

func factory(drv sqlitedb.Driver, memory bool) contract.Factory {
	return func(t *testing.T, p txretry.Policy) store.UnitOfWork {
		path := ":memory:"
		if !memory {
			path = filepath.Join(t.TempDir(), "test.db")
		}
		db, err := sqlitedb.Open(drv, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.SQL().Close() })
		if err := db.Migrate(t.Context()); err != nil {
			t.Fatal(err)
		}
		db.Policy = p
		return db
	}
}

func TestContract(t *testing.T) {
	for _, drv := range []sqlitedb.Driver{sqlitedb.Modernc, sqlitedb.Ncruces} {
		t.Run(string(drv)+"/file", func(t *testing.T) { contract.Run(t, factory(drv, false)) })
		t.Run(string(drv)+"/memory", func(t *testing.T) { contract.Run(t, factory(drv, true)) })
	}
}
