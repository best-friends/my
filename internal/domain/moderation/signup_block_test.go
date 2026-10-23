// SPDX-License-Identifier: AGPL-3.0-only

package moderation_test

import (
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mycroblog/my/internal/domain/moderation"
)

func validSignupBlock() moderation.SignupBlock {
	return moderation.SignupBlock{
		ID: "01890a5d-ac96-774b-bcce-b302099a8057", Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/24",
		Severity: moderation.SignupReject, CreatedAt: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
	}
}

func TestSignupBlockValidate(t *testing.T) {
	ok := []moderation.SignupBlock{validSignupBlock()}
	for _, v := range []struct {
		kind  moderation.SignupBlockKind
		value string
	}{
		{moderation.SignupBlockEmailDomain, "xn--r8jz45g.jp"},
		{moderation.SignupBlockEmailHash, moderation.EmailHash("a@example.com")},
		{moderation.SignupBlockIPRange, "2001:db8::/32"},
	} {
		b := validSignupBlock()
		b.Kind, b.Value = v.kind, v.value
		ok = append(ok, b)
	}
	for _, b := range ok {
		if err := b.Validate(); err != nil {
			t.Errorf("Validate(%s %s) = %v", b.Kind, b.Value, err)
		}
	}
	for name, edit := range map[string]func(*moderation.SignupBlock){
		"no ID":            func(b *moderation.SignupBlock) { b.ID = "" },
		"no time":          func(b *moderation.SignupBlock) { b.CreatedAt = time.Time{} },
		"unknown kind":     func(b *moderation.SignupBlock) { b.Kind = "email" },
		"unknown severity": func(b *moderation.SignupBlock) { b.Severity = "maybe" },
		"unmasked prefix":  func(b *moderation.SignupBlock) { b.Value = "192.0.2.1/24" },
		"bare address":     func(b *moderation.SignupBlock) { b.Value = "192.0.2.1" },
		"upper-case domain": func(b *moderation.SignupBlock) {
			b.Kind, b.Value = moderation.SignupBlockEmailDomain, "Spam.example"
		},
		"short hash": func(b *moderation.SignupBlock) { b.Kind, b.Value = moderation.SignupBlockEmailHash, "abcd" },
		"upper-case hash": func(b *moderation.SignupBlock) {
			b.Kind, b.Value = moderation.SignupBlockEmailHash, strings.ToUpper(moderation.EmailHash("a@example.com"))
		},
		"long comment": func(b *moderation.SignupBlock) { b.Comment = strings.Repeat("x", moderation.MaxSignupBlockComment+1) },
		"NUL comment":  func(b *moderation.SignupBlock) { b.Comment = "a\x00b" },
		"expires at its start": func(b *moderation.SignupBlock) {
			t := b.CreatedAt
			b.ExpiresAt = &t
		},
	} {
		b := validSignupBlock()
		edit(&b)
		if err := b.Validate(); !errors.Is(err, moderation.ErrInvalidSignupBlock) {
			t.Errorf("%s: Validate = %v, want ErrInvalidSignupBlock", name, err)
		}
	}
}

func TestSignupBlockMatches(t *testing.T) {
	domain := moderation.SignupBlock{Kind: moderation.SignupBlockEmailDomain, Value: "spam.example"}
	hash := moderation.SignupBlock{Kind: moderation.SignupBlockEmailHash, Value: moderation.EmailHash("a@example.com")}
	ip := moderation.SignupBlock{Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/24"}
	none := netip.Addr{}
	for _, c := range []struct {
		b      moderation.SignupBlock
		domain string
		hash   string
		ip     netip.Addr
		want   bool
	}{
		{domain, "spam.example", "", none, true},
		{domain, "mx.spam.example", "", none, true},
		{domain, "notspam.example", "", none, false},
		{domain, "", "", none, false},
		{hash, "", moderation.EmailHash("a@example.com"), none, true},
		{hash, "", "", none, false},
		{ip, "", "", netip.MustParseAddr("192.0.2.9"), true},
		{ip, "", "", netip.MustParseAddr("::ffff:192.0.2.9"), true},
		{ip, "", "", netip.MustParseAddr("192.0.3.9"), false},
		{ip, "", "", none, false},
	} {
		if got := c.b.Matches(c.domain, c.hash, c.ip); got != c.want {
			t.Errorf("%s %s Matches(%q, %q, %v) = %v, want %v", c.b.Kind, c.b.Value, c.domain, c.hash, c.ip, got, c.want)
		}
	}
}

func TestSignupDomainsAndEmailHash(t *testing.T) {
	if got, want := moderation.SignupDomains("mx.spam.example"), []string{"mx.spam.example", "spam.example", "example"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SignupDomains = %v, want %v", got, want)
	}
	if got := moderation.SignupDomains(""); got != nil {
		t.Errorf("SignupDomains(\"\") = %v, want nil", got)
	}
	h := moderation.EmailHash("alice@example.com")
	if len(h) != 64 || moderation.EmailHash("Alice+spam@Example.com") != h || moderation.EmailHash("alice2@example.com") == h {
		t.Errorf("EmailHash does not ignore exactly the case and the +tag: %s", h)
	}
	// The well-known SHA-256 of "alice@example.com".
	if h != "ff8d9819fc0e12bf0d24892e45987e249a28dce836a85cad60e28eaaa8c6d976" {
		t.Errorf("EmailHash(alice@example.com) = %s", h)
	}
}

func TestParseSignupBlockList(t *testing.T) {
	text := "\uFEFF# disposable domains\r\nspam.example\r\n\n  Mail.Example  \n# comment\n\"quoted.example\",2026-10-01,note\nplain.example, comment\n   \n,\n"
	want := []moderation.SignupBlockListEntry{
		{Line: 2, Value: "spam.example"}, {Line: 4, Value: "Mail.Example"}, {Line: 6, Value: "quoted.example"},
		{Line: 7, Value: "plain.example"}, {Line: 9, Value: ""},
	}
	if got := moderation.ParseSignupBlockList(text); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseSignupBlockList = %+v, want %+v", got, want)
	}
	if got := moderation.ParseSignupBlockList(""); got != nil {
		t.Errorf("ParseSignupBlockList(\"\") = %+v, want nil", got)
	}
}

func TestSignupBlockSeverityStronger(t *testing.T) {
	r, a := moderation.SignupReject, moderation.SignupRequireApproval
	for _, c := range []struct {
		s, t, want moderation.SignupBlockSeverity
	}{
		{"", "", ""}, {"", a, a}, {a, "", a}, {a, r, r}, {r, a, r}, {"", r, r},
	} {
		if got := c.s.Stronger(c.t); got != c.want {
			t.Errorf("%q.Stronger(%q) = %q, want %q", c.s, c.t, got, c.want)
		}
	}
}
