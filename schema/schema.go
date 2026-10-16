// SPDX-License-Identifier: AGPL-3.0-only

// Package schema embeds the desired-state schema files (ADR-0023) so that Go code can apply
// them without depending on the working directory.
//
// Atlas applies these files in every real deployment (`make db-apply`). Database adapter tests
// instead execute them directly on a new, empty database: on an empty database Atlas's plan
// consists of exactly the statements in the file, and CI's `make db-check` verifies, for both
// dialects, that Atlas applies the same file cleanly and leaves no diff.
//
// Tables lists the tables a file declares. The readiness check of the database adapters uses
// it to decide whether the schema is applied: every declared table exists.
package schema

import (
	_ "embed"
	"regexp"
)

// SQLite is schema/sqlite.sql.
//
//go:embed sqlite.sql
var SQLite string

// Postgres is schema/postgres.sql.
//
//go:embed postgres.sql
var Postgres string

// PostgresBootstrap is schema/postgres.bootstrap.sql, which runs before Postgres on every
// PostgreSQL database.
//
//go:embed postgres.bootstrap.sql
var PostgresBootstrap string

// createTable matches a CREATE TABLE statement that starts a line and captures the table name.
var createTable = regexp.MustCompile(`(?m)^CREATE TABLE (?:IF NOT EXISTS )?"?([A-Za-z_][A-Za-z0-9_]*)"? *\(`)

// Tables returns the names of the tables that the schema file ddl declares with CREATE TABLE,
// in file order.
func Tables(ddl string) []string {
	var names []string
	for _, m := range createTable.FindAllStringSubmatch(ddl, -1) {
		names = append(names, m[1])
	}
	return names
}
