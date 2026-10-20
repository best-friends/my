// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeInvites keeps invites newest first with the semantics of appidentity.Service.
type fakeInvites struct {
	list    []appidentity.InviteStatus
	creates []appidentity.CreateInviteInput
}

func (f *fakeInvites) ListInvites(_ context.Context, before string, limit int) ([]appidentity.InviteStatus, error) {
	var out []appidentity.InviteStatus
	for _, st := range f.list {
		if (before == "" || st.ID < before) && len(out) < limit {
			out = append(out, st)
		}
	}
	return out, nil
}

func (f *fakeInvites) CreateInvite(_ context.Context, in appidentity.CreateInviteInput) (identity.Invite, error) {
	f.creates = append(f.creates, in)
	inv := identity.Invite{
		ID: fmt.Sprintf("01890a5d-ac96-774b-bcce-b302099a8%03d", len(f.creates)), Code: "ABCDEFGHIJKLMNOP",
		CreatedByUserID: in.CreatedByUserID, MaxUses: in.MaxUses, CreatedAt: adminNow,
	}
	if in.ExpiresIn > 0 {
		at := adminNow.Add(in.ExpiresIn)
		inv.ExpiresAt = &at
	}
	f.list = append([]appidentity.InviteStatus{{Invite: inv, State: appidentity.InviteUsable}}, f.list...)
	return inv, nil
}

func (f *fakeInvites) RevokeInviteByID(_ context.Context, id, _ string) (appidentity.InviteStatus, error) {
	i := slices.IndexFunc(f.list, func(st appidentity.InviteStatus) bool { return st.ID == id })
	if i < 0 {
		return appidentity.InviteStatus{}, appidentity.ErrNotFound
	}
	f.list[i].Revoked, f.list[i].State = true, appidentity.InviteRevoked
	return f.list[i], nil
}

// newInvitesEnv is newAuthEnv with the tokens of newAdminEnv and the invites: alice holds every
// permission and bob none.
func newInvitesEnv(t *testing.T) (*authEnv, *fakeInvites) {
	t.Helper()
	f := &fakeInvites{}
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{"user-" + aliceAccountID: identity.AllPermissions()}
		o.Invites, o.SignUpPath = f, "/auth/sign_up"
	})
	e.auth.tokens["admin"] = bearer("admin", aliceAccountID)
	e.auth.tokens["admin-read"] = bearer("admin:read", aliceAccountID)
	e.auth.tokens["bob-admin"] = bearer("admin", bobAccountID)
	return e, f
}

func TestInviteOperationsNeedAPermission(t *testing.T) {
	e, f := newInvitesEnv(t)
	for _, op := range []struct{ method, path, body string }{
		{http.MethodGet, "/admin/invites", ""},
		{http.MethodPost, "/admin/invites", `{}`},
		{http.MethodPost, "/admin/invites/01890a5d-ac96-774b-bcce-b302099a8001/revoke", ""},
	} {
		res := e.send(t, op.method, op.path, "bob-admin", op.body)
		if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeMissingPermission ||
			p.Detail == nil || !strings.Contains(*p.Detail, "manage_invites") {
			t.Errorf("%s %s by bob = %d %+v, want 403 for manage_invites", op.method, op.path, res.StatusCode, p)
		}
	}
	if len(f.creates) != 0 {
		t.Error("a refused operation created an invite")
	}
}

func TestManageInvites(t *testing.T) {
	e, f := newInvitesEnv(t)
	res := e.sendJSON(t, http.MethodPost, "/admin/invites", "admin", `{"max_uses":5,"expires_in":86400}`)
	got := decodeJSON[clientapi.AdminInvite](t, res)
	if res.StatusCode != http.StatusOK || got.Code != "ABCDEFGHIJKLMNOP" || got.Url != "https://social.example/auth/sign_up?invite=ABCDEFGHIJKLMNOP" ||
		got.MaxUses == nil || *got.MaxUses != 5 || got.Uses != 0 || got.State != clientapi.AdminInviteStateUsable ||
		got.ExpiresAt == nil || !got.ExpiresAt.Equal(adminNow.Add(24*time.Hour)) {
		t.Fatalf("create = %d %+v", res.StatusCode, got)
	}
	want := appidentity.CreateInviteInput{CreatedByUserID: "user-" + aliceAccountID, MaxUses: 5, ExpiresIn: 24 * time.Hour}
	if len(f.creates) != 1 || f.creates[0] != want {
		t.Errorf("creates = %+v, want %+v", f.creates, want)
	}
	// Without limits, the invite works any number of times and forever.
	unlimited := decodeJSON[clientapi.AdminInvite](t, e.sendJSON(t, http.MethodPost, "/admin/invites", "admin", `{}`))
	if unlimited.MaxUses != nil || unlimited.ExpiresAt != nil || f.creates[1].MaxUses != 0 || f.creates[1].ExpiresIn != 0 {
		t.Errorf("unlimited = %+v, input %+v; want no limits", unlimited, f.creates[1])
	}

	// One invite a page: the newest first, then the next page from the cursor.
	res = e.admin(t, http.MethodGet, "/admin/invites?limit=1", "admin-read", nil)
	page := decodeJSON[clientapi.AdminInvitePage](t, res)
	if res.StatusCode != http.StatusOK || len(page.Items) != 1 || page.Items[0].Id != unlimited.Id || page.NextCursor == nil {
		t.Fatalf("first page = %d %+v", res.StatusCode, page)
	}
	page = decodeJSON[clientapi.AdminInvitePage](t, e.admin(t, http.MethodGet, "/admin/invites?limit=1&cursor="+*page.NextCursor, "admin-read", nil))
	if len(page.Items) != 1 || page.Items[0].Id != got.Id || page.NextCursor != nil {
		t.Errorf("second page = %+v, want the first invite and no cursor", page)
	}

	res = e.send(t, http.MethodPost, "/admin/invites/"+got.Id+"/revoke", "admin", "")
	if revoked := decodeJSON[clientapi.AdminInvite](t, res); res.StatusCode != http.StatusOK || revoked.State != clientapi.AdminInviteStateRevoked {
		t.Errorf("revoke = %d %+v, want revoked", res.StatusCode, revoked)
	}
	res = e.send(t, http.MethodPost, "/admin/invites/01890a5d-ac96-774b-bcce-b302099a8999/revoke", "admin", "")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("revoke unknown = %d, want 404", res.StatusCode)
	}
}
