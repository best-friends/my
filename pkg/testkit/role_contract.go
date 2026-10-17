// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/moderation"
)

// RoleRepositoryContract verifies the role persistence of a database adapter's identity
// repository (data-model §4.1, FR-720) through the role use cases: the roles list with their
// permissions; a grant and a revocation change the user's permissions and the holders of a
// permission at once, record their audit log entry in their transaction, and do nothing when
// they would change nothing; a user changes only the roles at or below their own; the admin role
// keeps its last holder, also when two revocations race; and unknown users
// and roles are reported. It uses the same harness as AccountRepositoryContract.
func RoleRepositoryContract(t *testing.T, newHarness AccountFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *roleEnv)
	}{
		{"ListsTheRoles", roleLists},
		{"GrantsAndRevokes", roleGrantsAndRevokes},
		{"RefusesRolesAboveTheActor", roleRefusesOutranking},
		{"KeepsTheLastAdmin", roleKeepsLastAdmin},
		{"KeepsTheLastAdminUnderConcurrency", roleLastAdminConcurrency},
		{"ReportsUnknownUsersAndRoles", roleReportsUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &auditRecorder{}
			e := &roleEnv{audit: rec}
			e.accountEnv = newAccountEnv(t, newHarness, func(p *uow.Projectors) {
				p.Register(appmoderation.EventAuditRecorded, func(_ context.Context, _ uow.Tx, ev uow.Event) error {
					rec.add(ev.(appmoderation.AuditRecorded).Entry)
					return nil
				})
			})
			admin, err := e.identity.CreateAdmin(t.Context(), appidentity.CreateAdminInput{
				Username: "root", Email: "root@example.com", Password: "correct horse battery staple",
			})
			if err != nil {
				t.Fatal(err)
			}
			e.admin = admin.User.ID
			tc.run(t, e)
		})
	}
}

type auditRecorder struct {
	mu      sync.Mutex
	entries []moderation.AuditEntry
}

func (r *auditRecorder) add(e moderation.AuditEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

func (r *auditRecorder) all() []moderation.AuditEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.entries)
}

type roleEnv struct {
	*accountEnv
	audit *auditRecorder
	// admin is the user of @root, the only admin at first.
	admin string
}

func (e *roleEnv) grant(t *testing.T, userID, key, by string) bool {
	t.Helper()
	changed, err := e.identity.GrantRole(t.Context(), appidentity.RoleChangeInput{UserID: userID, RoleKey: key, ByUserID: by})
	if err != nil {
		t.Fatalf("GrantRole(%s, %s, by %q) = %v", userID, key, by, err)
	}
	return changed
}

func (e *roleEnv) revoke(t *testing.T, userID, key, by string) bool {
	t.Helper()
	changed, err := e.identity.RevokeRole(t.Context(), appidentity.RoleChangeInput{UserID: userID, RoleKey: key, ByUserID: by})
	if err != nil {
		t.Fatalf("RevokeRole(%s, %s, by %q) = %v", userID, key, by, err)
	}
	return changed
}

