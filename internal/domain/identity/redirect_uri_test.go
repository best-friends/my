// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
)

func TestValidateRedirectURI(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://example.com/cb":                 true,
		"https://app.example.com/oauth/callback": true,
		"com.example.app:/cb":                    true,
		"com.example.app://cb":                   true,
		"http://127.0.0.1/cb":                    true,
		"http://127.0.0.1:53124/cb":              true,
		"http://[::1]/cb":                        true,
		"http://[::1]:8080/cb":                   true,
		"http://localhost/cb":                    false,
		"http://example.com/cb":                  false,
		"urn:ietf:wg:oauth:2.0:oob":              false,
		"javascript:alert(1)":                    false,
		"data:text/html,x":                       false,
		"myapp:/cb":                              false,
		"file:///etc/passwd":                     false,
		"https://example.com/cb#frag":            false,
		"https://user:pw@example.com/cb":         false,
		"com.example.app:/cb#x":                  false,
		"https:///cb":                            false,
		"/cb":                                    false,
		"":                                       false,
	} {
		err := identity.ValidateRedirectURI(raw)
		if ok != (err == nil) {
			t.Errorf("ValidateRedirectURI(%q) = %v, want ok=%v", raw, err, ok)
		}
	}
}

func TestMatchRedirectURI(t *testing.T) {
	for _, c := range []struct {
		registered, requested string
		want                  bool
	}{
		{"http://127.0.0.1/cb", "http://127.0.0.1/cb", true},
		{"http://127.0.0.1/cb", "http://127.0.0.1:53124/cb", true},
		{"http://[::1]/cb", "http://[::1]:53124/cb", true},
		{"http://127.0.0.1/cb", "http://127.0.0.1:53124/other", false},
		{"http://127.0.0.1/cb", "http://[::1]:53124/cb", false},
		{"http://127.0.0.1/cb", "http://localhost:53124/cb", false},
		{"http://127.0.0.1/cb", "https://127.0.0.1:53124/cb", false},
		{"http://127.0.0.1/cb", "http://127.0.0.1:99999/cb", false},
		{"http://127.0.0.1:8080/cb", "http://127.0.0.1:9090/cb", false},
		{"https://example.com/cb", "https://example.com:8443/cb", false},
		{"https://example.com/cb", "https://example.com/cb", true},
		{"com.example.app:/cb", "com.example.app:/cb", true},
		{"", "", false},
	} {
		if got := identity.MatchRedirectURI(c.registered, c.requested); got != c.want {
			t.Errorf("MatchRedirectURI(%q, %q) = %v, want %v", c.registered, c.requested, got, c.want)
		}
	}
}
