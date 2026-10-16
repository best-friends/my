// SPDX-License-Identifier: AGPL-3.0-only

package schema_test

import (
	"slices"
	"testing"

	"github.com/best-friends/my/schema"
)

func TestTables(t *testing.T) {
	got := schema.Tables("-- CREATE TABLE commented (\nCREATE TABLE users (\n  id TEXT\n) STRICT;\n" +
		"CREATE TABLE IF NOT EXISTS \"jobs\" (\n);\nCREATE INDEX users_id ON users (id);\n")
	if want := []string{"users", "jobs"}; !slices.Equal(got, want) {
		t.Errorf("Tables = %v, want %v", got, want)
	}
}

// Both dialects declare the same tables, so that readiness means the same on each.
func TestTablesMatchAcrossDialects(t *testing.T) {
	sqlite, postgres := schema.Tables(schema.SQLite), schema.Tables(schema.Postgres)
	if len(sqlite) == 0 {
		t.Fatal("schema/sqlite.sql declares no tables")
	}
	if !slices.Equal(sqlite, postgres) {
		t.Errorf("tables differ:\n sqlite:   %v\n postgres: %v", sqlite, postgres)
	}
}