func (e *roleEnv) permissions(t *testing.T, userID string) []identity.Permission {
	t.Helper()
	perms, err := e.identity.Permissions(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return perms
}

func (e *roleEnv) holders(t *testing.T, p identity.Permission) []string {
	t.Helper()
	var out []string
	err := e.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		hs, err := e.identity.PermissionHolders(t.Context(), tx, p)
		out = out[:0]
		for _, h := range hs {
			out = append(out, h.UserID)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func roleLists(t *testing.T, e *roleEnv) {
	roles, err := e.identity.Roles(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("Roles = %+v, want admin and moderator", roles)
	}
	for i, b := range identity.BuiltinRoles() {
		if r := roles[i]; r.ID == "" || r.Key != b.Key || r.Position != b.Position || !reflect.DeepEqual(r.Permissions, b.Permissions) {
			t.Errorf("role %d = %+v, want %+v", i, r, b)
		}
	}
	holders, err := e.identity.RoleHolders(t.Context(), identity.RoleAdmin)
	if err != nil || !reflect.DeepEqual(holders, []string{e.admin}) {
		t.Errorf("RoleHolders(admin) = %q, %v; want root", holders, err)
	}
	if holders, err := e.identity.RoleHolders(t.Context(), identity.RoleModerator); err != nil || len(holders) != 0 {
		t.Errorf("RoleHolders(moderator) = %q, %v; want none", holders, err)
	}
}

func roleGrantsAndRevokes(t *testing.T, e *roleEnv) {
	bob := e.register(t, "bob").User.ID
	if got := e.permissions(t, bob); len(got) != 0 {
		t.Fatalf("bob's permissions = %v, want none", got)
	}
	if !e.grant(t, bob, identity.RoleModerator, e.admin) {
		t.Fatal("granting moderator changed nothing")
	}
	grantedAt := e.clock.Now()
	moderatorPerms := []identity.Permission{identity.PermManageReports, identity.PermManageAccounts, identity.PermViewAudit}
	if got := e.permissions(t, bob); !reflect.DeepEqual(got, moderatorPerms) {
		t.Errorf("bob's permissions = %v, want %v", got, moderatorPerms)
	}
	if got, want := e.holders(t, identity.PermManageReports), sorted(e.admin, bob); !reflect.DeepEqual(got, want) {
		t.Errorf("holders of manage_reports = %q, want %q", got, want)
	}
	roles, err := e.identity.UserRoles(t.Context(), bob)
	if err != nil || len(roles) != 1 || roles[0].Key != identity.RoleModerator {
		t.Fatalf("UserRoles(bob) = %+v, %v; want moderator", roles, err)
	}
	moderatorID := roles[0].ID
	e.clock.Advance(time.Second)
	if e.grant(t, bob, identity.RoleModerator, "") {
		t.Error("granting a held role changed something")
	}

	e.clock.Advance(time.Second)
	if !e.revoke(t, bob, identity.RoleModerator, "") {
		t.Fatal("revoking moderator changed nothing")
	}
	revokedAt := e.clock.Now()
	if got := e.permissions(t, bob); len(got) != 0 {
		t.Errorf("bob's permissions after the revocation = %v, want none", got)
	}
	if got := e.holders(t, identity.PermManageReports); !reflect.DeepEqual(got, []string{e.admin}) {
		t.Errorf("holders of manage_reports after the revocation = %q, want root", got)
	}
	if e.revoke(t, bob, identity.RoleModerator, e.admin) {
		t.Error("revoking a role not held changed something")
	}
	// A new grant after a revocation is a grant of its own.
	e.clock.Advance(time.Second)
	if !e.grant(t, bob, identity.RoleModerator, "") {
		t.Fatal("granting moderator again changed nothing")
	}

	got := e.audit.all()
	if len(got) != 3 {
		t.Fatalf("audit entries = %+v, want 3", got)
	}
	want := []moderation.AuditEntry{
		{ActorKind: moderation.AuditActorUser, ActorUserID: e.admin, Action: moderation.AuditRoleGranted, OccurredAt: grantedAt},
		{ActorKind: moderation.AuditActorCLI, Action: moderation.AuditRoleRevoked, OccurredAt: revokedAt},
		{ActorKind: moderation.AuditActorCLI, Action: moderation.AuditRoleGranted, OccurredAt: e.clock.Now()},
	}
	for i, w := range want {
		g := got[i]
		if g.ID == "" || g.ActorKind != w.ActorKind || g.ActorUserID != w.ActorUserID || g.Action != w.Action ||
			g.TargetKind != moderation.AuditTargetRole || g.TargetID != moderatorID || g.TargetLabel != "moderator @bob" ||
			g.SourceEventID == "" || !g.OccurredAt.Equal(w.OccurredAt) {
			t.Errorf("audit entry %d = %+v, want %+v on the moderator role of @bob", i, g, w)
		}
		if err := g.Validate(); err != nil {
			t.Errorf("audit entry %d: %v", i, err)
		}
	}
	if got[0].SourceEventID != got[1].SourceEventID || got[2].SourceEventID == got[0].SourceEventID {
		t.Errorf("sources = %s, %s, %s; want the revocation to name the first grant and the new grant its own",
			got[0].SourceEventID, got[1].SourceEventID, got[2].SourceEventID)
	}
}

func sorted(ids ...string) []string {
	slices.Sort(ids)
	return ids
}

func roleRefusesOutranking(t *testing.T, e *roleEnv) {
	bob := e.register(t, "bob").User.ID
	carol := e.register(t, "carol").User.ID
	dave := e.register(t, "dave").User.ID
	e.grant(t, bob, identity.RoleModerator, "")
	for _, tc := range []struct {
		name      string
		change    func(context.Context, appidentity.RoleChangeInput) (bool, error)
		user, key string
		by        string
	}{
		{"a moderator grants admin", e.identity.GrantRole, carol, identity.RoleAdmin, bob},
		{"a moderator revokes admin", e.identity.RevokeRole, e.admin, identity.RoleAdmin, bob},
		{"a user without roles grants moderator", e.identity.GrantRole, carol, identity.RoleModerator, dave},
	} {
		if _, err := tc.change(t.Context(), appidentity.RoleChangeInput{UserID: tc.user, RoleKey: tc.key, ByUserID: tc.by}); !errors.Is(err, appidentity.ErrRoleOutranksActor) {
			t.Errorf("%s = %v, want ErrRoleOutranksActor", tc.name, err)
		}
	}
	// A moderator manages the roles at its own position.
	if !e.grant(t, carol, identity.RoleModerator, bob) || !e.revoke(t, carol, identity.RoleModerator, bob) {
		t.Error("a moderator could not grant and revoke moderator")
	}
	if got := e.permissions(t, carol); len(got) != 0 {
		t.Errorf("carol's permissions = %v, want none", got)
	}
	if n := len(e.audit.all()); n != 3 { // bob's grant, then carol's grant and revocation
		t.Errorf("audit entries = %d, want 3; refused changes record nothing", n)
	}
}

func roleKeepsLastAdmin(t *testing.T, e *roleEnv) {
	bob := e.register(t, "bob").User.ID
	if _, err := e.identity.RevokeRole(t.Context(), appidentity.RoleChangeInput{UserID: e.admin, RoleKey: identity.RoleAdmin, ByUserID: e.admin}); !errors.Is(err, appidentity.ErrLastAdmin) {
		t.Fatalf("revoking the only admin = %v, want ErrLastAdmin", err)
	}
	if got := e.permissions(t, e.admin); !reflect.DeepEqual(got, identity.AllPermissions()) {
		t.Errorf("root's permissions = %v, want all", got)
	}
	e.grant(t, bob, identity.RoleAdmin, e.admin)
	if !e.revoke(t, e.admin, identity.RoleAdmin, e.admin) {
		t.Fatal("revoking one of two admins changed nothing")
	}
	if _, err := e.identity.RevokeRole(t.Context(), appidentity.RoleChangeInput{UserID: bob, RoleKey: identity.RoleAdmin}); !errors.Is(err, appidentity.ErrLastAdmin) {
		t.Errorf("revoking the new only admin from the CLI = %v, want ErrLastAdmin", err)
	}
	if n := len(e.audit.all()); n != 2 {
		t.Errorf("audit entries = %d, want 2", n)
	}
}

func roleLastAdminConcurrency(t *testing.T, e *roleEnv) {
	bob := e.register(t, "bob").User.ID
	e.grant(t, bob, identity.RoleAdmin, "")
	// Two operators revoke the admin role of each admin at the same time: one admin must stay.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, user := range []string{e.admin, bob} {
		wg.Go(func() {
			_, errs[i] = e.identity.RevokeRole(t.Context(), appidentity.RoleChangeInput{UserID: user, RoleKey: identity.RoleAdmin})
		})
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, appidentity.ErrLastAdmin):
			t.Errorf("RevokeRole = %v, want nil or ErrLastAdmin", err)
		}
	}
	holders, err := e.identity.RoleHolders(t.Context(), identity.RoleAdmin)
	if ok != 1 || err != nil || len(holders) != 1 {
		t.Errorf("%d revocations succeeded, admins %q (%v); want 1 and one admin left", ok, holders, err)
	}
}

func roleReportsUnknown(t *testing.T, e *roleEnv) {
	bob := e.register(t, "bob").User.ID
	for _, in := range []appidentity.RoleChangeInput{
		{UserID: bob, RoleKey: "owner"},
		{UserID: e.ids.NewID(), RoleKey: identity.RoleModerator},
	} {
		if _, err := e.identity.GrantRole(t.Context(), in); !errors.Is(err, appidentity.ErrNotFound) {
			t.Errorf("GrantRole(%+v) = %v, want ErrNotFound", in, err)
		}
		if _, err := e.identity.RevokeRole(t.Context(), in); !errors.Is(err, appidentity.ErrNotFound) {
			t.Errorf("RevokeRole(%+v) = %v, want ErrNotFound", in, err)
		}
	}
	if _, err := e.identity.RoleHolders(t.Context(), "owner"); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("RoleHolders(owner) = %v, want ErrNotFound", err)
	}
}
