// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/identity"
)

// The session cases of AccountRepositoryContract.

const (
	sessionEmail    = "alice@example.com"
	sessionPassword = "correct horse battery staple"
)

// sessionUser creates a user who signs in with sessionEmail and sessionPassword.
func (e *accountEnv) sessionUser(t *testing.T) appidentity.Registration {
	t.Helper()
	reg, err := e.identity.CreateAdmin(t.Context(), appidentity.CreateAdminInput{
		Username: "alice", Email: "Alice@Example.com", Password: sessionPassword,
	})
	if err != nil {
		t.Fatalf("CreateAdmin = %v", err)
	}
	return reg
}

func (e *accountEnv) signIn(t *testing.T, in appidentity.SignInInput) appidentity.SignedIn {
	t.Helper()
	if in.Email == "" {
		in.Email, in.Password = sessionEmail, sessionPassword
	}
	got, err := e.identity.SignIn(t.Context(), in)
	if err != nil {
		t.Fatalf("SignIn(%q) = %v", in.Email, err)
	}
	return got
}

func (e *accountEnv) wantInvalidSession(t *testing.T, token, why string) {
	t.Helper()
	if s, err := e.identity.Authenticate(t.Context(), token); !errors.Is(err, appidentity.ErrInvalidSession) {
		t.Fatalf("Authenticate(%s) = %+v, %v; want ErrInvalidSession", why, s, err)
	}
}

func sessionSignsInAndOut(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.sessionUser(t)
	e.clock.Advance(time.Second)
	in := appidentity.SignInInput{
		Email: "ALICE@example.com", Password: sessionPassword, IP: "192.0.2.1",
		UserAgent: strings.Repeat("é", identity.MaxUserAgentLength), // cut to whole characters
	}
	got := e.signIn(t, in)
	if !strings.HasPrefix(got.Token, appidentity.SessionTokenPrefix) || len(got.Token) < len(appidentity.SessionTokenPrefix)+43 {
		t.Errorf("token = %q, want %s and 256 bits of base64url", got.Token, appidentity.SessionTokenPrefix)
	}
	if !bytes.Equal(got.Session.TokenHash, appidentity.HashToken(got.Token)) {
		t.Error("the session does not store the token's hash")
	}
	now := e.clock.Now()
	s, err := e.identity.Authenticate(ctx, got.Token)
	if err != nil {
		t.Fatalf("Authenticate = %v", err)
	}
	if s.ID != got.Session.ID || s.UserID != reg.User.ID || !s.CreatedAt.Equal(now) ||
		!s.ExpiresAt.Equal(now.Add(appidentity.DefaultSessionTTL)) {
		t.Errorf("Authenticate = %+v, want the new session of %s created at %v", s, reg.User.ID, now)
	}
	o, err := e.h.Identity.FindSessionOrigin(ctx, s.ID)
	if err != nil || o.SessionID != s.ID || o.UserID != reg.User.ID || o.IP != "192.0.2.1" || !o.CreatedAt.Equal(now) {
		t.Errorf("FindSessionOrigin = %+v, %v; want 192.0.2.1 for the session created at %v", o, err, now)
	}
	if len(o.UserAgent) > identity.MaxUserAgentLength || o.UserAgent != strings.Repeat("é", identity.MaxUserAgentLength/2) {
		t.Errorf("stored user agent has %d bytes, want it cut to %d bytes of whole characters", len(o.UserAgent), identity.MaxUserAgentLength)
	}

	// Activity is recorded at sign-in and refreshed only after the interval.
	_, act, err := e.h.Identity.FindUnrevokedSession(ctx, got.Session.TokenHash)
	if err != nil || !act.LastSeenAt.Equal(now) {
		t.Fatalf("activity after sign-in = %+v, %v; want %v", act, err, now)
	}
	e.clock.Advance(appidentity.SessionActivityInterval - time.Millisecond)
	if _, err := e.identity.Authenticate(ctx, got.Token); err != nil {
		t.Fatal(err)
	}
	if _, act, _ := e.h.Identity.FindUnrevokedSession(ctx, got.Session.TokenHash); !act.LastSeenAt.Equal(now) {
		t.Errorf("activity within the interval = %v, want it unchanged at %v", act.LastSeenAt, now)
	}
	later := e.clock.Advance(time.Millisecond)
	if _, err := e.identity.Authenticate(ctx, got.Token); err != nil {
		t.Fatal(err)
	}
	if _, act, _ := e.h.Identity.FindUnrevokedSession(ctx, got.Session.TokenHash); !act.LastSeenAt.Equal(later) {
		t.Errorf("activity after the interval = %v, want %v", act.LastSeenAt, later)
	}

	e.wantInvalidSession(t, "", "empty token")
	e.wantInvalidSession(t, appidentity.SessionTokenPrefix+"unknown", "unknown token")
	for range 2 { // signing out twice is harmless
		if err := e.identity.SignOut(ctx, got.Token); err != nil {
			t.Fatalf("SignOut = %v", err)
		}
	}
	e.wantInvalidSession(t, got.Token, "signed-out token")
	if err := e.identity.SignOut(ctx, "unknown"); err != nil {
		t.Errorf("SignOut(unknown) = %v, want nil", err)
	}
}

