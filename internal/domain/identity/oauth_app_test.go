// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
)

func TestValidateOAuthClientSettings(t *testing.T) {
	uris := []string{"https://app.example/cb"}
	got, err := identity.ValidateOAuthClientSettings(" Bot ", "https://app.example/", []string{"http://127.0.0.1/cb", "https://app.example/cb", "https://app.example/cb"}, "write read")
	if err != nil {
		t.Fatalf("ValidateOAuthClientSettings = %v", err)
	}
	if got.Name != "Bot" || got.Website != "https://app.example/" || got.Scope != "read write" ||
		!slices.Equal(got.RedirectURIs, []string{"http://127.0.0.1/cb", "https://app.example/cb"}) {
		t.Errorf("ValidateOAuthClientSettings = %+v, want it trimmed, sorted, and without duplicates", got)
	}
	tooMany := make([]string, identity.MaxOAuthClientRedirectURIs+1)
	for i := range tooMany {
		tooMany[i] = "https://app.example/cb" + strings.Repeat("x", i)
	}
	for _, tc := range []struct {
		name, appName, website, scope string
		uris                          []string
		want                          error
	}{
		{"empty name", "  ", "", "read", uris, identity.ErrInvalidOAuthClientName},
		{"long name", strings.Repeat("あ", identity.MaxOAuthClientNameLength+1), "", "read", uris, identity.ErrInvalidOAuthClientName},
		{"two lines", "a\nb", "", "read", uris, identity.ErrInvalidOAuthClientName},
		{"ftp website", "Bot", "ftp://app.example/", "read", uris, identity.ErrInvalidOAuthClientWebsite},
		{"relative website", "Bot", "/home", "read", uris, identity.ErrInvalidOAuthClientWebsite},
		{"website with a fragment", "Bot", "https://app.example/#x", "read", uris, identity.ErrInvalidOAuthClientWebsite},
		{"no redirect", "Bot", "", "read", nil, identity.ErrInvalidOAuthClientRedirectURIs},
		{"too many redirects", "Bot", "", "read", tooMany, identity.ErrInvalidOAuthClientRedirectURIs},
		{"localhost redirect", "Bot", "", "read", []string{"http://localhost:8080/cb"}, identity.ErrInvalidOAuthClientRedirectURIs},
		{"oob redirect", "Bot", "", "read", []string{"urn:ietf:wg:oauth:2.0:oob"}, identity.ErrInvalidOAuthClientRedirectURIs},
		{"empty scope", "Bot", "", "", uris, identity.ErrInvalidOAuthClientScope},
		{"unknown scope", "Bot", "", "read everything", uris, identity.ErrInvalidOAuthClientScope},
		{"admin scope", "Bot", "", "read admin", uris, identity.ErrInvalidOAuthClientScope},
		{"supporters scope", "Bot", "", "write:supporters", uris, identity.ErrInvalidOAuthClientScope},
	} {
		if _, err := identity.ValidateOAuthClientSettings(tc.appName, tc.website, tc.uris, tc.scope); !errors.Is(err, tc.want) {
			t.Errorf("%s: ValidateOAuthClientSettings = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestOAuthClientRevisionsAndOwners(t *testing.T) {
	c := identity.OAuthClient{
		ID: "c1", Name: "Bot", Website: "https://a.example/", Scope: "read", SecretHash: []byte{1},
		Registration: identity.ClientRegisteredByUser, OwnerUserID: "u1",
	}
	r := c.Revised(identity.OAuthClientRevision{Name: "New", Scope: "write"}).WithSecret(identity.OAuthClientSecretRotation{SecretHash: []byte{2}})
	if r.Name != "New" || r.Website != "" || r.Scope != "write" || r.SecretHash[0] != 2 || r.ID != "c1" {
		t.Errorf("revised client = %+v", r)
	}
	if !c.OwnedBy("u1") || c.OwnedBy("u2") || c.OwnedBy("") {
		t.Error("OwnedBy should hold for the owner only")
	}
	c.Registration = identity.ClientRegisteredDynamically
	if c.OwnedBy("u1") {
		t.Error("a dynamically registered client has no owner")
	}
}
