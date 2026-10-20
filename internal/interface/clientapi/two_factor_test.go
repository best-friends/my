// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeTwoFactor is the two-factor authentication of alice: code "123456" is right, setup
// "setup-1" is valid, and err, when set, fails every change.
type fakeTwoFactor struct {
	enabled bool
	err     error
	calls   []string
}

var twoFactorEnrolledAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func (f *fakeTwoFactor) TwoFactorStatus(_ context.Context, userID string) (appidentity.TwoFactorStatus, error) {
	f.calls = append(f.calls, "status "+userID)
	if !f.enabled {
		return appidentity.TwoFactorStatus{}, nil
	}
	return appidentity.TwoFactorStatus{Enabled: true, EnrolledAt: twoFactorEnrolledAt, RecoveryCodesRemaining: 7}, nil
}

func (f *fakeTwoFactor) BeginTOTPSetup(_ context.Context, userID string) (appidentity.TOTPSetup, error) {
	f.calls = append(f.calls, "setup "+userID)
	if f.enabled {
		return appidentity.TOTPSetup{}, appidentity.ErrTwoFactorEnabled
	}
	return appidentity.TOTPSetup{ //nolint:gosec // G101: a made-up secret of the test
		Secret: "JBSWY3DPEHPK3PXP", URI: "otpauth://totp/social.example:alice@social.example?secret=JBSWY3DPEHPK3PXP",
		Token: "setup-1", ExpiresAt: twoFactorEnrolledAt,
	}, nil
}

func (f *fakeTwoFactor) check(code string) error {
	switch {
	case f.err != nil:
		return f.err
	case code != "123456":
		return appidentity.ErrInvalidSecondFactor
	}
	return nil
}

func (f *fakeTwoFactor) EnableTOTP(_ context.Context, userID, setupToken, code string) ([]string, error) {
	f.calls = append(f.calls, "enable "+userID)
	if setupToken != "setup-1" {
		return nil, appidentity.ErrInvalidTOTPSetup
	}
	if err := f.check(code); err != nil {
		return nil, err
	}
	f.enabled = true
	return []string{"aaaa-bbbb-cccc-dddd"}, nil
}

func (f *fakeTwoFactor) DisableTOTP(_ context.Context, userID, code string) error {
	f.calls = append(f.calls, "disable "+userID)
	if !f.enabled {
		return appidentity.ErrTwoFactorNotEnabled
	}
	if err := f.check(code); err != nil {
		return err
	}
	f.enabled = false
	return nil
}

func (f *fakeTwoFactor) RegenerateRecoveryCodes(_ context.Context, userID, code string) ([]string, error) {
	f.calls = append(f.calls, "regenerate "+userID)
	if err := f.check(code); err != nil {
		return nil, err
	}
	return []string{"eeee-ffff-gggg-hhhh"}, nil
}

func newTwoFactorEnv(t *testing.T) (*authEnv, *fakeTwoFactor) {
	t.Helper()
	f := &fakeTwoFactor{}
	return newAuthEnv(t, func(o *clientapi.Options) { o.TwoFactor = f }), f
}

