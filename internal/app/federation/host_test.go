// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/domain/federation"
)

func TestCanonicalHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"例え.jp", "xn--r8jz45g.jp"},
		{"XN--R8JZ45G.JP", "xn--r8jz45g.jp"},
		{"例え.JP.", "xn--r8jz45g.jp"},
		{"Ｅｘａｍｐｌｅ．com", "example.com"},
		{"Bücher.example", "xn--bcher-kva.example"},
		{"faß.de", "xn--fa-hia.de"}, // non-transitional: ß is kept, not mapped to "ss"
		{"r3---sn-abc.example", "r3---sn-abc.example"},
		{"_dmarc.example", "_dmarc.example"},
		{"192.0.2.1", "192.0.2.1"},
		{"2001:DB8::1", "2001:db8::1"},
	}
	for _, c := range cases {
		got, err := appfederation.CanonicalHost(c.in)
		if err != nil || got != c.want {
			t.Errorf("CanonicalHost(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestCanonicalHostInvalid(t *testing.T) {
	for _, in := range []string{
		"",
		".",
		"a..example",
		"xn--zz.example",                // undecodable punycode
		"xn--a.example",                 // decodes to a disallowed code point
		"exa mple.example",              // space
		"a*b.example",                   // disallowed ASCII
		"a\u200db.example",              // ZWJ outside a ContextJ context
		"אa.example",                    // Bidi rule: RTL label ending in an L character
		"a⒈.example",                    // disallowed (STD3 mapping of ⒈ contains ".")
		string(make([]byte, 64)) + ".x", // NUL bytes
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example", // 64-octet label
	} {
		if got, err := appfederation.CanonicalHost(in); !errors.Is(err, appfederation.ErrInvalidHost) {
			t.Errorf("CanonicalHost(%q) = %q, %v; want ErrInvalidHost", in, got, err)
		}
	}
}

func TestCanonicalIRI(t *testing.T) {
	cases := []struct {
		in   string
		want federation.IRI
	}{
		{"https://例え.jp/users/alice", "https://xn--r8jz45g.jp/users/alice"},
		{"https://%E4%BE%8B%E3%81%88.jp/users/alice", "https://xn--r8jz45g.jp/users/alice"},
		{"HTTPS://XN--R8JZ45G.JP:8443/users/alice#main-key", "https://xn--r8jz45g.jp:8443/users/alice#main-key"},
		{"https://[2001:DB8::1]/x", "https://[2001:db8::1]/x"},
	}
	for _, c := range cases {
		got, err := appfederation.CanonicalIRI(c.in)
		if err != nil || got != c.want {
			t.Errorf("CanonicalIRI(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"https://xn--zz.jp/x", "https://a..jp/x", "https://例え.jp\u200d/x"} {
		if got, err := appfederation.CanonicalIRI(in); !errors.Is(err, federation.ErrInvalidIRI) || !errors.Is(err, appfederation.ErrInvalidHost) {
			t.Errorf("CanonicalIRI(%q) = %q, %v; want ErrInvalidIRI and ErrInvalidHost", in, got, err)
		}
	}
}

// TestSameHostSpellings checks that the U-label and A-label spellings of one host are the same
// origin and fall under the same domain-level moderation action.
func TestSameHostSpellings(t *testing.T) {
	cases := []struct {
		name   string
		a, b   string // IRIs
		domain string // a domain-level action, as a moderator types it
	}{
		{"U-label and A-label", "https://例え.jp/users/alice", "https://xn--r8jz45g.jp/users/alice/statuses/1", "例え.jp"},
		{"A-label action", "https://例え.jp/users/alice", "https://XN--R8JZ45G.jp/inbox", "xn--r8jz45g.jp"},
		{"subdomain", "https://social.例え.jp/users/alice", "https://social.xn--r8jz45g.jp/users/alice", "例え.JP."},
		{"full-width", "https://ｅｘａｍｐｌｅ.com/users/bob", "https://example.com/users/bob", "Example.com"},
		{"German", "https://bücher.example/u/c", "https://xn--bcher-kva.example/u/c", "BÜCHER.example"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, errA := appfederation.CanonicalIRI(c.a)
			b, errB := appfederation.CanonicalIRI(c.b)
			if errA != nil || errB != nil {
				t.Fatalf("CanonicalIRI: %v, %v", errA, errB)
			}
			if !federation.SameOrigin(a, b) {
				t.Errorf("SameOrigin(%q, %q) = false, want true", a, b)
			}
			domain, err := appfederation.CanonicalHost(c.domain)
			if err != nil {
				t.Fatalf("CanonicalHost(%q): %v", c.domain, err)
			}
			for _, i := range []federation.IRI{a, b} {
				if !slices.Contains(federation.DomainAndParents(i.Host()), domain) {
					t.Errorf("DomainAndParents(%q) = %q, want it to contain %q", i.Host(), federation.DomainAndParents(i.Host()), domain)
				}
			}
		})
	}
}

// TestNormalizeHostSpellings checks the origin rules on a received document whose IRIs spell one
// host differently: the embedded object is trusted, and id and attributedTo have the same origin.
func TestNormalizeHostSpellings(t *testing.T) {
	doc := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type": "Create",
		"id": "https://例え.jp/users/alice/statuses/1/activity",
		"actor": "https://xn--r8jz45g.jp/users/alice",
		"to": ["https://www.w3.org/ns/activitystreams#Public"],
		"object": {
			"type": "Note",
			"id": "https://XN--R8JZ45G.JP/users/alice/statuses/1",
			"attributedTo": "https://例え.jp/users/alice",
			"content": "hi",
			"tag": [{"type": "Mention", "href": "https://ｅｘａｍｐｌｅ.com/users/bob"}, {"type": "Mention", "href": "https://xn--zz.jp/u"}]
		}
	}`
	d, err := appfederation.Normalize([]byte(doc), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	a := d.Activity
	if a == nil || a.Actor != "https://xn--r8jz45g.jp/users/alice" {
		t.Fatalf("Activity = %+v", a)
	}
	o := a.Object()
	if o.Trusted() == nil || o.Trusted().Object == nil {
		t.Fatalf("embedded object not trusted: %+v", o)
	}
	note := o.Trusted().Object
	if !federation.SameOrigin(note.ID, note.AttributedTo) || !federation.SameOrigin(a.ID, a.Actor) {
		t.Errorf("origins differ: id %q, attributedTo %q, activity %q, actor %q", note.ID, note.AttributedTo, a.ID, a.Actor)
	}
	if len(note.Mentions) != 1 || note.Mentions[0].Href != "https://example.com/users/bob" {
		t.Errorf("Mentions = %+v, want only the valid host, canonical", note.Mentions)
	}
}

func TestDisplayHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"xn--r8jz45g.jp", "例え.jp"},
		{"xn--eckwd4c7c.xn--zckzah", "ドメイン.テスト"},
		{"xn--bcher-kva.example", "bücher.example"},
		{"xn--e1afmkfd.xn--p1ai", "пример.рф"},
		{"example.com", "example.com"},
		{"xn--mxacd.example", "αβγ.example"},
		{"xn--abc-s08fl0dtz6h.example", "日本語abc.example"},
		{"xn--abc-lt8lk11n.example", "한국abc.example"},
		{"xn--5ek909fo6p.example", "中文ㄅ.example"},
		{"xn--pple-43d.com", "xn--pple-43d.com"},                                   // Cyrillic "а" with Latin "pple"
		{"xn--v8j0cwa6g1527epp4at5w.example", "xn--v8j0cwa6g1527epp4at5w.example"}, // Hangul with Hiragana
		{"social.xn--pple-43d.com", "social.xn--pple-43d.com"},                     // one mixed label shows the whole host as A-labels
		{"xn--zz.example", "xn--zz.example"},                                       // undecodable
	}
	for _, c := range cases {
		if got := appfederation.DisplayHost(c.in); got != c.want {
			t.Errorf("DisplayHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
