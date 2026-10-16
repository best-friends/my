// SPDX-License-Identifier: AGPL-3.0-only

package authpages_test

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/authpages"
)

const (
	webRedirect = "https://web.example/login/callback"
	appRedirect = "https://app.example/cb?keep=1"
)

// fakeOAuth knows a first-party client "web" and a third-party client "app", and requires
// response_type=code.
type fakeOAuth struct {
	issued []appidentity.Authorization
}

func (f *fakeOAuth) ValidateAuthorization(_ context.Context, req appidentity.AuthorizationRequest) (appidentity.Authorization, error) {
	clients := map[string]struct {
		c   identity.OAuthClient
		uri string
	}{
		"web": {identity.OAuthClient{ID: "1", ClientID: "web", Name: "Web", FirstParty: true}, webRedirect},
		"app": {identity.OAuthClient{ID: "2", ClientID: "app", Name: "Example App"}, appRedirect},
	}
	c, ok := clients[req.ClientID]
	if !ok {
		return appidentity.Authorization{}, appidentity.ErrUnknownClient
	}
	if req.RedirectURI != c.uri {
		return appidentity.Authorization{}, appidentity.ErrInvalidRedirectURI
	}
	a := appidentity.Authorization{Client: c.c, RedirectURI: c.uri, State: req.State, CodeChallenge: req.CodeChallenge, Scope: "read write"}
	if req.ResponseType != "code" {
		return a, &appidentity.OAuthError{Code: appidentity.OAuthUnsupportedResponseType}
	}
	return a, nil
}

func (f *fakeOAuth) IssueAuthorizationCode(_ context.Context, a appidentity.Authorization, _ string) (string, error) {
	f.issued = append(f.issued, a)
	return "my_ac_code", nil
}

func newOAuthServer(t *testing.T, id authpages.Identity, oauth *fakeOAuth) http.Handler {
	t.Helper()
	b, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	m := mux{http.NewServeMux()}
	authpages.New(authpages.Options{
		Identity: id, OAuth: oauth, Issuer: "https://social.example", FormActionOrigins: []string{"https://web.example"},
		Locales: &i18n.Resolver{Bundle: b},
	}).Register(m)
	return m
}

func authorizeQuery(clientID, redirectURI string) string {
	return authpages.PathAuthorize + "?" + url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectURI}, "state": {"st"},
		"code_challenge": {strings.Repeat("c", 43)}, "code_challenge_method": {"S256"},
	}.Encode()
}

// signedIn returns a browser with a session.
func signedIn(t *testing.T, h http.Handler) *browser {
	t.Helper()
	b := newBrowser(t, h)
	page := b.do(http.MethodGet, authpages.PathSignIn, nil, nil)
	if w := b.do(http.MethodPost, authpages.PathSignIn, b.form(page, "email", "alice@example.com", "password", "correct horse"), nil); w.Code != http.StatusSeeOther {
		t.Fatalf("sign-in = %d", w.Code)
	}
	return b
}

