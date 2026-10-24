// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/app/identity"
	domain "github.com/mycroblog/my/internal/domain/identity"
)

// clientRepo serves one client; the other Repository methods are never reached.
type clientRepo struct {
	identity.Repository
	client domain.OAuthClient
	uris   []string
}

func (r clientRepo) FindOAuthClient(context.Context, string) (domain.OAuthClient, []string, error) {
	return r.client, r.uris, nil
}

func (r clientRepo) FindOAuthClientState(context.Context, string) (domain.OAuthClientState, error) {
	return domain.OAuthClientState{}, nil
}

func TestValidateAuthorizationRedirectURI(t *testing.T) {
	verifier := strings.Repeat("v", 43)
	svc := identity.NewService(identity.Deps{Repository: clientRepo{
		client: domain.OAuthClient{ID: "c1", ClientID: "app", Scope: "read write"},
		uris:   []string{"http://127.0.0.1/cb", "https://example.com/cb"},
	}})
	for _, c := range []struct {
		name, redirect string
		want           string // the redirect of the authorization; "" = rejected
	}{
		{"loopback exact", "http://127.0.0.1/cb", "http://127.0.0.1/cb"},
		{"loopback with any port", "http://127.0.0.1:53124/cb", "http://127.0.0.1:53124/cb"},
		{"loopback other path", "http://127.0.0.1:53124/other", ""},
		{"loopback other host", "http://[::1]:53124/cb", ""},
		{"loopback localhost", "http://localhost:53124/cb", ""},
		{"https exact", "https://example.com/cb", "https://example.com/cb"},
		{"https with a port", "https://example.com:8443/cb", ""},
		{"empty", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := svc.ValidateAuthorization(t.Context(), identity.AuthorizationRequest{
				ResponseType: "code", ClientID: "app", RedirectURI: c.redirect,
				CodeChallenge: domain.S256Challenge(verifier), CodeChallengeMethod: domain.CodeChallengeMethodS256,
			})
			if c.want == "" {
				if !errors.Is(err, identity.ErrInvalidRedirectURI) {
					t.Fatalf("err = %v, want ErrInvalidRedirectURI", err)
				}
				return
			}
			if err != nil || a.RedirectURI != c.want {
				t.Fatalf("ValidateAuthorization = %q, %v; want redirect %q", a.RedirectURI, err, c.want)
			}
		})
	}
}
