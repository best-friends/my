// SPDX-License-Identifier: AGPL-3.0-only

package schema_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/best-friends/my/internal/app/supporters"
	"github.com/best-friends/my/internal/domain/finance"
	"github.com/best-friends/my/internal/domain/instance"
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

// The CHECK constraint of instance_theme_tokens.token lists exactly the tokens a theme may override,
// which are generated from the web client's theme.css (ADR-0059).
func TestThemeTokenCheckMatchesTheTokenList(t *testing.T) {
	var want []string
	for _, tok := range instance.ThemeTokens() {
		want = append(want, tok.Name)
	}
	check := regexp.MustCompile(`(?s)CREATE TABLE instance_theme_tokens \(.*?token [^\n]*CHECK \(token IN \((.*?)\)\)`)
	for name, sql := range map[string]string{"sqlite": schema.SQLite, "postgres": schema.Postgres} {
		m := check.FindStringSubmatch(sql)
		if m == nil {
			t.Fatalf("%s: no CHECK on instance_theme_tokens.token", name)
		}
		var got []string
		for _, q := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m[1], -1) {
			got = append(got, q[1])
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: CHECK lists %v, want %v", name, got, want)
		}
	}
}

// The CHECK constraint of cost_item_label_revisions.category lists exactly the categories of the
// finance domain (ADR-0067 §2).
func TestCostCategoryCheckMatchesTheDomain(t *testing.T) {
	var want []string
	for _, c := range finance.Categories() {
		want = append(want, string(c))
	}
	check := regexp.MustCompile(`(?s)CREATE TABLE cost_item_label_revisions \(.*?category [^\n]*CHECK \(category IN \((.*?)\)\)`)
	for name, sql := range map[string]string{"sqlite": schema.SQLite, "postgres": schema.Postgres} {
		m := check.FindStringSubmatch(sql)
		if m == nil {
			t.Fatalf("%s: no CHECK on cost_item_label_revisions.category", name)
		}
		var got []string
		for _, q := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m[1], -1) {
			got = append(got, q[1])
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: CHECK lists %v, want %v", name, got, want)
		}
	}
}

// The CHECK constraint of payment_reconcile_runs.error_kind lists exactly the kinds of failure of
// the supporters module (ADR-0074 §4).
func TestPaymentErrorKindCheckMatchesTheModule(t *testing.T) {
	var want []string
	for _, k := range supporters.ErrorKinds() {
		want = append(want, string(k))
	}
	check := regexp.MustCompile(`(?s)CREATE TABLE payment_reconcile_runs \(.*?error_kind [^\n]*CHECK \(error_kind IN \((.*?)\)\)`)
	for name, sql := range map[string]string{"sqlite": schema.SQLite, "postgres": schema.Postgres} {
		m := check.FindStringSubmatch(sql)
		if m == nil {
			t.Fatalf("%s: no CHECK on payment_reconcile_runs.error_kind", name)
		}
		var got []string
		for _, q := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m[1], -1) {
			got = append(got, q[1])
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: CHECK lists %v, want %v", name, got, want)
		}
	}
}
