// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_postgres

package db_test

import (
	"os"
	"testing"

	"github.com/best-friends/my/internal/adapter/db/postgres"
	kvdb "github.com/best-friends/my/internal/adapter/kv/db"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/pkg/testkit"
	"github.com/best-friends/my/pkg/testkit/pgtest"
)

// TestMain stops the PostgreSQL server of package pgtest after the tests.
func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m))
}

// newDatabase returns the URL of a new database with the product schema on the test server
// (package pgtest).
func newDatabase(t *testing.T) string {
	t.Helper()
	return pgtest.NewDatabase(t)
}

func TestPostgresKeyValueStoreContract(t *testing.T) {
	testkit.KeyValueStoreContract(t, func(t *testing.T, clock *testkit.Clock) kernel.KeyValueStore {
		d, err := postgres.Open(t.Context(), newDatabase(t), uow.Options{})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		t.Cleanup(d.Close)
		return kvdb.NewPostgres(d, clock)
	})
}
