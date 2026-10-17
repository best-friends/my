// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/domain/identity"
)

func TestNormalizeInviteCode(t *testing.T) {
	for in, want := range map[string]string{
		"ABCD2345EFGH6723":      "ABCD2345EFGH6723",
		"abcd-2345 efgh\t6723":  "ABCD2345EFGH6723",
		" abcd-2345-efgh-6723 ": "ABCD2345EFGH6723",
		"":                      "",
		"- -":                   "",
	} {
		if got := identity.NormalizeInviteCode(in); got != want {
			t.Errorf("NormalizeInviteCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInviteLimits(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	at := now.Add(time.Hour)
	inv := identity.Invite{MaxUses: 2, ExpiresAt: &at}
	if inv.Expired(now) || !inv.Expired(at) || inv.Exhausted(1) || !inv.Exhausted(2) {
		t.Errorf("limits of %+v are wrong", inv)
	}
	unlimited := identity.Invite{}
	if unlimited.Expired(now.Add(1000*time.Hour)) || unlimited.Exhausted(1_000_000) {
		t.Error("an invite without limits expired or ran out")
	}
}

func TestValidateRegistrationReason(t *testing.T) {
	for _, ok := range []string{"", "I like ducks.", strings.Repeat("あ", identity.MaxRegistrationReasonLength)} {
		if err := identity.ValidateRegistrationReason(ok); err != nil {
			t.Errorf("ValidateRegistrationReason(%d chars) = %v", len([]rune(ok)), err)
		}
	}
	for _, bad := range []string{strings.Repeat("a", identity.MaxRegistrationReasonLength+1), "\xff", "a\x00b"} {
		if err := identity.ValidateRegistrationReason(bad); !errors.Is(err, identity.ErrInvalidReason) {
			t.Errorf("ValidateRegistrationReason(%q...) = %v, want ErrInvalidReason", bad[:1], err)
		}
	}
}

func TestValidateDecisionNote(t *testing.T) {
	for _, ok := range []string{"", "welcome", strings.Repeat("あ", identity.MaxDecisionNoteLength)} {
		if err := identity.ValidateDecisionNote(ok); err != nil {
			t.Errorf("ValidateDecisionNote(%d chars) = %v", len([]rune(ok)), err)
		}
	}
	for _, bad := range []string{strings.Repeat("a", identity.MaxDecisionNoteLength+1), "\xff", "a\x00b"} {
		if err := identity.ValidateDecisionNote(bad); !errors.Is(err, identity.ErrInvalidNote) {
			t.Errorf("ValidateDecisionNote(%q...) = %v, want ErrInvalidNote", bad[:1], err)
		}
	}
}

func TestDecisionState(t *testing.T) {
	if identity.DecisionApproved.State() != identity.RegistrationApproved || identity.DecisionRejected.State() != identity.RegistrationRejected {
		t.Error("Decision.State maps wrongly")
	}
}
