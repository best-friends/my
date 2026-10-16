// SPDX-License-Identifier: AGPL-3.0-only

// Package schema embeds the desired-state schema files (ADR-0023) so that Go code can apply
// them without depending on the working directory.
//
// Atlas applies these files in every real deployment (`make db-apply`). Database adapter tests
// instead execute them directly on a new, empty database: on an empty database Atlas's plan
// consists of exactly the statements in the file, and CI's `make db-check` verifies, for both
// dialects, that Atlas applies the same file cleanly and leaves no diff.
package schema

import _ "embed"

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
