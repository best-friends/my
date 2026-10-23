// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
)

func TestBadgeColors(t *testing.T) {
	colors := identity.AllBadgeColors()
	if len(colors) != 9 {
		t.Fatalf("AllBadgeColors = %v, want the 9 token colors", colors)
	}
	for _, c := range colors {
		if !c.Valid() {
			t.Errorf("%q.Valid() = false", c)
		}
	}
	for _, c := range []identity.BadgeColor{"", "gold", "Red", "#ff0000"} {
		if c.Valid() {
			t.Errorf("%q.Valid() = true", c)
		}
	}
}

func TestNormalizeRoleBadge(t *testing.T) {
	for label, want := range map[string]string{
		"Supporter": "Supporter", "  サポーター ": "サポーター", "a": "a", strings.Repeat("あ", 32): strings.Repeat("あ", 32),
		"داعم": "داعم", "🎉 Fan": "🎉 Fan",
	} {
		got, err := identity.NormalizeRoleBadge(label, identity.BadgeBlue)
		if err != nil || got != want {
			t.Errorf("NormalizeRoleBadge(%q) = %q, %v, want %q", label, got, err, want)
		}
	}
	for _, label := range []string{"", "   ", strings.Repeat("a", 33), "two\nlines", "tab\there", "line sep", "\xff"} {
		if _, err := identity.NormalizeRoleBadge(label, identity.BadgeBlue); !errors.Is(err, identity.ErrInvalidBadgeLabel) {
			t.Errorf("NormalizeRoleBadge(%q) = %v, want ErrInvalidBadgeLabel", label, err)
		}
	}
	if _, err := identity.NormalizeRoleBadge("Supporter", "gold"); !errors.Is(err, identity.ErrInvalidBadgeColor) {
		t.Errorf("NormalizeRoleBadge(gold) = %v, want ErrInvalidBadgeColor", err)
	}
}

func TestSameBadge(t *testing.T) {
	a := &identity.RoleBadge{Label: "Fan", Color: identity.BadgeRed}
	for _, c := range []struct {
		a, b *identity.RoleBadge
		want bool
	}{
		{nil, nil, true},
		{a, nil, false},
		{nil, a, false},
		{a, &identity.RoleBadge{Label: "Fan", Color: identity.BadgeRed}, true},
		{a, &identity.RoleBadge{Label: "Fan", Color: identity.BadgeBlue}, false},
		{a, &identity.RoleBadge{Label: "Fans", Color: identity.BadgeRed}, false},
	} {
		if got := identity.SameBadge(c.a, c.b); got != c.want {
			t.Errorf("SameBadge(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestKeepsBadgeDisplay(t *testing.T) {
	for _, c := range []struct{ displayed, hasBadge, want bool }{
		{true, true, true}, {true, false, false}, {false, true, false}, {false, false, false},
	} {
		if got := identity.KeepsBadgeDisplay(c.displayed, c.hasBadge); got != c.want {
			t.Errorf("KeepsBadgeDisplay(%v, %v) = %v, want %v", c.displayed, c.hasBadge, got, c.want)
		}
	}
}
