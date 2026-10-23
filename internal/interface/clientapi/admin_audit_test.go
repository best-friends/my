// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	appmoderation "github.com/mycroblog/my/internal/app/moderation"
	"github.com/mycroblog/my/internal/domain/identity"
	"github.com/mycroblog/my/internal/domain/moderation"
	"github.com/mycroblog/my/internal/interface/clientapi"
	"github.com/mycroblog/my/internal/interface/clientapi/apitest"
	"github.com/mycroblog/my/pkg/testkit"
)

// fakeAuditLog keeps entries oldest first and lists them like appmoderation.Service.AuditLog,
// with the targets in gone erased.
type fakeAuditLog struct {
	entries []moderation.AuditEntry
	queries []appmoderation.AuditQuery
	gone    map[appmoderation.AuditTarget]bool
	lookups int
}

// userCarol is a user that no account belongs to.
var userCarol = testkit.NamedID("user-carol")

func (f *fakeAuditLog) GoneAuditTargets(_ context.Context, entries []moderation.AuditEntry) (map[appmoderation.AuditTarget]bool, error) {
	f.lookups++
	out := map[appmoderation.AuditTarget]bool{}
	for _, e := range entries {
		if t := (appmoderation.AuditTarget{Kind: e.TargetKind, ID: e.TargetID}); f.gone[t] {
			out[t] = true
		}
	}
	return out, nil
}

func (f *fakeAuditLog) AuditLog(_ context.Context, q appmoderation.AuditQuery, limit int) ([]moderation.AuditEntry, error) {
	f.queries = append(f.queries, q)
	var out []moderation.AuditEntry
	for i := len(f.entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := f.entries[i]
		if (q.Before == "" || e.ID < q.Before) && (q.ActorUserID == "" || e.ActorUserID == q.ActorUserID) &&
			(q.Action == "" || e.Action == q.Action) && (q.TargetKind == "" || e.TargetKind == q.TargetKind) &&
			(q.TargetID == "" || e.TargetID == q.TargetID) {
			out = append(out, e)
		}
	}
	return out, nil
}

const goneAccountID = "01890a5d-ac96-774b-bcce-b302099a5999"

// newAuditEnv is newAuthEnv with an audit log of three entries: alice's domain action, the CLI's
// resolution of a report that is gone, and a rule added by a user whose account is gone. alice holds every
// permission, carol (bob's account) only the moderator's, and bob none.
func newAuditEnv(t *testing.T) (*authEnv, *fakeAuditLog) {
	t.Helper()
	at := func(m int) time.Time { return adminNow.Add(time.Duration(m) * time.Minute) }
	f := &fakeAuditLog{entries: []moderation.AuditEntry{
		{
			ID: "01890a5d-ac96-774b-bcce-b302099a7001", ActorKind: moderation.AuditActorUser, ActorUserID: userOf(aliceAccountID),
			Action: moderation.AuditDomainActionIssued, TargetKind: moderation.AuditTargetDomain, TargetID: "bad.example",
			TargetLabel: "bad.example", SourceEventID: "01890a5d-ac96-774b-bcce-b302099a6001", OccurredAt: at(1),
		},
		{
			ID: "01890a5d-ac96-774b-bcce-b302099a7002", ActorKind: moderation.AuditActorCLI,
			Action: moderation.AuditReportResolved, TargetKind: moderation.AuditTargetReport, TargetID: reportOne,
			TargetLabel: "@spammer@remote.example", SourceEventID: "01890a5d-ac96-774b-bcce-b302099a6002", OccurredAt: at(2),
		},
		{
			ID: "01890a5d-ac96-774b-bcce-b302099a7003", ActorKind: moderation.AuditActorUser, ActorUserID: userOf(goneAccountID),
			Action: moderation.AuditRuleAdded, TargetKind: moderation.AuditTargetRule, TargetID: "01890a5d-ac96-774b-bcce-b302099a6003",
			TargetLabel: "Be kind", OccurredAt: at(3),
		},
	}, gone: map[appmoderation.AuditTarget]bool{{Kind: moderation.AuditTargetReport, ID: reportOne}: true}}
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{
			userOf(aliceAccountID): identity.AllPermissions(),
			userCarol:              {identity.PermManageReports, identity.PermViewAudit},
		}
		o.AuditLog = f
		o.UserAccounts = fakeUserAccounts{}
	})
	e.auth.tokens["admin"] = bearer("admin", aliceAccountID)
	e.auth.tokens["admin-read"] = bearer("admin:read", aliceAccountID)
	e.auth.tokens["bob-admin"] = bearer("admin", bobAccountID)
	carol := bearer("admin:read", bobAccountID)
	carol.UserID = userCarol
	e.auth.tokens["carol"] = carol
	return e, f
}

