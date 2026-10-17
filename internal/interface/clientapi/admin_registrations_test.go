// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeRegistrations keeps registrations by user with the semantics of the identity use cases:
// a decision applies to a pending registration only.
type fakeRegistrations struct {
	entries []appidentity.RegistrationEntry
	decided []appidentity.DecideInput
}

func (f *fakeRegistrations) ListRegistrationsAfter(_ context.Context, state identity.RegistrationState, after appidentity.RegistrationPosition, limit int) ([]appidentity.RegistrationEntry, error) {
	var out []appidentity.RegistrationEntry
	for _, e := range f.entries {
		p := e.Position()
		if e.State.State != state || (after.UserID != "" && (p.Since.Before(after.Since) || (p.Since.Equal(after.Since) && p.UserID <= after.UserID))) {
			continue
		}
		if out = append(out, e); len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeRegistrations) find(userID string) (int, bool) {
	for i, e := range f.entries {
		if e.Application.UserID == userID {
			return i, true
		}
	}
	return 0, false
}

func (f *fakeRegistrations) Registration(_ context.Context, userID string) (appidentity.RegistrationEntry, error) {
	i, ok := f.find(userID)
	if !ok {
		return appidentity.RegistrationEntry{}, appidentity.ErrNotFound
	}
	return f.entries[i], nil
}

func (f *fakeRegistrations) DecideRegistration(_ context.Context, in appidentity.DecideInput) (identity.RegistrationDecision, error) {
	if err := identity.ValidateDecisionNote(in.Note); err != nil {
		return identity.RegistrationDecision{}, err
	}
	i, ok := f.find(in.UserID)
	if !ok {
		return identity.RegistrationDecision{}, appidentity.ErrNotFound
	}
	if f.entries[i].State.State != identity.RegistrationPending {
		return identity.RegistrationDecision{}, appidentity.ErrNotPending
	}
	f.entries[i].State.State = in.Decision.State()
	f.decided = append(f.decided, in)
	return identity.RegistrationDecision{ID: "decision", Decision: in.Decision, Note: in.Note}, nil
}

var registrationsNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func newRegistrationsEnv(t *testing.T, perms ...identity.Permission) (*authEnv, *fakeRegistrations) {
	t.Helper()
	regs := &fakeRegistrations{}
	for i, name := range []string{"dave", "erin", "fern"} {
		since := registrationsNow.Add(time.Duration(i) * time.Minute)
		regs.entries = append(regs.entries, appidentity.RegistrationEntry{
			Application: identity.RegistrationApplication{ID: "app-" + name, UserID: "user-" + name, Reason: "I am " + name, SubmittedAt: since},
			State:       identity.UserRegistrationState{UserID: "user-" + name, State: identity.RegistrationPending, Since: since},
			Email:       name + "@example.com", AccountID: "account-" + name, Username: name,
		})
	}
	regs.entries[1].Application.Reason = ""
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{"user-" + aliceAccountID: perms}
		o.Registrations = regs
	})
	e.auth.tokens["admin-read"] = bearer("admin:read", aliceAccountID)
	e.auth.tokens["admin-write"] = bearer("admin:write", aliceAccountID)
	return e, regs
}

func (e *authEnv) adminBody(t *testing.T, method, path, token, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, clientapi.BasePath+path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept-Language", "en")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return e.spec.Serve(t, e.h, req)
}

func TestListPendingRegistrations(t *testing.T) {
	e, _ := newRegistrationsEnv(t, identity.PermManageAccounts)
	res := e.admin(t, http.MethodGet, "/admin/registrations?limit=2", "admin-read", nil)
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET /admin/registrations = %d (Cache-Control %q)", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	page := decodeJSON[clientapi.AdminRegistrationPage](t, res)
	if len(page.Items) != 2 || page.NextCursor == nil || page.Items[0].UserId != "user-dave" || page.Items[1].UserId != "user-erin" {
		t.Fatalf("first page = %+v", page)
	}
	d := page.Items[0]
	if d.Reason == nil || *d.Reason != "I am dave" || d.Username == nil || *d.Username != "dave" || d.AccountId == nil ||
		*d.AccountId != "account-dave" || d.Email != "dave@example.com" || d.State != clientapi.RegistrationStatePending ||
		!d.Since.Equal(registrationsNow) || page.Items[1].Reason != nil {
		t.Errorf("first page = %+v", page.Items)
	}
	res = e.admin(t, http.MethodGet, "/admin/registrations?limit=2&cursor="+*page.NextCursor, "admin-read", nil)
	if rest := decodeJSON[clientapi.AdminRegistrationPage](t, res); len(rest.Items) != 1 || rest.Items[0].UserId != "user-fern" || rest.NextCursor != nil {
		t.Errorf("second page = %+v", rest)
	}
	// "eA" is base64url for "x": a cursor without a position.
	if res := e.admin(t, http.MethodGet, "/admin/registrations?cursor=eA", "admin-read", nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("GET with a malformed cursor = %d, want 400", res.StatusCode)
	}
}

