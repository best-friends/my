// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"testing"
	"time"
)

func TestNormalizeLinkURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"HTTPS://Example.COM:443/Path?b=2&a=1#frag": "https://example.com/Path?b=2&a=1",
		"http://example.com:80":                     "http://example.com/",
		"https://user@example.com:8443/":            "https://example.com:8443/",
		"https://[::1]:443/x":                       "https://[::1]/x",
		"ftp://example.com/":                        "",
		"https:///nohost":                           "",
		"mailto:a@example.com":                      "",
		"/relative":                                 "",
	}
	for in, want := range cases {
		got, ok := NormalizeLinkURL(in)
		if got != want || ok != (want != "") {
			t.Errorf("NormalizeLinkURL(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
}

func TestLinkCardStale(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	c := LinkCard{FetchedAt: at}
	if c.Stale(at.Add(LinkCardRefetchAfter - time.Millisecond)) {
		t.Error("stale before 14 days")
	}
	if !c.Stale(at.Add(LinkCardRefetchAfter)) {
		t.Error("fresh after 14 days")
	}
}
