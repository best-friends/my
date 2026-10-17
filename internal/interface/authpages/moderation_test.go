// SPDX-License-Identifier: AGPL-3.0-only

package authpages_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/moderation"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/authpages"
)

type fakeModeration struct {
	actions map[string][]appmoderation.TargetActionView
	filters []appmoderation.OwnActionFilter
}

func (f *fakeModeration) OwnActions(_ context.Context, accountID string, flt appmoderation.OwnActionFilter, _ int) ([]appmoderation.TargetActionView, error) {
	f.filters = append(f.filters, flt)
	return f.actions[accountID], nil
}

type fakeUserLocales map[string]string

func (f fakeUserLocales) UserLocale(_ context.Context, userID string) (string, error) {
	return f[userID], nil
}

// TestStatusOfARestrictedUser checks that a user whom moderation keeps out of the app signs in
// to the screens and sees the actions in effect, in their saved locale, and gets no
// authorization code.
func TestStatusOfARestrictedUser(t *testing.T) {
	id := newFakeIdentity()
	id.access["u1"] = appidentity.Access{
		Registration: identity.RegistrationApproved, EmailVerified: true, AccountID: "acc1", AccountActive: true, LoginRestricted: true,
	}
	ends := time.Now().Add(72*time.Hour - time.Minute)
	mod := &fakeModeration{actions: map[string][]appmoderation.TargetActionView{"acc1": {
		{ID: "a2", Kind: moderation.ActionSuspend, ReasonCategory: moderation.CategorySpam, PublicReason: "spam <wave>",
			ExpiresAt: &ends, State: moderation.ActionActive},
		{ID: "a1", Kind: moderation.ActionDisableLogin, ReasonCategory: moderation.CategoryOther, State: moderation.ActionActive},
		{ID: "a0", Kind: moderation.ActionWarn, ReasonCategory: moderation.CategoryOther, PublicReason: "a warning", State: moderation.ActionActive},
	}}}
	b, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	oauth := &fakeOAuth{}
	m := mux{http.NewServeMux()}
	authpages.New(authpages.Options{
		Identity: id, OAuth: oauth, Locales: &i18n.Resolver{Bundle: b}, Moderation: mod, UserLocales: fakeUserLocales{"u1": "ja"},
	}).Register(m)
	br := newBrowser(t, m)

	page := br.do(http.MethodGet, authpages.PathSignIn, nil, map[string]string{"Accept-Language": "en"})
	w := br.do(http.MethodPost, authpages.PathSignIn, br.form(page, "email", "alice@example.com", "password", "correct horse", "return_to", "/home"), nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != authpages.PathSignIn {
		t.Fatalf("sign-in of a restricted user = %d → %q, want the status page", w.Code, w.Header().Get("Location"))
	}
	w = br.do(http.MethodGet, authpages.PathSignIn, nil, map[string]string{"Accept-Language": "en"})
	if w.Code != http.StatusOK || w.Header().Get("Content-Language") != "ja" {
		t.Fatalf("status page = %d in %q, want 200 in the user's locale", w.Code, w.Header().Get("Content-Language"))
	}
	body := w.Body.String()
	wantContains(t, body,
		`<h1>アカウントは処分を受けています</h1>`,
		`<strong>凍結（アプリを使えず、プロフィールと投稿が隠れる）</strong>`,
		`<p class="hint">理由: スパム</p>`,
		`<p dir="auto">spam &lt;wave&gt;</p>`,
		`<time datetime="`+ends.UTC().Format(time.RFC3339)+`">あと 3 日で終わります</time>`,
		`<strong>ログインの停止</strong>`,
		`<p class="hint">モデレーターが解除するまで続きます</p>`,
		`action="/auth/sign_out"`,
	)
	wantLacks(t, body, "verify_email/resend", "承認待ち", "a warning")
	if len(mod.filters) == 0 || !mod.filters[0].Active {
		t.Errorf("filters = %+v, want the actions in effect", mod.filters)
	}

	// The authorization endpoint gives no code; it shows the same page.
	w = br.do(http.MethodGet, authorizeQuery("web", webRedirect), nil, nil)
	if w.Code != http.StatusForbidden || len(oauth.issued) != 0 {
		t.Fatalf("authorize of a restricted user = %d with %d codes, want 403 and none", w.Code, len(oauth.issued))
	}
	wantContains(t, w.Body.String(), `<strong>ログインの停止</strong>`)
}
