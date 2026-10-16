// SPDX-License-Identifier: AGPL-3.0-only

// Package contract holds the test-only probe table of the UnitOfWork contract tests for SQLite.
// Its queries are generated into contractdb by sqlc (see sqlc.yaml).
package contract

import _ "embed"

// Schema is the DDL of the probe table, applied on top of schema/sqlite.sql.
//
//go:embed schema.sql
var Schema string
