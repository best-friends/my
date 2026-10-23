// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"reflect"
	"testing"
)

func TestObfuscateDomain(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"bad.example", "bad.*****le"},
		{"a.b", "a.*"},
		{"spam.example.com", "spam.*******.com"},
		{"xn--r8jz45g.jp", "xn--*******.jp"},
		{"x", "x"},
		{"", ""},
	} {
		if got := ObfuscateDomain(tc.in); got != tc.want {
			t.Errorf("ObfuscateDomain(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPublicDomainAction(t *testing.T) {
	a := DomainAction{ID: testID(20), Domain: "bad.example", Kind: DomainSuspend, PublicComment: "spam", PrivateNote: "see #12"}
	const digest = "86bbe8ffb912a153c9a8b396246aeeae079cd324af4dd947a2bf59a698eabc62" // printf bad.example | sha256sum
	want := PublicDomainAction{ID: testID(20), Domain: "bad.example", Digest: digest, Kind: DomainSuspend, Comment: "spam"}
	if got := a.Public(); !reflect.DeepEqual(got, want) {
		t.Errorf("Public() = %+v, want %+v", got, want)
	}
	a.Obfuscate = true
	if got := a.Public(); got.Domain != "bad.*****le" || got.Digest != digest {
		t.Errorf("Public() of an obfuscated action = %+v, want the domain obfuscated and the digest of the domain", got)
	}
	if got := PublicDomainActionKinds(); !reflect.DeepEqual(got, []DomainActionKind{DomainLimit, DomainSuspend}) {
		t.Errorf("PublicDomainActionKinds() = %v", got)
	}
}
