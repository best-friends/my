// SPDX-License-Identifier: AGPL-3.0-only

package dbtrace_test

import (
	"testing"

	"github.com/mycroblog/my/internal/adapter/telemetry/dbtrace"
)

func TestOperation(t *testing.T) {
	for query, want := range map[string]string{
		"-- name: InsertJob :exec\nINSERT INTO jobs (id) VALUES (?)": "InsertJob",
		"  -- name: ClaimJob :one\nUPDATE jobs SET x = 1":            "ClaimJob",
		"-- name: bad-name :one\nSELECT 1":                           "_OTHER",
		"begin isolation level read committed":                       "BEGIN",
		"SELECT name FROM sqlite_schema WHERE type = 'table'":        "SELECT",
		"commit":     "COMMIT",
		"":           "_OTHER",
		"(SELECT 1)": "_OTHER",
		"SELECTWITHAVERYLONGKEYWORDTHATISNOTSQL 1":               "_OTHER",
		"-- a comment\nSELECT * FROM users WHERE name = 'alice'": "_OTHER",
	} {
		if got := dbtrace.Operation(query); got != want {
			t.Errorf("Operation(%q) = %q, want %q", query, got, want)
		}
	}
}