func auditIDs(p clientapi.AdminAuditEntryPage) []string {
	out := []string{}
	for _, it := range p.Items {
		out = append(out, it.Id)
	}
	return out
}

func TestAuditLogNeedsViewAudit(t *testing.T) {
	e, f := newAuditEnv(t)
	res := e.send(t, http.MethodGet, "/admin/audit_log", "bob-admin", "")
	if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeMissingPermission ||
		p.Detail == nil || !strings.Contains(*p.Detail, "view_audit") {
		t.Errorf("audit log by bob = %d %+v, want 403 for view_audit", res.StatusCode, p)
	}
	if len(f.queries) != 0 {
		t.Error("a refused request read the audit log")
	}
	// A moderator's role grants view_audit.
	res = e.send(t, http.MethodGet, "/admin/audit_log", "carol", "")
	if page := decodeJSON[clientapi.AdminAuditEntryPage](t, res); res.StatusCode != http.StatusOK || len(page.Items) != 3 {
		t.Errorf("audit log by a moderator = %d %+v", res.StatusCode, page)
	}
}

func TestListAuditLog(t *testing.T) {
	e, f := newAuditEnv(t)
	res := e.send(t, http.MethodGet, "/admin/audit_log?limit=2", "admin-read", "")
	page := decodeJSON[clientapi.AdminAuditEntryPage](t, res)
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" || page.NextCursor == nil ||
		!slices.Equal(auditIDs(page), []string{f.entries[2].ID, f.entries[1].ID}) {
		t.Fatalf("first page = %d (%s) %+v", res.StatusCode, res.Header.Get("Cache-Control"), page)
	}
	gone, cli := page.Items[0], page.Items[1]
	// The actor whose account is gone keeps the ID of the account, without its presentation.
	if gone.ActorKind != clientapi.AdminAuditActorKindUser || gone.ActorAccountId == nil || *gone.ActorAccountId != goneAccountID ||
		gone.ActorAccount != nil || gone.SourceEventId != nil || gone.TargetLabel != "Be kind" || !gone.TargetExists {
		t.Errorf("entry by a gone account = %+v", gone)
	}
	if cli.ActorKind != clientapi.AdminAuditActorKindCli || cli.ActorAccountId != nil || cli.Action != clientapi.AdminAuditActionReportResolved ||
		cli.TargetKind != clientapi.AdminAuditTargetKindReport || cli.TargetId != reportOne || cli.TargetLabel != "@spammer@remote.example" || cli.TargetExists ||
		cli.SourceEventId == nil || *cli.SourceEventId != f.entries[1].SourceEventID || !cli.OccurredAt.Equal(f.entries[1].OccurredAt) {
		t.Errorf("CLI entry = %+v", cli)
	}
	// The targets of a page are looked up once.
	if f.lookups != 1 {
		t.Errorf("targets looked up %d times for one page, want 1", f.lookups)
	}
	res = e.send(t, http.MethodGet, "/admin/audit_log?limit=2&cursor="+*page.NextCursor, "admin-read", "")
	rest := decodeJSON[clientapi.AdminAuditEntryPage](t, res)
	if !slices.Equal(auditIDs(rest), []string{f.entries[0].ID}) || rest.NextCursor != nil {
		t.Fatalf("second page = %+v", rest)
	}
	if a := rest.Items[0]; a.ActorAccountId == nil || *a.ActorAccountId != aliceAccountID || a.ActorAccount == nil || a.ActorAccount.Username != "alice" || !a.TargetExists {
		t.Errorf("alice's entry = %+v", a)
	}

	// Filters: by the actor's account, the action, and the target.
	for query, want := range map[string][]string{
		"actor_account_id=" + aliceAccountID:                               {f.entries[0].ID},
		"actor_account_id=" + bobAccountID:                                 {}, // not a known account
		"action=report_resolved":                                           {f.entries[1].ID},
		"target_kind=rule":                                                 {f.entries[2].ID},
		"target_kind=domain&target_id=bad.example":                         {f.entries[0].ID},
		"target_kind=domain&target_id=good.example":                        {},
		"target_kind=report&action=report_resolved&target_id=" + reportOne: {f.entries[1].ID},
	} {
		res := e.send(t, http.MethodGet, "/admin/audit_log?"+query, "admin-read", "")
		if got := decodeJSON[clientapi.AdminAuditEntryPage](t, res); res.StatusCode != http.StatusOK || !slices.Equal(auditIDs(got), want) {
			t.Errorf("?%s = %d %v, want %v", query, res.StatusCode, auditIDs(got), want)
		}
	}
	if !slices.ContainsFunc(f.queries, func(q appmoderation.AuditQuery) bool {
		return q.ActorUserID == "" && q.TargetKind == moderation.AuditTargetReport && q.TargetID == reportOne
	}) {
		t.Errorf("queries = %+v, want one on the report", f.queries)
	}
	if !slices.ContainsFunc(f.queries, func(q appmoderation.AuditQuery) bool { return q.ActorUserID == userOf(aliceAccountID) }) {
		t.Errorf("queries = %+v, want one by alice's user", f.queries)
	}

	for query, name := range map[string]string{
		"action=domain_burned":          "action",
		"target_kind=planet":            "target_kind",
		"target_id=bad.example":         "target_id",
		"target_kind=domain&target_id=": "target_id",
		"limit=0":                       "limit",
		"cursor=%25":                    "cursor",
	} {
		res := e.send(t, http.MethodGet, "/admin/audit_log?"+query, "admin-read", "")
		if p := decodeProblem(t, res); res.StatusCode != http.StatusBadRequest || p.Detail == nil || !strings.Contains(*p.Detail, name) {
			t.Errorf("?%s = %d %+v, want 400 naming %s", query, res.StatusCode, p, name)
		}
	}
}