func sessionReplacedOnSignIn(t *testing.T, e *accountEnv) {
	e.sessionUser(t)
	first := e.signIn(t, appidentity.SignInInput{})
	second := e.signIn(t, appidentity.SignInInput{CurrentToken: first.Token})
	if second.Token == first.Token || second.Session.ID == first.Session.ID {
		t.Fatal("signing in again kept the session token or ID")
	}
	e.wantInvalidSession(t, first.Token, "replaced token")
	if _, err := e.identity.Authenticate(t.Context(), second.Token); err != nil {
		t.Fatalf("Authenticate(new token) = %v", err)
	}
	// A revoked or unknown current token does not stop the sign-in.
	third := e.signIn(t, appidentity.SignInInput{CurrentToken: first.Token})
	e.signIn(t, appidentity.SignInInput{CurrentToken: "garbage"})
	if _, err := e.identity.Authenticate(t.Context(), third.Token); err != nil {
		t.Fatalf("Authenticate(third) = %v", err)
	}
	// A failed sign-in leaves the current session alone.
	if _, err := e.identity.SignIn(t.Context(), appidentity.SignInInput{
		Email: sessionEmail, Password: "wrong password", CurrentToken: third.Token,
	}); !errors.Is(err, appidentity.ErrInvalidCredentials) {
		t.Fatalf("SignIn(wrong password) = %v", err)
	}
	if _, err := e.identity.Authenticate(t.Context(), third.Token); err != nil {
		t.Fatalf("Authenticate after a failed sign-in = %v", err)
	}
}

func sessionExpires(t *testing.T, e *accountEnv) {
	e.sessionUser(t)
	got := e.signIn(t, appidentity.SignInInput{})
	e.clock.Advance(appidentity.DefaultSessionTTL - time.Millisecond)
	if _, err := e.identity.Authenticate(t.Context(), got.Token); err != nil {
		t.Fatalf("Authenticate just before expiry = %v", err)
	}
	e.clock.Advance(time.Millisecond)
	e.wantInvalidSession(t, got.Token, "expired token")
}

func sessionRejectsWrongCredentials(t *testing.T, e *accountEnv) {
	e.sessionUser(t)
	e.register(t, "nopassword") // a user without an email address cannot be signed in to by email
	for _, in := range []appidentity.SignInInput{
		{Email: sessionEmail, Password: "wrong password"},
		{Email: "nobody@example.com", Password: sessionPassword},
		{Email: "", Password: ""},
	} {
		got, err := e.identity.SignIn(t.Context(), in)
		if !errors.Is(err, appidentity.ErrInvalidCredentials) || got.Token != "" {
			t.Errorf("SignIn(%q, %q) = %+v, %v; want ErrInvalidCredentials", in.Email, in.Password, got, err)
		}
	}
}

