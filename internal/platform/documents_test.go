// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/interface/authpages"
	"github.com/best-friends/my/internal/platform"
)

// TestDocumentPages publishes the terms of service written in Markdown on the real database:
// the public page renders and sanitizes them, the registration page asks to agree to them, and
// GET /instance links to them; the privacy policy, which has no text, is a 404 and unlinked.
func TestDocumentPages(t *testing.T) {
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	s := start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		_, err := a.Instance.SetText(t.Context(), appinstance.SetTextInput{
			Key: instance.TextTerms, Locale: "ja", Body: "# 第1条\n\n**親切に**。<script>alert(1)</script>\n\n[ヘルプ](javascript:alert(1))",
		})
		if err != nil {
			t.Fatalf("SetText: %v", err)
		}
	})
	c := &authClient{t: t, s: s, cookies: map[string]string{}}
	ja := map[string]string{"Accept-Language": "ja"}

	resp, body := c.do(http.MethodGet, authpages.PathTerms, nil, ja)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET terms = %d\n%s", resp.StatusCode, body)
	}
	for _, want := range []string{`<h1>利用規約</h1>`, `<div class="document" lang="ja" dir="auto">`, `<h2>第1条</h2>`, `<strong>親切に</strong>`} {
		if !strings.Contains(body, want) {
			t.Errorf("terms page lacks %s:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"<script", "javascript:", authpages.PathPrivacyPolicy} {
		if strings.Contains(body, unwanted) {
			t.Errorf("terms page has %s:\n%s", unwanted, body)
		}
	}
	if resp, _ := c.do(http.MethodGet, authpages.PathPrivacyPolicy, nil, ja); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET privacy without a policy = %d, want 404", resp.StatusCode)
	}
	// An en reader finds no terms: they exist only in ja, and the default locale is en.
	if resp, _ := c.do(http.MethodGet, authpages.PathTerms, nil, map[string]string{"Accept-Language": "en"}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET terms in en = %d, want 404", resp.StatusCode)
	}

	if _, body := c.do(http.MethodGet, authpages.PathSignUp, nil, ja); !strings.Contains(body, `id="sign-up-agreement"`) ||
		!strings.Contains(body, `<a href="/auth/terms">利用規約</a>`) {
		t.Errorf("sign-up page lacks the agreement:\n%s", body)
	}

	resp, body = c.do(http.MethodGet, "/api/v1/instance", nil, ja)
	var got struct {
		TermsURL   string `json:"terms_url"`
		PrivacyURL string `json:"privacy_policy_url"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /instance = %d %v\n%s", resp.StatusCode, err, body)
	}
	if got.TermsURL != cfg.ServerOrigin()+authpages.PathTerms || got.PrivacyURL != "" {
		t.Errorf("instance links = %+v, want the terms only", got)
	}
}