func location(t *testing.T, w *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", w.Code, w.Body.String())
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAuthorizeSendsUnsignedBrowsersToSignIn(t *testing.T) {
	oauth := &fakeOAuth{}
	h := newOAuthServer(t, newFakeIdentity(), oauth)
	b := newBrowser(t, h)
	target := authorizeQuery("web", webRedirect)
	u := location(t, b.do(http.MethodGet, target, nil, nil))
	if u.Path != authpages.PathSignIn || u.Query().Get("return_to") != target {
		t.Fatalf("redirect = %s, want the sign-in form returning to %s", u, target)
	}
	// Signing in returns to the authorization request, which then answers the client.
	page := b.do(http.MethodGet, u.String(), nil, nil)
	if !strings.Contains(page.Body.String(), `name="return_to"`) {
		t.Fatalf("the sign-in form lost return_to:\n%s", page.Body.String())
	}
	back := location(t, b.do(http.MethodPost, authpages.PathSignIn, b.form(page, "email", "alice@example.com", "password", "correct horse",
		"return_to", target), nil))
	if back.String() != target {
		t.Fatalf("sign-in returned to %s, want %s", back, target)
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' https://web.example;") {
		t.Errorf("sign-in CSP = %q, want the web origin in form-action", csp)
	}
	if len(oauth.issued) != 0 {
		t.Error("a code was issued without a session")
	}
}

func TestAuthorizeSkipsConsentForFirstPartyClients(t *testing.T) {
	oauth := &fakeOAuth{}
	b := signedIn(t, newOAuthServer(t, newFakeIdentity(), oauth))
	u := location(t, b.do(http.MethodGet, authorizeQuery("web", webRedirect), nil, nil))
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != webRedirect || q.Get("code") != "my_ac_code" || q.Get("state") != "st" ||
		q.Get("iss") != "https://social.example" {
		t.Fatalf("redirect = %s, want the code, state, and iss at %s", u, webRedirect)
	}
	// The code records the session, so that the web client's sign-out can end it.
	if session := b.cookies[authpages.CookieSession]; len(oauth.issued) != 1 || oauth.issued[0].SessionID != session {
		t.Errorf("issued = %+v, want one code of the session %q", oauth.issued, session)
	}
}

func TestAuthorizeIssuesNoCodeToAccountsThatCannotBeUsedYet(t *testing.T) {
	oauth := &fakeOAuth{}
	id := newFakeIdentity()
	b := signedIn(t, newOAuthServer(t, id, oauth))
	id.access["u1"] = appidentity.Access{Registration: identity.RegistrationPending, EmailVerified: true}
	w := b.do(http.MethodGet, authorizeQuery("web", webRedirect), nil, nil)
	if w.Code != http.StatusForbidden || w.Header().Get("Location") != "" || len(oauth.issued) != 0 {
		t.Fatalf("authorize for a pending registration = %d → %q, issued %d; want the 403 status page and no code",
			w.Code, w.Header().Get("Location"), len(oauth.issued))
	}
}

func TestAuthorizeRefusesUnverifiedRedirects(t *testing.T) {
	b := signedIn(t, newOAuthServer(t, newFakeIdentity(), &fakeOAuth{}))
	for _, target := range []string{
		authorizeQuery("nobody", webRedirect),
		authorizeQuery("web", "https://evil.example/cb"),
		authorizeQuery("web", ""),
	} {
		w := b.do(http.MethodGet, target, nil, map[string]string{"Accept-Language": "en"})
		if w.Code != http.StatusBadRequest || w.Header().Get("Location") != "" ||
			!strings.Contains(w.Body.String(), "authorization request is invalid") {
			t.Errorf("GET %s = %d → %q, want the 400 page without a redirect", target, w.Code, w.Header().Get("Location"))
		}
	}
	// Other errors go back to the verified redirect URI.
	target := strings.Replace(authorizeQuery("app", appRedirect), "response_type=code", "response_type=token", 1)
	u := location(t, b.do(http.MethodGet, target, nil, nil))
	if q := u.Query(); q.Get("error") != appidentity.OAuthUnsupportedResponseType || q.Get("state") != "st" || q.Get("keep") != "1" {
		t.Errorf("error redirect = %s, want unsupported_response_type with the state and the URI's own query", u)
	}
}

func TestAuthorizeAsksConsentForThirdPartyClients(t *testing.T) {
	oauth := &fakeOAuth{}
	b := signedIn(t, newOAuthServer(t, newFakeIdentity(), oauth))
	page := b.do(http.MethodGet, authorizeQuery("app", appRedirect), nil, map[string]string{"Accept-Language": "en"})
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, "<h1>Authorize Example App</h1>") ||
		!strings.Contains(body, "Read your account, posts, and timelines") || !strings.Contains(body, "app.example") {
		t.Fatalf("consent page = %d\n%s", page.Code, body)
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' https://web.example https://app.example;") {
		t.Errorf("consent CSP = %q, want the redirect origin in form-action", csp)
	}
	if len(oauth.issued) != 0 {
		t.Fatal("a code was issued before consent")
	}
	fields := func(decision string) url.Values {
		v := b.form(page, "decision", decision)
		for _, k := range []string{"response_type", "client_id", "redirect_uri", "scope", "state", "code_challenge", "code_challenge_method"} {
			m := strings.SplitN(body, `name="`+k+`" value="`, 2)
			if len(m) != 2 {
				t.Fatalf("consent form has no %s", k)
			}
			raw, _, _ := strings.Cut(m[1], `"`)
			v.Set(k, html.UnescapeString(raw))
		}
		return v
	}

	// Without the anti-forgery token or from another site, nothing happens.
	noToken := fields(decisionApprove)
	noToken.Del("csrf_token")
	if w := b.do(http.MethodPost, authpages.PathAuthorize, noToken, nil); w.Code != http.StatusForbidden {
		t.Errorf("consent without csrf_token = %d, want 403", w.Code)
	}
	if w := b.do(http.MethodPost, authpages.PathAuthorize, fields(decisionApprove), map[string]string{"Sec-Fetch-Site": "cross-site"}); w.Code != http.StatusForbidden {
		t.Errorf("cross-site consent = %d, want 403", w.Code)
	}
	if len(oauth.issued) != 0 {
		t.Fatal("a refused post issued a code")
	}

	denied := location(t, b.do(http.MethodPost, authpages.PathAuthorize, fields(decisionDeny), nil))
	if q := denied.Query(); q.Get("error") != appidentity.OAuthAccessDenied || q.Get("code") != "" || q.Get("state") != "st" {
		t.Errorf("denied = %s, want access_denied", denied)
	}
	approved := location(t, b.do(http.MethodPost, authpages.PathAuthorize, fields(decisionApprove), nil))
	if q := approved.Query(); q.Get("code") != "my_ac_code" || q.Get("keep") != "1" || q.Get("iss") != "https://social.example" {
		t.Errorf("approved = %s, want the code", approved)
	}
	if len(oauth.issued) != 1 || oauth.issued[0].Client.ClientID != "app" {
		t.Errorf("issued = %+v, want one code for app", oauth.issued)
	}
}

const (
	decisionApprove = "approve"
	decisionDeny    = "deny"
)
