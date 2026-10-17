// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// TestReportsThroughTheAPI files a report through the client API on the real database: it
// lands open in the moderators' list with its statuses and rules, it names the reporter
// neither in the response nor to the target, and statuses or rules that it may not cite are
// a 422.
func TestReportsThroughTheAPI(t *testing.T) {
	c := newStatusesClient(t, "alice", "bob", "carol")
	ctx := t.Context()
	rule, err := c.app.Instance.AddRule(ctx, appinstance.AddRuleInput{Texts: []instance.RuleText{{Locale: "en", Text: "Be kind"}}})
	if err != nil {
		t.Fatal(err)
	}
	retired, err := c.app.Instance.AddRule(ctx, appinstance.AddRuleInput{Texts: []instance.RuleText{{Locale: "en", Text: "Old"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.app.Instance.RetireRule(ctx, retired.ID, ""); err != nil {
		t.Fatal(err)
	}
	public := c.post("bob", map[string]any{"text": "rude", "visibility": "public"})
	private := c.post("bob", map[string]any{"text": "followers only", "visibility": "private"})
	other := c.post("carol", map[string]any{"text": "not bob's", "visibility": "public"})

	var raw map[string]any
	res := c.do("alice", http.MethodPost, "/reports", map[string]any{
		"account_id": c.ids["bob"], "status_ids": []string{public.Id}, "category": "violation",
		"rule_ids": []string{rule.ID}, "comment": "Please look", "forward": true,
	}, &raw)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /reports = %d", res.StatusCode)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), c.ids["alice"]) || strings.Contains(string(data), "reporter") {
		t.Errorf("report = %s, want no trace of the reporter", data)
	}
	var report clientapi.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.TargetAccount.Id != c.ids["bob"] || report.Forwarded || report.Category != clientapi.ReportCategoryViolation ||
		!reflect.DeepEqual(report.StatusIds, []string{public.Id}) || !reflect.DeepEqual(report.RuleIds, []string{rule.ID}) {
		t.Errorf("report = %+v, want bob's status and the rule, not forwarded to a local account", report)
	}

	var page clientapi.AdminReportPage
	if res := c.do("carol", http.MethodGet, "/admin/reports?state=open", nil, &page); res.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/reports = %d", res.StatusCode)
	}
	i := slices.IndexFunc(page.Items, func(r clientapi.AdminReport) bool { return r.Id == report.Id })
	if i < 0 {
		t.Fatalf("open reports = %+v, want %s", page.Items, report.Id)
	}
	if got := page.Items[i]; got.State != clientapi.AdminReportStateOpen || got.ReporterAccountId != c.ids["alice"] ||
		got.Comment != "Please look" || !reflect.DeepEqual(got.RuleIds, []string{rule.ID}) || !reflect.DeepEqual(got.StatusIds, []string{public.Id}) {
		t.Errorf("listed report = %+v, want it open with the reporter, statuses, and rules", got)
	}

	// Nothing tells the target.
	var notifications clientapi.NotificationPage
	if res := c.do("bob", http.MethodGet, "/notifications", nil, &notifications); res.StatusCode != http.StatusOK || len(notifications.Items) != 0 {
		t.Errorf("bob's notifications = %d %+v, want none", res.StatusCode, notifications.Items)
	}

	many := make([]string, 21)
	for i := range many {
		many[i] = fmt.Sprintf("0192f0c1-0000-7000-8000-%012d", i)
	}
	for name, body := range map[string]map[string]any{
		"a status alice may not see": {"account_id": c.ids["bob"], "status_ids": []string{private.Id}, "category": "spam"},
		"another account's status":   {"account_id": c.ids["bob"], "status_ids": []string{other.Id}, "category": "spam"},
		"rules of another category":  {"account_id": c.ids["bob"], "category": "spam", "rule_ids": []string{rule.ID}},
		"a retired rule":             {"account_id": c.ids["bob"], "category": "violation", "rule_ids": []string{retired.ID}},
		"the reporter's own account": {"account_id": c.ids["alice"], "category": "spam"},
		"a comment that is too long": {"account_id": c.ids["bob"], "category": "other", "comment": strings.Repeat("a", 1001)},
		"an unknown category":        {"account_id": c.ids["bob"], "category": "abuse"},
		"an unknown account":         {"account_id": "0192f0c1-0000-7000-8000-000000000000", "category": "spam"},
		"more than twenty statuses":  {"account_id": c.ids["bob"], "category": "spam", "status_ids": many},
	} {
		if res := c.do("alice", http.MethodPost, "/reports", body, nil); res.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("POST /reports with %s = %d, want 422", name, res.StatusCode)
		}
	}
	if page = (clientapi.AdminReportPage{}); c.do("carol", http.MethodGet, "/admin/reports", nil, &page).StatusCode != http.StatusOK || len(page.Items) != 1 {
		t.Errorf("reports = %+v, want only the filed one", page.Items)
	}
}
