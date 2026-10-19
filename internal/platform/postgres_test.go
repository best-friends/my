// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"os"
	"testing"

	"github.com/best-friends/my/pkg/testkit/pgtest"
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m))
}

// newPostgresDatabase returns the URL of a new database with the product schema on the test server
// (package pgtest), for the few tests of this package that run on both dialects; most tests use
// SQLite and never start the server.
func newPostgresDatabase(t *testing.T) string {
	t.Helper()
	return pgtest.NewDatabase(t)
}