// The audit log has no operation that changes or deletes it.
func TestAuditLogIsReadOnly(t *testing.T) {
	e, _ := newAuditEnv(t)
	for path, item := range e.spec.Doc().Paths.Map() {
		if !strings.Contains(path, "audit") {
			continue
		}
		for method := range item.Operations() {
			if method != http.MethodGet {
				t.Errorf("%s %s changes the audit log", method, path)
			}
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		// Outside the spec, so not through e.send, which validates the response against it.
		req := httptest.NewRequest(method, clientapi.BasePath+"/admin/audit_log", nil)
		req.Header.Set("Authorization", "Bearer admin")
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Errorf("%s /admin/audit_log = %d, want 404 or 405", method, rec.Code)
		}
	}
}

// Every audited operation of the domain is in the API's enum, and every kind of target too.
func TestAuditEnumsMatchTheDomain(t *testing.T) {
	spec := newSpecEnum(t, "AdminAuditAction")
	var actions []string
	for _, a := range moderation.AuditActions() {
		actions = append(actions, string(a))
	}
	if !slices.Equal(spec, actions) {
		t.Errorf("AdminAuditAction = %v, want %v", spec, actions)
	}
	var kinds []string
	for _, k := range moderation.AuditTargetKinds() {
		kinds = append(kinds, string(k))
	}
	if spec := newSpecEnum(t, "AdminAuditTargetKind"); !slices.Equal(spec, kinds) {
		t.Errorf("AdminAuditTargetKind = %v, want %v", spec, kinds)
	}
}

func newSpecEnum(t *testing.T, schema string) []string {
	t.Helper()
	ref := apitest.Load(t).Doc().Components.Schemas[schema]
	if ref == nil || ref.Value == nil {
		t.Fatalf("no schema %s", schema)
	}
	var out []string
	for _, v := range ref.Value.Enum {
		out = append(out, v.(string))
	}
	return out
}