func TestDecideRegistration(t *testing.T) {
	e, regs := newRegistrationsEnv(t, identity.PermManageAccounts)
	res := e.adminBody(t, http.MethodPost, "/admin/registrations/user-dave/approve", "admin-write", `{"note":"welcome"}`)
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("approve = %d", res.StatusCode)
	}
	if got := decodeJSON[clientapi.AdminRegistration](t, res); got.State != clientapi.RegistrationStateApproved || got.UserId != "user-dave" {
		t.Errorf("approved = %+v", got)
	}
	if res := e.adminBody(t, http.MethodPost, "/admin/registrations/user-erin/reject", "admin-write", ""); res.StatusCode != http.StatusOK ||
		decodeJSON[clientapi.AdminRegistration](t, res).State != clientapi.RegistrationStateRejected {
		t.Errorf("reject without a body = %d", res.StatusCode)
	}
	// A decided registration stays as it is.
	res = e.adminBody(t, http.MethodPost, "/admin/registrations/user-dave/reject", "admin-write", "")
	if res.StatusCode != http.StatusOK || decodeJSON[clientapi.AdminRegistration](t, res).State != clientapi.RegistrationStateApproved {
		t.Errorf("deciding again = %d, want 200 approved", res.StatusCode)
	}
	want := []appidentity.DecideInput{
		{UserID: "user-dave", Decision: identity.DecisionApproved, DecidedByUserID: "user-" + aliceAccountID, Note: "welcome"},
		{UserID: "user-erin", Decision: identity.DecisionRejected, DecidedByUserID: "user-" + aliceAccountID},
	}
	if len(regs.decided) != 2 || regs.decided[0] != want[0] || regs.decided[1] != want[1] {
		t.Errorf("decisions = %+v, want %+v", regs.decided, want)
	}

	if res := e.adminBody(t, http.MethodPost, "/admin/registrations/user-nobody/approve", "admin-write", ""); res.StatusCode != http.StatusNotFound {
		t.Errorf("approving an unknown user = %d, want 404", res.StatusCode)
	}
	long := `{"note":"` + strings.Repeat("a", identity.MaxDecisionNoteLength+1) + `"}`
	res = e.adminBody(t, http.MethodPost, "/admin/registrations/user-fern/approve", "admin-write", long)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusUnprocessableEntity || p.Detail == nil || !strings.Contains(*p.Detail, "note") {
		t.Errorf("a too long note = %d %+v, want 422 on note", res.StatusCode, p)
	}
	if len(regs.decided) != 2 {
		t.Errorf("refused decisions were recorded: %+v", regs.decided)
	}
}

// TestRegistrationReviewNeedsManageAccounts checks that each operation needs the
// manage_accounts permission and its admin scope.
func TestRegistrationReviewNeedsManageAccounts(t *testing.T) {
	e, refused := newRegistrationsEnv(t, identity.PermManageReports)
	for _, op := range []struct{ method, path, token string }{
		{http.MethodGet, "/admin/registrations", "admin-read"},
		{http.MethodPost, "/admin/registrations/user-dave/approve", "admin-write"},
		{http.MethodPost, "/admin/registrations/user-dave/reject", "admin-write"},
	} {
		res := e.adminBody(t, op.method, op.path, op.token, "")
		if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeMissingPermission {
			t.Errorf("%s %s without manage_accounts = %d %+v, want 403 missing-permission", op.method, op.path, res.StatusCode, p)
		}
	}
	e, scoped := newRegistrationsEnv(t, identity.PermManageAccounts)
	if res := e.adminBody(t, http.MethodPost, "/admin/registrations/user-dave/approve", "admin-read", ""); res.StatusCode != http.StatusForbidden {
		t.Errorf("approve with admin:read = %d, want 403", res.StatusCode)
	}
	if len(refused.decided)+len(scoped.decided) != 0 {
		t.Errorf("a refused operation decided: %+v %+v", refused.decided, scoped.decided)
	}
}
