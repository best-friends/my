// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/platform"
	"github.com/best-friends/my/internal/platform/config"
)

// TestFederationEndpoints checks that the composition root serves the federation endpoints
// on the real database adapter: from a handle to the actor and its key, and NodeInfo.
func TestFederationEndpoints(t *testing.T) {
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	var accountID string
	s := start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		reg, err := a.Identity.RegisterLocalUser(t.Context(), identity.RegisterInput{
			Username: "alice", Password: "correct horse battery staple", DisplayName: "Alice",
		})
		if err != nil {
			t.Fatalf("RegisterLocalUser: %v", err)
		}
		accountID = reg.AccountID
		if _, err := a.Accounts.EnsureInstanceActor(t.Context()); err != nil {
			t.Fatalf("EnsureInstanceActor: %v", err)
		}
	})
	actorURI := "https://" + cfg.Host + "/ap/actors/" + accountID

	resp, body := s.do(t, http.MethodGet, "/.well-known/webfinger?resource=acct:alice@"+cfg.Host, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"href":"`+actorURI+`"`) {
		t.Errorf("WebFinger = %d %s, want a self link to %s", resp.StatusCode, body, actorURI)
	}

	resp, body = s.do(t, http.MethodGet, "/ap/actors/"+accountID, map[string]string{"Accept": "application/activity+json"})
	var actor struct {
		ID        string `json:"id"`
		PublicKey struct {
			ID  string `json:"id"`
			PEM string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal([]byte(body), &actor); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("actor = %d %s (%v)", resp.StatusCode, body, err)
	}
	if actor.ID != actorURI || actor.PublicKey.ID != actorURI+"#main-key" || !strings.HasPrefix(actor.PublicKey.PEM, "-----BEGIN PUBLIC KEY-----") {
		t.Errorf("actor = %+v, want id %s with its RSA main key", actor, actorURI)
	}
	if got := resp.Header.Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
		t.Errorf("actor Cross-Origin-Resource-Policy = %q, want cross-origin", got)
	}

	resp, body = s.do(t, http.MethodGet, "/nodeinfo/2.1", nil)
	var ni struct {
		Software struct{ Version, Repository string }
		Usage    struct{ Users struct{ Total int } }
	}
	if err := json.Unmarshal([]byte(body), &ni); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("nodeinfo = %d %s (%v)", resp.StatusCode, body, err)
	}
	if ni.Usage.Users.Total != 1 || ni.Software.Version != "dev" || ni.Software.Repository != config.DefaultSourceURL {
		t.Errorf("nodeinfo = %+v, want 1 user (not the instance actor), version dev, the default source URL", ni)
	}

	// A missing actor is the server's ordinary 404 problem.
	resp, body = s.do(t, http.MethodGet, "/ap/actors/0192f3a4-7b1c-7d2e-9f00-ffffffffffff", map[string]string{"Accept": "application/activity+json"})
	if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("missing actor = %d Cache-Control %q, want 404 no-store", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if p := decodeProblem(t, resp, body); p.Status != http.StatusNotFound {
		t.Errorf("missing actor problem = %+v", p)
	}
}
