// SPDX-License-Identifier: AGPL-3.0-only

package db_test

import (
	"os"
	"testing"

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
