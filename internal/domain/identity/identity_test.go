// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/domain/identity"
)

func TestValidatePassword(t *testing.T) {
	for _, ok := range []string{"12345678", "pässwörd", strings.Repeat("あ", identity.MaxPasswordLength)} {
		if err := identity.ValidatePassword(ok); err != nil {
			t.Errorf("ValidatePassword(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1234567", strings.Repeat("a", identity.MaxPasswordLength+1), "abcdefg\xff"} {
		if err := identity.ValidatePassword(bad); !errors.Is(err, identity.ErrInvalidPassword) {
			t.Errorf("ValidatePassword(%q) = %v, want ErrInvalidPassword", bad, err)
		}
	}
}

func TestBuiltinRoles(t *testing.T) {
	roles := identity.BuiltinRoles()
	if len(roles) != 2 || roles[0].Key != identity.RoleAdmin || roles[1].Key != identity.RoleModerator {
		t.Fatalf("BuiltinRoles = %+v, want admin and moderator", roles)
	}
	if len(roles[0].Permissions) != len(identity.AllPermissions()) {
		t.Errorf("admin permissions = %v, want all", roles[0].Permissions)
	}
	if roles[0].Position <= roles[1].Position {
		t.Errorf("admin does not outrank moderator")
	}
}
