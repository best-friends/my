// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"errors"
	"testing"
)

// testMemberPermission is a member permission (ADR-0070 §1).
const testMemberPermission = PermViewFinance

func managedRoleFixtures() []RoleFacts {
	return []RoleFacts{
		{Role: Role{ID: "admin", Key: RoleAdmin, Position: AdminPosition}, Permissions: AllPermissions()},
		{Role: Role{ID: "mod", Key: RoleModerator, Position: 100}, Permissions: []Permission{PermManageReports}},
		{Role: Role{ID: "helper", Key: "helper", Position: 50}, Permissions: []Permission{PermManageCustomEmojis}},
		{Role: Role{ID: "plan", Key: "plan", Position: 10}, Permissions: []Permission{testMemberPermission}, Managed: true},
		{Role: Role{ID: "fan", Key: "fan", Position: 20}, Permissions: []Permission{testMemberPermission}},
		{Role: Role{ID: "badge", Key: "badge", Position: 70}},
	}
}

// A plan links only a role that nobody holds, that is not built in nor linked already, whose
// permissions are member permissions, and that is below every staff role (ADR-0072 §5).
func TestCheckManageable(t *testing.T) {
	roles := managedRoleFixtures()
	byID := map[string]RoleFacts{}
	for _, r := range roles {
		byID[r.Role.ID] = r
	}
	for _, tc := range []struct {
		name    string
		role    RoleFacts
		holders int64
		want    ManageRule
	}{
		{"member role below staff", byID["fan"], 0, ""},
		{"no permissions below staff", RoleFacts{Role: Role{ID: "new", Key: "new", Position: 49}}, 0, ""},
		{"admin", byID["admin"], 0, RuleBuiltin},
		{"moderator", byID["mod"], 0, RuleBuiltin},
		{"managed already", byID["plan"], 0, RuleManaged},
		{"held", byID["fan"], 1, RuleHeld},
		{"staff permission", byID["helper"], 0, RulePermissions},
		{"server permission below staff", RoleFacts{Role: Role{ID: "x", Key: "x", Position: 1}, Permissions: []Permission{PermManageSupporters}}, 0, RulePermissions},
		{"at the lowest staff role", RoleFacts{Role: Role{ID: "x", Key: "x", Position: 50}}, 0, RulePosition},
		{"above a staff role", byID["badge"], 0, RulePosition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckManageable(tc.role, tc.holders, roles)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("CheckManageable = %v, want nil", err)
				}
				return
			}
			var ue *UnmanageableRoleError
			if !errors.As(err, &ue) || ue.Rule != tc.want || !errors.Is(err, ErrRoleNotManageable) {
				t.Fatalf("CheckManageable = %v, want rule %s", err, tc.want)
			}
		})
	}
}

// The plan form keeps a managed role to member permissions below every staff role.
func TestCheckManagedShape(t *testing.T) {
	roles := managedRoleFixtures()
	if err := CheckManagedShape("plan", 49, []Permission{testMemberPermission}, roles); err != nil {
		t.Errorf("below the lowest staff role: %v", err)
	}
	if err := CheckManagedShape("plan", 50, nil, roles); !errors.Is(err, ErrRoleNotManageable) {
		t.Errorf("at the lowest staff role: %v", err)
	}
	if err := CheckManagedShape("plan", 5, []Permission{PermViewAudit}, roles); !errors.Is(err, ErrRoleNotManageable) {
		t.Errorf("a moderation permission: %v", err)
	}
}

// A change to a role that is not managed never puts a staff role at or below a managed one.
func TestCheckUnmanagedChange(t *testing.T) {
	roles := managedRoleFixtures()
	for _, tc := range []struct {
		name     string
		id, key  string
		position int
		perms    []Permission
		ok       bool
	}{
		{"staff role above the managed ones", "helper", "helper", 11, []Permission{PermManageCustomEmojis}, true},
		{"staff role moved onto a managed one", "helper", "helper", 10, []Permission{PermManageCustomEmojis}, false},
		{"moderator moved below a managed one", "mod", RoleModerator, 5, nil, false},
		{"staff permission added below a managed one", "fan", "fan", 5, []Permission{PermManageReports}, false},
		{"member permission below a managed one", "fan", "fan", 5, []Permission{testMemberPermission}, true},
		{"new staff role below a managed one", "", "new", 3, []Permission{PermManageJobs}, false},
		{"new role without permissions", "", "new", 3, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckUnmanagedChange(tc.id, tc.key, tc.position, tc.perms, roles)
			if tc.ok != (err == nil) || (err != nil && !errors.Is(err, ErrBreaksManagedRoles)) {
				t.Fatalf("CheckUnmanagedChange = %v, want ok %v", err, tc.ok)
			}
		})
	}
	// Without a managed role, anything goes.
	if err := CheckUnmanagedChange("mod", RoleModerator, 0, nil, roles[:3]); err != nil {
		t.Errorf("without managed roles: %v", err)
	}
}
