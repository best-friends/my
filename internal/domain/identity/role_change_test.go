// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
)

func TestCanManageRole(t *testing.T) {
	admin := identity.Role{Key: identity.RoleAdmin, Position: 1000}
	moderator := identity.Role{Key: identity.RoleModerator, Position: 100}
	helper := identity.Role{Key: "helper", Position: 100}
	for _, tc := range []struct {
		name  string
		actor []identity.Role
		role  identity.Role
		want  bool
	}{
		{"admin grants admin", []identity.Role{admin}, admin, true},
		{"admin grants moderator", []identity.Role{admin}, moderator, true},
		{"moderator grants an equal role", []identity.Role{moderator}, helper, true},
		{"moderator grants admin", []identity.Role{moderator}, admin, false},
		{"the highest role counts", []identity.Role{moderator, admin}, admin, true},
		{"no roles", nil, moderator, false},
	} {
		if got := identity.CanManageRole(tc.actor, tc.role); got != tc.want {
			t.Errorf("%s: CanManageRole = %v, want %v", tc.name, got, tc.want)
		}
	}
}
