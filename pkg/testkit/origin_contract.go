// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"errors"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
)

// The origin cases of AccountRepositoryContract and RegistrationRepositoryContract: where
// sign-ins and registrations came from, and their purge after the retention (ADR-0038).

// purgeOrigins runs the origin purge with batch rows per statement and returns how many rows it
// deleted.
func (e *accountEnv) purgeOrigins(t *testing.T, batch int) int64 {
	t.Helper()
	n, err := appidentity.NewOriginPurger(appidentity.OriginPurgeDeps{
		UnitOfWork: e.h.UnitOfWork, Repository: e.h.Identity, Clock: e.clock, Batch: batch,
	}).Run(t.Context())
	if err != nil {
		t.Fatalf("purge origins = %v", err)
	}
	return n
}

func (e *accountEnv) wantNoSessionOrigin(t *testing.T, sessionID, why string) {
	t.Helper()
	if o, err := e.h.Identity.FindSessionOrigin(t.Context(), sessionID); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindSessionOrigin(%s) = %+v, %v; want ErrNotFound", why, o, err)
	}
}

func originRecordsSignIns(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	e.sessionUser(t)
	// An IPv4-mapped IPv6 address is recorded as IPv4, so that the same address compares equal.
	mapped := e.signIn(t, appidentity.SignInInput{IP: "::ffff:192.0.2.7", UserAgent: "ua"})
	if o, err := e.h.Identity.FindSessionOrigin(ctx, mapped.Session.ID); err != nil || o.IP != "192.0.2.7" || o.UserAgent != "ua" {
		t.Errorf("origin of a mapped address = %+v, %v; want 192.0.2.7", o, err)
	}
	// An unparsable address is left out; the User-Agent is still recorded.
	garbled := e.signIn(t, appidentity.SignInInput{IP: "not an address", UserAgent: "ua"})
	if o, err := e.h.Identity.FindSessionOrigin(ctx, garbled.Session.ID); err != nil || o.IP != "" || o.UserAgent != "ua" {
		t.Errorf("origin of an unparsable address = %+v, %v; want only the user agent", o, err)
	}
	// Without an address or a User-Agent the sign-in succeeds and records no origin.
	unknown := e.signIn(t, appidentity.SignInInput{})
	if _, err := e.identity.Authenticate(ctx, unknown.Token); err != nil {
		t.Fatalf("Authenticate(session without origin) = %v", err)
	}
	e.wantNoSessionOrigin(t, unknown.Session.ID, "session without origin")
}

func originPurgesSessionOrigins(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	e.sessionUser(t)
	old := e.signIn(t, appidentity.SignInInput{IP: "192.0.2.1"})
	e.clock.Advance(time.Hour)
	recent := e.signIn(t, appidentity.SignInInput{IP: "192.0.2.2"})
	e.clock.Advance(appidentity.DefaultSessionOriginRetention - time.Hour)
	if n := e.purgeOrigins(t, 0); n != 0 {
		t.Fatalf("purge at exactly the retention = %d, want 0", n)
	}
	e.clock.Advance(time.Millisecond)
	if n := e.purgeOrigins(t, 1); n != 1 {
		t.Fatalf("purge past the retention of the first = %d, want 1", n)
	}
	e.wantNoSessionOrigin(t, old.Session.ID, "purged")
	if o, err := e.h.Identity.FindSessionOrigin(ctx, recent.Session.ID); err != nil || o.IP != "192.0.2.2" {
		t.Errorf("origin within the retention = %+v, %v; want it kept", o, err)
	}
	e.clock.Advance(time.Hour)
	if n := e.purgeOrigins(t, 1); n != 1 {
		t.Fatalf("second purge = %d, want 1", n)
	}
	e.wantNoSessionOrigin(t, recent.Session.ID, "purged later")
	if n := e.purgeOrigins(t, 1); n != 0 {
		t.Errorf("repeated purge = %d, want 0", n)
	}
}

func registrationRecordsOrigins(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	r := e.registration(identity.RegistrationOpen, true)
	in := signUpInput("alice")
	in.IP = "2001:db8::1%eth0"
	alice := r.signUp(t, in)
	o, err := e.h.Identity.FindRegistrationOrigin(ctx, alice.User.ID)
	if err != nil || o.IP != "2001:db8::1" || !o.RecordedAt.Equal(alice.User.CreatedAt) {
		t.Errorf("FindRegistrationOrigin = %+v, %v; want 2001:db8::1 at %v", o, err, alice.User.CreatedAt)
	}
	if so, err := e.h.Identity.FindSessionOrigin(ctx, alice.Session.Session.ID); err != nil || so.IP != "2001:db8::1" || so.UserAgent != in.UserAgent {
		t.Errorf("origin of the sign-up session = %+v, %v", so, err)
	}

	// A sign-up without an address succeeds and records no registration origin.
	bob := r.signUp(t, signUpInput("bob"))
	if o, err := e.h.Identity.FindRegistrationOrigin(ctx, bob.User.ID); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindRegistrationOrigin(no address) = %+v, %v; want ErrNotFound", o, err)
	}
	// Only the registration page records one.
	admin, err := r.CreateAdmin(ctx, appidentity.CreateAdminInput{Username: "root", Email: "root@example.com", Password: "admin password"})
	if err != nil {
		t.Fatal(err)
	}
	if o, err := e.h.Identity.FindRegistrationOrigin(ctx, admin.User.ID); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindRegistrationOrigin(admin) = %+v, %v; want ErrNotFound", o, err)
	}

	// Past the retention, the purge erases the registration's origin and its session's.
	e.clock.Advance(appidentity.DefaultRegistrationOriginRetention)
	if n := e.purgeOrigins(t, 0); n != 0 {
		t.Fatalf("purge at exactly the retention = %d, want 0", n)
	}
	e.clock.Advance(time.Millisecond)
	if n := e.purgeOrigins(t, 0); n != 3 { // alice's registration and session, bob's session
		t.Fatalf("purge past the retention = %d, want 3", n)
	}
	if o, err := e.h.Identity.FindRegistrationOrigin(ctx, alice.User.ID); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindRegistrationOrigin after the purge = %+v, %v; want ErrNotFound", o, err)
	}
	if _, err := e.h.Identity.FindUser(ctx, alice.User.ID); err != nil {
		t.Errorf("FindUser after the purge = %v; want the user kept", err)
	}
}
