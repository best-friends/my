// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"net/http"
	"net/url"
	"testing"

	appfederation "github.com/mycroblog/my/internal/app/federation"
)

func TestSignatureKeyHost(t *testing.T) {
	for keyID, want := range map[string]string{
		"https://remote.example/users/bob#main-key":  "remote.example",
		"https://Remote.Example:8443/actor/main-key": "remote.example",
		"https://例え.jp/users/a#main-key":             "xn--r8jz45g.jp",
		"/users/bob#main-key":                        "",
		"https://exa mple.com/key":                   "",
	} {
		s := &appfederation.Signature{KeyID: keyID}
		if got := s.KeyHost(); got != want {
			t.Errorf("KeyHost(%q) = %q, want %q", keyID, got, want)
		}
	}
}

func TestNewSignedRequest(t *testing.T) {
	origin, _ := url.Parse("https://social.example")
	r, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:8080/ap/tags/%E6%97%A5?page=2", nil)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("X-Forwarded-Host", "evil.example")
	sr := appfederation.NewSignedRequest(origin, r, []byte("{}"))
	if got := sr.URL.String(); got != "https://social.example/ap/tags/%E6%97%A5?page=2" {
		t.Errorf("URL = %s", got)
	}
	if got := sr.Header.Get("Host"); got != "social.example" {
		t.Errorf("Host = %q, want the public origin's host", got)
	}
}
