// SPDX-License-Identifier: AGPL-3.0-only

package authpages_test

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	appidentity "github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/internal/domain/identity"
	"github.com/mycroblog/my/internal/i18n"
	"github.com/mycroblog/my/internal/interface/authpages"
)

// fakePasskeys hands out the challenges "challenge-1", "challenge-2", …, each of which signs
// in once: the credential "alice" as alice (userAlice), "bob" into bob's code step, and anything
// else fails.
type fakePasskeys struct {
	n       int
	used    map[string]bool
	limited bool
	inputs  []appidentity.PasskeySignInInput
	id      *fakeIdentity
	broken  bool
}

const passkeyOptions = `{"challenge":"Y2hhbGxlbmdl","rpId":"social.example","userVerification":"required"}` //nolint:gosec // G101: made-up options of the test

func (f *fakePasskeys) BeginPasskeySignIn(context.Context) (appidentity.PasskeySignIn, error) {
	if f.broken {
		return appidentity.PasskeySignIn{}, context.DeadlineExceeded
	}
	f.n++
	return appidentity.PasskeySignIn{Options: json.RawMessage(passkeyOptions), Challenge: "challenge-" + strconv.Itoa(f.n)}, nil
}

func (f *fakePasskeys) SignInWithPasskey(_ context.Context, in appidentity.PasskeySignInInput) (appidentity.SignedIn, error) {
	f.inputs = append(f.inputs, in)
	switch {
	case f.limited:
		return appidentity.SignedIn{}, &kernel.RateLimitedError{RetryAfter: 61 * time.Second}
	case !strings.HasPrefix(in.Challenge, "challenge-") || f.used[in.Challenge]:
		return appidentity.SignedIn{}, appidentity.ErrPasskeyCeremonyExpired
	}
	f.used[in.Challenge] = true
	switch string(in.Response) {
	case `{"id":"alice"}`:
		delete(f.id.live, in.CurrentToken)
		token := f.id.newToken(userAlice)
		return appidentity.SignedIn{Token: token, Session: identity.Session{UserID: userAlice, ExpiresAt: time.Now().Add(time.Hour)}}, nil
	case `{"id":"bob"}`:
		return appidentity.SignedIn{}, &appidentity.SecondFactorRequiredError{Challenge: bobChallenge}
	}
	return appidentity.SignedIn{}, appidentity.ErrInvalidPasskey
}

const passkeyScript = "/assets/server-pages-passkey-FFFF.js"

func newPasskeyServer(t *testing.T) (*fakeIdentity, *fakePasskeys, http.Handler) {
	t.Helper()
	b, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	id := newFakeIdentity()
	p := &fakePasskeys{used: map[string]bool{}, id: id}
	m := mux{http.NewServeMux()}
	authpages.New(authpages.Options{
		Identity: id, Locales: &i18n.Resolver{Bundle: b}, Passkeys: p, PasskeyScript: passkeyScript,
	}).Register(m)
	return id, p, m
}

// challengeOf reads the challenge of the page's passkey form.
func challengeOf(t *testing.T, page string) string {
	t.Helper()
	const field = `name="challenge" value="`
	i := strings.Index(page, field)
	if i < 0 {
		t.Fatalf("no passkey challenge in %s", page)
	}
	rest := page[i+len(field):]
	return rest[:strings.IndexByte(rest, '"')]
}