func TestTwoFactorEnrollment(t *testing.T) {
	e, f := newTwoFactorEnv(t)
	res := e.send(t, http.MethodGet, "/two_factor", "read-accounts", "")
	if st := decodeJSON[clientapi.TwoFactor](t, res); res.StatusCode != http.StatusOK || st.Enabled || st.EnrolledAt != nil ||
		st.RecoveryCodesRemaining != nil || res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("status = %d %+v, want disabled", res.StatusCode, st)
	}
	res = e.send(t, http.MethodPost, "/two_factor/totp/setup", "write", "")
	setup := decodeJSON[clientapi.TotpSetup](t, res)
	if res.StatusCode != http.StatusOK || setup.Secret != "JBSWY3DPEHPK3PXP" || setup.SetupToken != "setup-1" ||
		!strings.HasPrefix(setup.Uri, "otpauth://totp/") || !setup.ExpiresAt.Equal(twoFactorEnrolledAt) ||
		res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("setup = %d %+v", res.StatusCode, setup)
	}
	// The QR code has its quiet zone and starts with the top row of the top-left finder pattern.
	if q := setup.QrCode; q.Size < 21+8 || (q.Size-8-17)%4 != 0 || !strings.HasPrefix(q.Path, "M4 4h7v1h-7z") {
		t.Errorf("QR code = %d %.40q", q.Size, q.Path)
	}

	for _, tc := range []struct {
		body, typ string
		status    int
	}{
		{`{"setup_token":"setup-1","code":"000000"}`, clientapi.ProblemTypeInvalidTwoFactorCode, http.StatusUnprocessableEntity},
		{`{"setup_token":"old","code":"123456"}`, clientapi.ProblemTypeTOTPSetupExpired, http.StatusUnprocessableEntity},
	} {
		res := e.send(t, http.MethodPost, "/two_factor/totp", "write", tc.body)
		if p := decodeProblem(t, res); res.StatusCode != tc.status || p.Type != tc.typ {
			t.Errorf("enable %s = %d %+v, want %d %s", tc.body, res.StatusCode, p, tc.status, tc.typ)
		}
	}
	res = e.send(t, http.MethodPost, "/two_factor/totp", "write", `{"setup_token":"setup-1","code":"123456"}`)
	if codes := decodeJSON[clientapi.RecoveryCodes](t, res); res.StatusCode != http.StatusOK || !slices.Equal(codes.RecoveryCodes, []string{"aaaa-bbbb-cccc-dddd"}) {
		t.Errorf("enable = %d %+v", res.StatusCode, codes)
	}
	res = e.send(t, http.MethodGet, "/two_factor", "read-accounts", "")
	if st := decodeJSON[clientapi.TwoFactor](t, res); !st.Enabled || st.EnrolledAt == nil || !st.EnrolledAt.Equal(twoFactorEnrolledAt) ||
		st.RecoveryCodesRemaining == nil || *st.RecoveryCodesRemaining != 7 {
		t.Errorf("status after enabling = %+v", st)
	}
	res = e.send(t, http.MethodPost, "/two_factor/totp/setup", "write", "")
	if p := decodeProblem(t, res); res.StatusCode != http.StatusConflict || p.Type != clientapi.ProblemTypeTwoFactorEnabled {
		t.Errorf("setup when enabled = %d %+v, want 409", res.StatusCode, p)
	}
	if want := "user-" + aliceAccountID; !slices.ContainsFunc(f.calls, func(c string) bool { return c == "enable "+want }) {
		t.Errorf("calls = %v, want alice's", f.calls)
	}
}

func TestTwoFactorChanges(t *testing.T) {
	e, f := newTwoFactorEnv(t)
	res := e.send(t, http.MethodPost, "/two_factor/totp/disable", "write", `{"code":"123456"}`)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusConflict || p.Type != clientapi.ProblemTypeTwoFactorNotEnabled {
		t.Errorf("disable when disabled = %d %+v, want 409", res.StatusCode, p)
	}
	f.enabled = true
	res = e.send(t, http.MethodPost, "/two_factor/recovery_codes", "write", `{"code":"123456"}`)
	if codes := decodeJSON[clientapi.RecoveryCodes](t, res); res.StatusCode != http.StatusOK || !slices.Equal(codes.RecoveryCodes, []string{"eeee-ffff-gggg-hhhh"}) {
		t.Errorf("regenerate = %d %+v", res.StatusCode, codes)
	}
	res = e.send(t, http.MethodPost, "/two_factor/totp/disable", "write", `{"code":"654321"}`)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusUnprocessableEntity || p.Type != clientapi.ProblemTypeInvalidTwoFactorCode {
		t.Errorf("disable with a wrong code = %d %+v, want 422", res.StatusCode, p)
	}
	f.err = &kernel.RateLimitedError{RetryAfter: 90 * time.Second}
	res = e.send(t, http.MethodPost, "/two_factor/recovery_codes", "write", `{"code":"123456"}`)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") != "90" {
		t.Errorf("rate limited = %d %+v (Retry-After %q), want 429 after 90 s", res.StatusCode, p, res.Header.Get("Retry-After"))
	}
	f.err = nil
	res = e.send(t, http.MethodPost, "/two_factor/totp/disable", "write", `{"code":"123456"}`)
	if st := decodeJSON[clientapi.TwoFactor](t, res); res.StatusCode != http.StatusOK || st.Enabled {
		t.Errorf("disable = %d %+v", res.StatusCode, st)
	}
	res = e.send(t, http.MethodPost, "/two_factor/totp/disable", "read-accounts", `{"code":"123456"}`)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeInsufficientScope {
		t.Errorf("disable with read:accounts = %d %+v, want 403", res.StatusCode, p)
	}
}
