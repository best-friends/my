// SPDX-License-Identifier: AGPL-3.0-only

// Package schema embeds the per-dialect DDL (schema/<dialect>.sql), which is
// also the input of sqlc.
package schema

import _ "embed"

//go:embed sqlite.sql
var SQLite string

//go:embed postgres.sql
var Postgres string
