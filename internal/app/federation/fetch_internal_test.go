// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"testing"
)

func TestIsActivityStreams(t *testing.T) {
	for ct, want := range map[string]bool{
		"application/activity+json":                                                                  true,
		"application/activity+json; charset=utf-8":                                                   true,
		`application/ld+json; profile="https://www.w3.org/ns/activitystreams"`:                       true,
		`application/ld+json; profile="https://example.org/a https://www.w3.org/ns/activitystreams"`: true,
		"application/ld+json":                                                                        false,
		"application/json":                                                                           false,
		"text/html; charset=utf-8":                                                                   false,
		"":                                                                                           false,
	} {
		if got := isActivityStreams(ct); got != want {
			t.Errorf("isActivityStreams(%q) = %v, want %v", ct, got, want)
		}
	}
}

func TestParseAcct(t *testing.T) {
	cases := []struct {
		in, user, domain string
		ok               bool
	}{
		{"acct:alice@example.org", "alice", "example.org", true},
		{"acct:Alice@EXAMPLE.org", "Alice", "example.org", true},
		{"acct:alice@例え.jp", "alice", "xn--r8jz45g.jp", true},
		{"acct:a@b@example.org", "a@b", "example.org", true},
		{"https://example.org/users/alice", "", "", false},
		{"acct:@example.org", "", "", false},
		{"acct:alice@", "", "", false},
	}
	for _, c := range cases {
		user, domain, ok := parseAcct(c.in)
		if user != c.user || domain != c.domain || ok != c.ok {
			t.Errorf("parseAcct(%q) = %q, %q, %v; want %q, %q, %v", c.in, user, domain, ok, c.user, c.domain, c.ok)
		}
	}
}
