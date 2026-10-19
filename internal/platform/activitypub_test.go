// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/identity"
	domcontent "github.com/best-friends/my/internal/domain/content"
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
		Software          struct{ Version, Repository string }
		OpenRegistrations bool
		Usage             struct{ Users struct{ Total int } }
	}
	if err := json.Unmarshal([]byte(body), &ni); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("nodeinfo = %d %s (%v)", resp.StatusCode, body, err)
	}
	// The default registration mode, approval, lets anyone sign up for a moderator's approval.
	if ni.Usage.Users.Total != 1 || ni.Software.Version != "dev" || ni.Software.Repository != config.DefaultSourceURL || !ni.OpenRegistrations {
		t.Errorf("nodeinfo = %+v, want 1 user (not the instance actor), version dev, the default source URL, open registrations", ni)
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

// TestFederationObjects checks that the composition root serves the objects and collections of
// local statuses on the real database adapter: anyone gets public and unlisted statuses, nobody
// gets the others without a signature that this server can verify, a deleted status is gone, and
// the outbox lists only public and unlisted statuses.
func TestFederationObjects(t *testing.T) {
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	ids := map[domcontent.Visibility]string{}
	var accountID, deletedID string
	s := start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		ctx := t.Context()
		reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
			Username: "alice", Password: "correct horse battery staple", DisplayName: "Alice",
		})
		if err != nil {
			t.Fatalf("RegisterLocalUser: %v", err)
		}
		accountID = reg.AccountID
		post := func(v domcontent.Visibility) string {
			p, err := a.Content.Create(ctx, appcontent.CreateInput{
				AuthorAccountID: accountID, TextInput: appcontent.TextInput{Text: "a " + string(v) + " post"}, Visibility: v,
			})
			if err != nil {
				t.Fatalf("Create %s: %v", v, err)
			}
			return p.Status.ID
		}
		for _, v := range []domcontent.Visibility{
			domcontent.VisibilityPublic, domcontent.VisibilityLocal, domcontent.VisibilityUnlisted,
			domcontent.VisibilityPrivate, domcontent.VisibilityDirect,
		} {
			ids[v] = post(v)
		}
		deletedID = post(domcontent.VisibilityPublic)
		if _, err := a.Content.Delete(ctx, accountID, deletedID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
	})
	ap := map[string]string{"Accept": "application/activity+json"}
	// A signature whose key this server cannot resolve makes the request anonymous.
	signed := map[string]string{
		"Accept":    "application/activity+json",
		"Date":      time.Now().UTC().Format(http.TimeFormat),
		"Signature": `keyId="https://remote.example/users/bob#main-key",algorithm="rsa-sha256",headers="(request-target) host date",signature="AAAA"`,
	}
	for v, id := range ids {
		want := http.StatusNotFound
		if v == domcontent.VisibilityPublic || v == domcontent.VisibilityUnlisted {
			want = http.StatusOK
		}
		for _, h := range []map[string]string{ap, signed} {
			for _, path := range []string{"/ap/objects/" + id, "/ap/objects/" + id + "/activity"} {
				if resp, body := s.do(t, http.MethodGet, path, h); resp.StatusCode != want {
					t.Errorf("GET %s (%s) = %d %s, want %d", path, v, resp.StatusCode, body, want)
				}
			}
		}
	}
	resp, body := s.do(t, http.MethodGet, "/ap/objects/"+ids[domcontent.VisibilityPublic], ap)
	var note struct{ ID, Type, AttributedTo string }
	if err := json.Unmarshal([]byte(body), &note); err != nil || note.Type != "Note" ||
		note.AttributedTo != "https://"+cfg.Host+"/ap/actors/"+accountID || resp.Header.Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("public Note = %d %v %s (%v)", resp.StatusCode, resp.Header, body, err)
	}
	if resp, body := s.do(t, http.MethodGet, "/ap/objects/"+deletedID, ap); resp.StatusCode != http.StatusGone || !strings.Contains(body, `"type":"Tombstone"`) {
		t.Errorf("deleted status = %d %s, want 410 with a Tombstone", resp.StatusCode, body)
	}

	resp, body = s.do(t, http.MethodGet, "/ap/actors/"+accountID+"/outbox?page=true", ap)
	var page struct {
		OrderedItems []struct {
			Type   string
			Object struct{ ID string }
		}
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("outbox page = %d %s (%v)", resp.StatusCode, body, err)
	}
	var listed []string
	for _, it := range page.OrderedItems {
		listed = append(listed, it.Object.ID)
	}
	want := []string{
		"https://" + cfg.Host + "/ap/objects/" + ids[domcontent.VisibilityUnlisted],
		"https://" + cfg.Host + "/ap/objects/" + ids[domcontent.VisibilityPublic],
	}
	if !slices.Equal(listed, want) {
		t.Errorf("outbox lists %v, want the unlisted and public statuses %v", listed, want)
	}
	if _, body := s.do(t, http.MethodGet, "/ap/actors/"+accountID+"/outbox", ap); !strings.Contains(body, `"totalItems":5`) {
		t.Errorf("outbox = %s, want the 5 statuses that are not deleted", body)
	}
}
