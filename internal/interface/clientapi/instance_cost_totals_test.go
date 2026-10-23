// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	appfinance "github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/internal/domain/finance"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// publicCosts is fakeCosts whose totals carry a breakdown, recording the queries.
type publicCosts struct {
	*fakeCosts
	queries []appfinance.TotalsQuery
}

func (f *publicCosts) Totals(ctx context.Context, q appfinance.TotalsQuery) ([]finance.PeriodTotal, error) {
	f.queries = append(f.queries, q)
	out, err := f.fakeCosts.Totals(ctx, q)
	for i := range out {
		out[i].Breakdown = []finance.Line{{
			Key:    finance.BreakdownKey{Provider: "registrar", Category: finance.CategoryDomain, Currency: "USD"},
			Amount: 1234,
		}}
	}
	return out, err
}

// settingsInstance is fakeSettings that also serves the instance's texts (none).
type settingsInstance struct{ *fakeSettings }

func (settingsInstance) Text(_ context.Context, key instance.TextKey) (instance.Text, bool, error) {
	return instance.Text{Key: key}, false, nil
}

// TestInstanceCostTotals checks what GET /instance/cost_totals publishes at each level of
// finance_visibility (FR-903, ADR-0067 §8), and that only manage_settings changes the level.
func TestInstanceCostTotals(t *testing.T) {
	start, _ := finance.ParseDate("2026-04-10")
	costs := &publicCosts{fakeCosts: &fakeCosts{item: finance.CostItem{
		ID: "0199f0e0-0000-7000-8000-0000000000c1", Recurrence: finance.Recurrence{Schedule: finance.ScheduleYears, Interval: 1},
		StartsOn: start, Label: finance.Label{Name: "example.com", Provider: "registrar", Category: finance.CategoryDomain, Memo: "bucket-tokyo-1"},
		Prices: []finance.PriceRevision{{EffectiveOn: start, Price: finance.Money{Minor: 1650, Currency: "JPY"}}},
	}}}
	settings := settingsInstance{&fakeSettings{}}
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{
			userOf(aliceAccountID): {identity.PermManageSettings},
			userOf(bobAccountID):   {identity.PermManageFinance},
		}
		o.Costs, o.Settings, o.Instance = costs, settings.fakeSettings, settings
	})
	e.auth.tokens["alice-admin"] = bearer("admin", aliceAccountID)
	e.auth.tokens["bob-admin"] = bearer("admin", bobAccountID)
	const path = "/instance/cost_totals?granularity=month&from=2026-04-01&to=2026-05-31"

	visibility := func() clientapi.FinanceVisibility {
		t.Helper()
		return decode[clientapi.Instance](t, e.send(t, http.MethodGet, "/instance", "", "")).FinanceVisibility
	}
	// read fetches the public totals anonymously and checks that the answer names no item,
	// memo, or payment.
	read := func() clientapi.CostTotals {
		t.Helper()
		res := e.send(t, http.MethodGet, path, "", "")
		if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "public, max-age=60" {
			t.Fatalf("GET %s = %d, Cache-Control %q", path, res.StatusCode, res.Header.Get("Cache-Control"))
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"example.com", "bucket-tokyo-1", "1650", costs.item.ID, `"name"`, `"memo"`, `"prices"`} {
			if strings.Contains(string(body), secret) {
				t.Errorf("public totals contain %s: %s", secret, body)
			}
		}
		var out clientapi.CostTotals
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	set := func(token, value string) int {
		t.Helper()
		return e.send(t, http.MethodPatch, "/admin/settings", token, `{"values":{"finance_visibility":"`+value+`"}}`).StatusCode
	}

	// private by default: no totals, a 404.
	if got := visibility(); got != clientapi.FinanceVisibilityPrivate {
		t.Errorf("finance_visibility = %q, want private", got)
	}
	if res := e.send(t, http.MethodGet, path, "", ""); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET %s while private = %d, want 404", path, res.StatusCode)
	}
	if res := e.send(t, http.MethodGet, path, "bob-admin", ""); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET %s while private as manage_finance = %d, want 404", path, res.StatusCode)
	}
	if len(costs.queries) != 0 {
		t.Errorf("private totals were computed: %+v", costs.queries)
	}

	// manage_finance alone does not change what is published.
	if got := set("bob-admin", "breakdown"); got != http.StatusForbidden {
		t.Errorf("PATCH finance_visibility without manage_settings = %d, want 403", got)
	}
	if got := visibility(); got != clientapi.FinanceVisibilityPrivate {
		t.Errorf("finance_visibility after a forbidden change = %q, want private", got)
	}
	if got := set("alice-admin", "everything"); got != http.StatusUnprocessableEntity {
		t.Errorf("PATCH finance_visibility everything = %d, want 422", got)
	}

	// totals: the totals by period, without the breakdown or the providers' rules.
	if got := set("alice-admin", "totals"); got != http.StatusOK {
		t.Fatalf("PATCH finance_visibility totals = %d", got)
	}
	if got := visibility(); got != clientapi.FinanceVisibilityTotals {
		t.Errorf("finance_visibility = %q, want totals", got)
	}
	totals := read()
	if len(totals.Periods) != 2 || totals.Periods[0].Totals[0] != (clientapi.Money{Amount: "12.34", Currency: "USD"}) {
		t.Errorf("totals = %+v", totals)
	}
	for _, p := range totals.Periods {
		if len(p.Breakdown) != 0 {
			t.Errorf("breakdown of %s with totals = %+v", p.Label, p.Breakdown)
		}
	}
	if len(totals.Rules.Sources) != 0 || totals.Rules.WeekStart != "monday" {
		t.Errorf("rules with totals = %+v", totals.Rules)
	}
	if q := costs.queries[len(costs.queries)-1]; !q.UntilToday {
		t.Errorf("public query = %+v, want UntilToday", q)
	}

	// breakdown: the totals and the breakdown by provider and category.
	if got := set("alice-admin", "breakdown"); got != http.StatusOK {
		t.Fatalf("PATCH finance_visibility breakdown = %d", got)
	}
	totals = read()
	if len(totals.Periods) != 2 || len(totals.Periods[0].Breakdown) != 1 {
		t.Fatalf("totals with breakdown = %+v", totals)
	}
	if line := totals.Periods[0].Breakdown[0]; line.Provider != "registrar" || line.Category != clientapi.CostCategoryDomain ||
		line.Amount != (clientapi.Money{Amount: "12.34", Currency: "USD"}) {
		t.Errorf("breakdown line = %+v", line)
	}
	if len(totals.Rules.Sources) != 1 || totals.Rules.Sources[0].Provider != "Google Cloud" {
		t.Errorf("rules with breakdown = %+v", totals.Rules)
	}

	// The query is checked like the admin one.
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/instance/cost_totals?granularity=day&from=2026-01-01&to=2026-01-02", http.StatusBadRequest},
		{"/instance/cost_totals?granularity=month&from=2026-01-01&to=2031-01-01", http.StatusUnprocessableEntity},
		{"/instance/cost_totals?granularity=month&from=2026-02-30&to=2026-03-01", http.StatusBadRequest},
	} {
		if res := e.send(t, http.MethodGet, tc.path, "", ""); res.StatusCode != tc.status {
			t.Errorf("GET %s = %d, want %d", tc.path, res.StatusCode, tc.status)
		}
	}
}