func sessionRateLimitsFailures(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	e.sessionUser(t)
	fail := func(email, ip string) error {
		_, err := e.identity.SignIn(ctx, appidentity.SignInInput{Email: email, Password: "wrong password", IP: ip})
		return err
	}
	// Five failures from one IP address (IPv6 addresses count per /64) close it for everyone.
	for i := range appidentity.LoginIPLimit.Max {
		ip := fmt.Sprintf("2001:db8::%d", i+1)
		if err := fail("nobody@example.com", ip); !errors.Is(err, appidentity.ErrInvalidCredentials) {
			t.Fatalf("failure #%d = %v, want ErrInvalidCredentials", i+1, err)
		}
	}
	_, err := e.identity.SignIn(ctx, appidentity.SignInInput{Email: sessionEmail, Password: sessionPassword, IP: "2001:db8::ff"})
	var limited *kernel.RateLimitedError
	if !errors.As(err, &limited) || limited.RetryAfter <= 0 {
		t.Fatalf("SignIn from the limited /64 = %v, want a RateLimitedError with a Retry-After", err)
	}
	e.signIn(t, appidentity.SignInInput{Email: sessionEmail, Password: sessionPassword, IP: "192.0.2.1"})

	// Ten failures for one address, from any IP addresses, close the address.
	for i := range appidentity.LoginAccountLimit.Max {
		ip := fmt.Sprintf("198.51.100.%d", i+1)
		if err := fail(sessionEmail, ip); !errors.Is(err, appidentity.ErrInvalidCredentials) {
			t.Fatalf("account failure #%d = %v, want ErrInvalidCredentials", i+1, err)
		}
	}
	_, err = e.identity.SignIn(ctx, appidentity.SignInInput{Email: "ALICE@example.com", Password: sessionPassword, IP: "203.0.113.1"})
	if !errors.Is(err, kernel.ErrRateLimited) {
		t.Fatalf("SignIn to the limited address = %v, want ErrRateLimited", err)
	}
	e.clock.Advance(2 * appidentity.LoginAccountLimit.Window)
	e.signIn(t, appidentity.SignInInput{Email: sessionEmail, Password: sessionPassword, IP: "203.0.113.1"})
}

func sessionRehashes(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.sessionUser(t)
	before, err := e.h.Identity.FindCurrentPasswordCredential(ctx, reg.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	stronger := cheapArgon2
	stronger.Iterations++
	hasher := appidentity.NewPasswordHasher(stronger, 0)
	e.clock.Advance(time.Second)
	if _, err := e.newIdentity(hasher).SignIn(ctx, appidentity.SignInInput{Email: sessionEmail, Password: sessionPassword}); err != nil {
		t.Fatalf("SignIn with stronger parameters = %v", err)
	}
	after, err := e.h.Identity.FindCurrentPasswordCredential(ctx, reg.User.ID)
	if err != nil || after.ID == before.ID {
		t.Fatalf("current credential = %+v, %v; want a new one", after, err)
	}
	if ok, rehash, err := hasher.Verify(ctx, sessionPassword, after.Hash); !ok || rehash || err != nil {
		t.Errorf("new hash Verify = %v, %v, %v; want true, false, nil", ok, rehash, err)
	}
}

func sessionRejectsInactiveAccounts(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.sessionUser(t)
	_, wrongErr := e.identity.SignIn(ctx, appidentity.SignInInput{Email: sessionEmail, Password: "wrong password"})
	for _, kind := range []account.LifecycleKind{account.LifecycleDeactivated, account.LifecycleReactivated} {
		e.clock.Advance(time.Second)
		if _, err := e.accounts.RecordLifecycleEvent(ctx, appaccount.LifecycleInput{
			AccountID: reg.AccountID, Kind: kind, InitiatedBy: account.InitiatedBySelf,
		}); err != nil {
			t.Fatalf("RecordLifecycleEvent(%s) = %v", kind, err)
		}
		_, err := e.identity.SignIn(ctx, appidentity.SignInInput{Email: sessionEmail, Password: sessionPassword})
		if kind == account.LifecycleReactivated {
			if err != nil {
				t.Fatalf("SignIn after reactivation = %v", err)
			}
			continue
		}
		// The right password of an inactive account fails exactly like a wrong one.
		if !errors.Is(err, appidentity.ErrInvalidCredentials) || err.Error() != wrongErr.Error() {
			t.Fatalf("SignIn to a deactivated account = %v, want the wrong-password error %v", err, wrongErr)
		}
	}
}
