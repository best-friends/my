// SPDX-License-Identifier: AGPL-3.0-only

package testkit_test

import (
	"regexp"
	"testing"

	"github.com/mycroblog/my/pkg/testkit"
)

var canonicalUUIDv7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNamedID(t *testing.T) {
	a := testkit.NamedID("alice")
	if !canonicalUUIDv7.MatchString(a) {
		t.Fatalf("NamedID(alice) = %q, want a canonical UUIDv7", a)
	}
	if again := testkit.NamedID("alice"); again != a {
		t.Errorf("NamedID(alice) = %q then %q, want the same ID", a, again)
	}
	if b := testkit.NamedID("bob"); b == a || !canonicalUUIDv7.MatchString(b) {
		t.Errorf("NamedID(bob) = %q, want another canonical UUIDv7 than %q", b, a)
	}
}

func TestOrderedID(t *testing.T) {
	prev := ""
	for n := range uint32(300) {
		id := testkit.OrderedID(n)
		if !canonicalUUIDv7.MatchString(id) {
			t.Fatalf("OrderedID(%d) = %q, want a canonical UUIDv7", n, id)
		}
		if id <= prev {
			t.Fatalf("OrderedID(%d) = %q does not sort after %q", n, id, prev)
		}
		prev = id
	}
}
