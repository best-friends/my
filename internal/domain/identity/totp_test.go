// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/mycroblog/my/internal/domain/identity"
)

// The SHA-1 test vectors of RFC 6238 Appendix B, cut to six digits.
func TestTOTPCodeMatchesRFC6238(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, tc := range []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		if got := identity.TOTPCode(secret, identity.TOTPStep(time.Unix(tc.unix, 0))); got != tc.want {
			t.Errorf("TOTPCode(T=%d) = %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestMatchTOTPAcceptsTheAdjacentSteps(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111111, 0)
	cur := identity.TOTPStep(now)
	for _, d := range []int64{-1, 0, 1} {
		code := identity.TOTPCode(secret, cur+d)
		if step, ok := identity.MatchTOTP(secret, code[:3]+" "+code[3:], now); !ok || step != cur+d {
			t.Errorf("MatchTOTP(step %+d) = %d, %v; want %d, true", d, step, ok, cur+d)
		}
	}
	for _, code := range []string{identity.TOTPCode(secret, cur-2), identity.TOTPCode(secret, cur+2), "", "12345", "abcdef", "1234567"} {
		if _, ok := identity.MatchTOTP(secret, code, now); ok {
			t.Errorf("MatchTOTP(%q) = true, want false", code)
		}
	}
}

func TestRecoveryCodes(t *testing.T) {
	b := bytes.Repeat([]byte{0, 1, 31, 32}, 4)
	code := identity.RecoveryCodeFromBytes(b)
	if code != "ab7a-ab7a-ab7a-ab7a" {
		t.Errorf("RecoveryCodeFromBytes = %q", code)
	}
	for _, typed := range []string{code, " AB7A-AB7A-AB7A-AB7A ", "ab7aab7aab7aab7a", "ab7a ab7a ab7a ab7a"} {
		if got := identity.NormalizeRecoveryCode(typed); got != "ab7aab7aab7aab7a" {
			t.Errorf("NormalizeRecoveryCode(%q) = %q", typed, got)
		}
	}
	for _, typed := range []string{"", "ab7a-b7ab-7ab7", "ab7a-b7ab-7ab7-ab71", "ab7a-b7ab-7ab7-ab7a-a"} {
		if got := identity.NormalizeRecoveryCode(typed); got != "" {
			t.Errorf("NormalizeRecoveryCode(%q) = %q, want empty", typed, got)
		}
	}
	if !bytes.Equal(identity.HashRecoveryCode("ab7ab7ab7ab7ab7a"), identity.HashRecoveryCode("ab7ab7ab7ab7ab7a")) ||
		bytes.Equal(identity.HashRecoveryCode("ab7ab7ab7ab7ab7a"), identity.HashRecoveryCode("ab7ab7ab7ab7ab7b")) {
		t.Error("HashRecoveryCode is not a function of the code")
	}
}
