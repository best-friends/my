// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"bytes"
	"crypto/ed25519"
	"math/big"
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

// encodeBase58 is the inverse of decodeBase58, for the test.
func encodeBase58(b []byte) string {
	n := new(big.Int).SetBytes(b)
	var out []byte
	radix, mod := big.NewInt(58), new(big.Int)
	for n.Sign() > 0 {
		n.DivMod(n, radix, mod)
		out = append([]byte{base58Alphabet[mod.Int64()]}, out...)
	}
	for _, c := range b {
		if c != 0 {
			break
		}
		out = append([]byte{'1'}, out...)
	}
	return string(out)
}

func TestParseMultikey(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	mb := "z" + encodeBase58(append(append([]byte{}, multicodecEd25519...), pub...))
	if mb[:4] != "z6Mk" {
		t.Errorf("multibase %s does not start with z6Mk", mb)
	}
	got, err := parseMultikey(mb)
	if err != nil || !bytes.Equal(got, pub) {
		t.Errorf("parseMultikey = %x, %v; want %x", got, err, pub)
	}
	for _, bad := range []string{"", "6Mk", "z0OIl", "z" + encodeBase58([]byte{0x12, 0x00, 1, 2, 3})} {
		if _, err := parseMultikey(bad); err == nil {
			t.Errorf("parseMultikey(%q) succeeded", bad)
		}
	}
	if b, err := decodeBase58("11" + encodeBase58([]byte{7})); err != nil || !bytes.Equal(b, []byte{0, 0, 7}) {
		t.Errorf("decodeBase58 leading zeros = %x, %v", b, err)
	}
}