func TestSignInPageOffersPasskeys(t *testing.T) {
	_, _, h := newPasskeyServer(t)
	b := newBrowser(t, h)
	page := b.do(http.MethodGet, authpages.PathSignIn+"?return_to=%2Fhome", nil, nil)
	body := page.Body.String()
	for _, want := range []string{
		`<form class="form" method="post" action="/auth/sign_in/passkey" data-passkey-options="` + html.EscapeString(passkeyOptions) + `" hidden>`,
		`<input type="hidden" name="challenge" value="challenge-1">`,
		`<input type="hidden" name="return_to" value="/home">`,
		`<script type="module" src="` + passkeyScript + `"></script>`,
		`>Sign in with a passkey</button>`,
		`role="alert" data-passkey-error="The passkey was not used.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sign-in page lacks %s:\n%s", want, body)
		}
	}
	csp := page.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe") {
		t.Errorf("CSP = %q, want this site's scripts and nothing unsafe", csp)
	}
	// The password form still works alone, before the passkey form.
	if strings.Index(body, `action="/auth/sign_in"`) > strings.Index(body, `action="/auth/sign_in/passkey"`) {
		t.Error("the passkey form comes before the password form")
	}
}

func TestSignInPageWithoutPasskeys(t *testing.T) {
	b := newBrowser(t, newServer(t, newFakeIdentity()))
	page := b.do(http.MethodGet, authpages.PathSignIn, nil, nil)
	if body := page.Body.String(); strings.Contains(body, "passkey") || strings.Contains(body, "<script") {
		t.Errorf("sign-in page without passkeys:\n%s", body)
	}
	if csp := page.Header().Get("Content-Security-Policy"); strings.Contains(csp, "script-src") {
		t.Errorf("CSP without passkeys = %q, want no scripts", csp)
	}
	w := b.do(http.MethodPost, authpages.PathSignInPasskey, b.form(page, "challenge", "x", "credential", "{}"), nil)
	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST %s without passkeys = %d, want no such route", authpages.PathSignInPasskey, w.Code)
	}
}

func TestSignInPageLeavesPasskeysOutWhenTheyFail(t *testing.T) {
	_, p, h := newPasskeyServer(t)
	p.broken = true
	page := newBrowser(t, h).do(http.MethodGet, authpages.PathSignIn, nil, nil)
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), "data-passkey-options") {
		t.Errorf("sign-in page with a failing passkey start = %d:\n%s", page.Code, page.Body.String())
	}
}

func TestPasskeySignIn(t *testing.T) {
	_, p, h := newPasskeyServer(t)
	b := newBrowser(t, h)
	page := b.do(http.MethodGet, authpages.PathSignIn, nil, nil)
	challenge := challengeOf(t, page.Body.String())
	w := b.do(http.MethodPost, authpages.PathSignInPasskey,
		b.form(page, "challenge", challenge, "credential", `{"id":"alice"}`, "return_to", "/home"),
		map[string]string{"User-Agent": "test-agent"})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/home" {
		t.Fatalf("POST passkey = %d → %q, want 303 to return_to:\n%s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	if _, ok := b.cookies[authpages.CookieSession]; !ok {
		t.Error("the passkey started no session")
	}
	last := p.inputs[len(p.inputs)-1]
	if last.Challenge != challenge || string(last.Response) != `{"id":"alice"}` || last.IP != "192.0.2.1" || last.UserAgent != "test-agent" {
		t.Errorf("SignInWithPasskey input = %+v", last)
	}

	// A user with two-factor authentication goes on to the code form.
	b = newBrowser(t, h)
	page = b.do(http.MethodGet, authpages.PathSignIn, nil, nil)
	w = b.do(http.MethodPost, authpages.PathSignInPasskey, b.form(page, "challenge", challengeOf(t, page.Body.String()), "credential", `{"id":"bob"}`), nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != authpages.PathSignInTwoFactor || b.cookies[authpages.CookieTwoFactor] != bobChallenge {
		t.Errorf("POST passkey of bob = %d → %q, want the code form", w.Code, w.Header().Get("Location"))
	}
	if _, ok := b.cookies[authpages.CookieSession]; ok {
		t.Error("the passkey of a user with two-factor authentication started a session")
	}
}

func TestPasskeySignInFailures(t *testing.T) {
	_, p, h := newPasskeyServer(t)
	b := newBrowser(t, h)
	page := b.do(http.MethodGet, authpages.PathSignIn, nil, nil)
	used := challengeOf(t, page.Body.String())
	b.do(http.MethodPost, authpages.PathSignInPasskey, b.form(page, "challenge", used, "credential", `{"id":"mallory"}`), nil)
	b.cookies = map[string]string{authpages.CookieCSRF: b.cookies[authpages.CookieCSRF]}

	for _, tc := range []struct {
		name, challenge, credential, want string
		status                            int
	}{
		{"an unknown passkey", "challenge-9", `{"id":"mallory"}`, "The passkey could not be verified.", http.StatusUnprocessableEntity},
		{"a used challenge", used, `{"id":"alice"}`, "The sign-in with a passkey took too long.", http.StatusUnprocessableEntity},
		{"no credential", "challenge-8", "", "The passkey was not used.", http.StatusUnprocessableEntity},
	} {
		w := b.do(http.MethodPost, authpages.PathSignInPasskey, b.form(page, "challenge", tc.challenge, "credential", tc.credential), nil)
		body := w.Body.String()
		if w.Code != tc.status || !strings.Contains(body, `<p id="passkey-error" class="alert" role="alert"`) || !strings.Contains(body, tc.want) {
			t.Errorf("%s = %d, want %d with %q:\n%s", tc.name, w.Code, tc.status, tc.want, body)
		}
		// The alert belongs to the passkey form, which is shown; the password fields are not marked.
		if strings.Contains(body, `aria-invalid="true"`) || !strings.Contains(body, `data-passkey-options="`) || strings.Contains(body, `" hidden>`+"\n"+`<input type="hidden" name="csrf_token"`) {
			t.Errorf("%s marks the password form or hides the passkey form:\n%s", tc.name, body)
		}
		if _, ok := b.cookies[authpages.CookieSession]; ok {
			t.Errorf("%s started a session", tc.name)
		}
	}

	p.limited = true
	w := b.do(http.MethodPost, authpages.PathSignInPasskey, b.form(page, "challenge", "challenge-7", "credential", `{"id":"alice"}`), nil)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "61" {
		t.Errorf("rate limited = %d (Retry-After %q)", w.Code, w.Header().Get("Retry-After"))
	}
	p.limited = false

	// The anti-forgery token is checked before the passkey.
	n := len(p.inputs)
	delete(b.cookies, authpages.CookieCSRF)
	w = b.do(http.MethodPost, authpages.PathSignInPasskey, b.form(page, "challenge", "challenge-6", "credential", `{"id":"alice"}`), nil)
	if w.Code != http.StatusForbidden || len(p.inputs) != n {
		t.Errorf("without the CSRF cookie = %d (%d sign-ins tried), want 403 and none", w.Code, len(p.inputs)-n)
	}
}
