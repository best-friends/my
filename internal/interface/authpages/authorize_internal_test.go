// SPDX-License-Identifier: AGPL-3.0-only

package authpages

import (
	"testing"

	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/i18n"
)

// TestScopeDescriptions checks that the consent screen words every scope the server knows,
// in every locale, instead of showing the scope token.
func TestScopeDescriptions(t *testing.T) {
	messages, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"en", "ja"} {
		l := messages.Localizer(locale)
		for _, s := range identity.KnownScopes() {
			if got := scopeDescription(l, s); got == s || got == "" {
				t.Errorf("%s: scope %s has no description (%q)", locale, s, got)
			}
		}
	}
}
