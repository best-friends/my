// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
)

func TestNormalizePasskeyName(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		err      bool
	}{
		{in: "  MacBook  ", want: "MacBook"},
		{in: "鍵 🔑", want: "鍵 🔑"},
		{in: strings.Repeat("あ", identity.MaxPasskeyNameLength), want: strings.Repeat("あ", identity.MaxPasskeyNameLength)},
		{in: strings.Repeat("あ", identity.MaxPasskeyNameLength+1), err: true},
		{in: "   ", err: true},
		{in: "", err: true},
		{in: "a\nb", err: true},
		{in: "a\x00b", err: true},
		{in: "\xff", err: true},
	} {
		got, err := identity.NormalizePasskeyName(tc.in)
		if tc.err {
			if !errors.Is(err, identity.ErrInvalidPasskeyName) {
				t.Errorf("NormalizePasskeyName(%q) = %q, %v; want ErrInvalidPasskeyName", tc.in, got, err)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("NormalizePasskeyName(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}
