// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/domain/identity"
)

func TestValidateTimeZone(t *testing.T) {
	for _, ok := range []string{"UTC", "Asia/Tokyo", "America/Argentina/Buenos_Aires", "Etc/GMT-9"} {
		if err := identity.ValidateTimeZone(ok); err != nil {
			t.Errorf("ValidateTimeZone(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "Local", "Mars/Olympus_Mons", "+09:00", "../etc/passwd", "/etc/localtime",
		strings.Repeat("A", identity.MaxTimeZoneLength+1),
	} {
		if err := identity.ValidateTimeZone(bad); !errors.Is(err, identity.ErrInvalidTimeZone) {
			t.Errorf("ValidateTimeZone(%q) = %v, want ErrInvalidTimeZone", bad, err)
		}
	}
}

func TestValidateVisibility(t *testing.T) {
	for _, ok := range []identity.Visibility{
		identity.VisibilityPublic, identity.VisibilityLocal, identity.VisibilityUnlisted,
		identity.VisibilityPrivate, identity.VisibilityDirect,
	} {
		if err := identity.ValidateVisibility(ok); err != nil {
			t.Errorf("ValidateVisibility(%q) = %v", ok, err)
		}
	}
	for _, bad := range []identity.Visibility{"", "Public", "followers"} {
		if err := identity.ValidateVisibility(bad); !errors.Is(err, identity.ErrInvalidVisibility) {
			t.Errorf("ValidateVisibility(%q) = %v, want ErrInvalidVisibility", bad, err)
		}
	}
}
