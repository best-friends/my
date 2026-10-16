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

func TestValidateEmail(t *testing.T) {
	for _, ok := range []string{"alice@example.com", "Alice.Smith+tag@Example.COM", "ä@例え.jp"} {
		if err := identity.ValidateEmail(ok); err != nil {
			t.Errorf("ValidateEmail(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "alice", "@example.com", "alice@", " alice@example.com", "Alice <alice@example.com>",
		"<alice@example.com>", "a@b, c@d", "alice@example.com (comment)", "\xff@example.com",
		strings.Repeat("a", identity.MaxEmailLength-len("@example.com")+1) + "@example.com",
	} {
		if err := identity.ValidateEmail(bad); !errors.Is(err, identity.ErrInvalidEmail) {
			t.Errorf("ValidateEmail(%q) = %v, want ErrInvalidEmail", bad, err)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := identity.NormalizeEmail("Alice.Smith@Example.COM"); got != "alice.smith@example.com" {
		t.Errorf("NormalizeEmail = %q", got)
	}
}
