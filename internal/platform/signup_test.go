// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/app/identity"
	appinstance "github.com/best-friends/my/internal/app/instance"
	domainidentity "github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/interface/authpages"
	"github.com/best-friends/my/internal/platform"
)

var loggedLink = regexp.MustCompile(`https://example\.test(/auth/verify_email\?token=[A-Za-z0-9_%-]+)`)

// TestSignUpScreens drives registration in each mode on the real database, with mail = none:
// verification emails go to the log, and a moderator's approval stands in for verification.
func TestSignUpScreens(t *testing.T) {
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	s := start(t, cfg, platform.ServeOptions{}, nil)
	ctx := t.Context()
	setMode := func(mode instance.RegistrationMode) {
		t.Helper()
		_, err := s.app.Instance.SetSetting(ctx, appinstance.SetSettingInput{Key: instance.SettingRegistrationMode, Value: string(mode)})
		if err != nil {
			t.Fatal(err)
		}
	}
	signUp := func(fields ...string) (*authClient, *http.Response) {
		t.Helper()
		c := &authClient{t: t, s: s, cookies: map[string]string{}}
		resp, _ := c.do(http.MethodPost, authpages.PathSignUp, c.formAt(authpages.PathSignUp, fields...), nil)
		return c, resp
	}
	status := func(c *authClient) string {
		t.Helper()
		_, body := c.do(http.MethodGet, authpages.PathSignIn, nil, nil)
		return body
	}
	userID := func(email string) string {
		t.Helper()
		reg, err := s.app.Identity.ListRegistrations(ctx, domainidentity.RegistrationPending, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range reg {
			if r.Email == email {
				return r.Application.UserID
			}
		}
		t.Fatalf("no pending registration of %s in %+v", email, reg)
		return ""
	}

	// closed: the page says so and the form is refused.
	setMode(instance.RegistrationClosed)
	c := &authClient{t: t, s: s, cookies: map[string]string{}}
	if resp, body := c.do(http.MethodGet, authpages.PathSignUp, nil, nil); resp.StatusCode != http.StatusForbidden ||
		!strings.Contains(body, "not accepting new accounts") {
		t.Fatalf("closed GET sign_up = %d\n%s", resp.StatusCode, body)
	}

	// open: without email delivery, the new user waits for a moderator (FR-002).
	setMode(instance.RegistrationOpen)
	alice, resp := signUp("username", "alice", "email", "alice@example.com", "password", "correct horse battery")
	if resp.StatusCode != http.StatusSeeOther || alice.cookies[authpages.CookieSession] == "" {
		t.Fatalf("open sign-up = %d", resp.StatusCode)
	}
	if body := status(alice); !strings.Contains(body, "waiting for a moderator") || strings.Contains(body, "verify_email/resend") {
		t.Errorf("open sign-up status page:\n%s", body)
	}

	// The verification email went to the log and only there: the access log has no token.
	logs := s.logs.String()
	m := loggedLink.FindStringSubmatch(logs)
	if m == nil || !strings.Contains(logs, "mail not sent (mail = none)") {
		t.Fatalf("no verification link in the log:\n%s", logs)
	}
	link := m[1]
	token := strings.TrimPrefix(link, "/auth/verify_email?token=")
	if strings.Count(logs, token) != 1 {
		t.Errorf("the token appears %d times in the log", strings.Count(logs, token))
	}
	resp, body := alice.do(http.MethodGet, link, nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `name="token" value="`+html.EscapeString(token)+`"`) {
		t.Fatalf("GET %s = %d\n%s", link, resp.StatusCode, body)
	}
	if resp, body := alice.do(http.MethodPost, authpages.PathVerifyEmail, alice.formAt(link, "token", token), nil); resp.StatusCode != http.StatusOK ||
		!strings.Contains(body, "Your email address is confirmed.") {
		t.Fatalf("POST verify_email = %d\n%s", resp.StatusCode, body)
	}
	if resp, _ := alice.do(http.MethodPost, authpages.PathVerifyEmail, alice.formAt(link, "token", token), nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("second POST verify_email = %d, want 422", resp.StatusCode)
	}
	if strings.Count(s.logs.String(), token) != 1 {
		t.Error("confirming the email logged the token")
	}

	// approval: the reason reaches the moderator, whose approval lets the user in.
	setMode(instance.RegistrationApproval)
	bob, resp := signUp("username", "bob", "email", "bob@example.com", "password", "correct horse battery", "reason", "Friend of alice")
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("approval sign-up = %d", resp.StatusCode)
	}
	if body := status(bob); !strings.Contains(body, "waiting for a moderator") {
		t.Errorf("approval sign-up status page:\n%s", body)
	}
	pending, err := s.app.Identity.ListRegistrations(ctx, domainidentity.RegistrationPending, 10)
	if err != nil || len(pending) != 2 || pending[1].Username != "bob" || pending[1].Application.Reason != "Friend of alice" {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	_, err = s.app.Identity.DecideRegistration(ctx, identity.DecideInput{UserID: userID("bob@example.com"), Decision: domainidentity.DecisionApproved})
	if err != nil {
		t.Fatal(err)
	}
	if body := status(bob); !strings.Contains(body, "You are signed in") {
		t.Errorf("approved user's page:\n%s", body)
	}

	// invite: a valid invite admits the user at once.
	setMode(instance.RegistrationInvite)
	_, resp = signUp("username", "carol", "email", "carol@example.com", "password", "correct horse battery", "invite_code", "WRONGCODE")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("sign-up with a wrong invite = %d, want 422", resp.StatusCode)
	}
	inv, err := s.app.Identity.CreateInvite(ctx, identity.CreateInviteInput{MaxUses: 1})
	if err != nil {
		t.Fatal(err)
	}
	carol, resp := signUp("username", "carol", "email", "carol@example.com", "password", "correct horse battery", "invite_code", inv.Code)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("invite sign-up = %d", resp.StatusCode)
	}
	if body := status(carol); !strings.Contains(body, "You are signed in") {
		t.Errorf("invited user's page:\n%s", body)
	}

	// The fifth attempt from this address within the window is the last one allowed.
	_, resp = signUp("username", "dave", "email", "dave@example.com", "password", "correct horse battery", "invite_code", inv.Code)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("sign-up with a used-up invite = %d, want 422", resp.StatusCode)
	}
	_, resp = signUp("username", "erin", "email", "erin@example.com", "password", "correct horse battery", "invite_code", inv.Code)
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("sixth sign-up = %d, want 429", resp.StatusCode)
	}
}
