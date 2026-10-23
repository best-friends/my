// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mycroblog/my/internal/adapter/httpsig"
	appaccount "github.com/mycroblog/my/internal/app/account"
	appfederation "github.com/mycroblog/my/internal/app/federation"
	"github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/app/kernel"
	appnotification "github.com/mycroblog/my/internal/app/notification"
	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/internal/domain/notification"
	"github.com/mycroblog/my/internal/platform"
)

// TestInboxIngestsRemoteStatuses checks that the composition root registers the handlers of
// remote posts on the inbox processor with the real sanitizer: a signed Create that mentions a
// local account is stored with its HTML sanitized and notifies the account, and its Delete
// removes it.
func TestInboxIngestsRemoteStatuses(t *testing.T) {
	t.Parallel()
	const actor = "https://remote.example/users/bob"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true), "MY_QUEUE": "inline"})
	var app *platform.App
	var aliceID string
	s := start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		ctx := t.Context()
		app = a
		reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
			Username: "alice", Password: "correct horse battery staple", DisplayName: "Alice",
		})
		if err != nil {
			t.Fatalf("RegisterLocalUser: %v", err)
		}
		aliceID = reg.AccountID
		err = a.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			_, err := a.Accounts.UpsertRemoteActor(ctx, tx, appaccount.RemoteActorInput{
				URI: actor, Domain: "remote.example", Type: "Person", PreferredUsername: "bob",
				InboxURL: actor + "/inbox", FollowersURL: actor + "/followers",
				Keys:     []appaccount.RemoteKeyInput{{KeyID: actor + "#main-key", PublicKey: &key.PublicKey}},
				Document: []byte(`{"id":"` + actor + `"}`),
			})
			return err
		})
		if err != nil {
			t.Fatalf("UpsertRemoteActor: %v", err)
		}
	})
	aliceIRI := "https://" + cfg.Host + "/ap/actors/" + aliceID
	note := actor + "/statuses/1"
	post := func(t *testing.T, activity map[string]any) {
		t.Helper()
		activity["@context"] = "https://www.w3.org/ns/activitystreams"
		body, err := json.Marshal(activity)
		if err != nil {
			t.Fatal(err)
		}
		req := &kernel.HTTPRequest{
			Method: http.MethodPost, URL: "https://" + cfg.Host + "/ap/inbox", Body: body,
			Header: http.Header{"Date": {time.Now().UTC().Format(http.TimeFormat)}, "Host": {cfg.Host}},
		}
		err = httpsig.New().Sign(req, appfederation.SigningKey{KeyID: actor + "#main-key", PrivateKey: key}, appfederation.SignOptions{
			Scheme: appfederation.SchemeCavage, Algorithm: appfederation.AlgRSAPKCS1SHA256,
			Components: []string{"(request-target)", "host", "date", "digest"},
		})
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		hr, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.url+"/ap/inbox", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range req.Header {
			if k != "Host" {
				hr.Header[k] = v
			}
		}
		hr.Header.Set("Content-Type", "application/activity+json")
		resp, err := http.DefaultTransport.RoundTrip(hr)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("POST %s = %d, want 202", activity["id"], resp.StatusCode)
		}
	}

	post(t, map[string]any{
		"id": note + "/activity", "type": "Create", "actor": actor,
		"object": map[string]any{
			"id": note, "type": "Note", "attributedTo": actor, "published": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
			"to": []string{"https://www.w3.org/ns/activitystreams#Public"}, "cc": []string{actor + "/followers", aliceIRI},
			"content": `<p>hi <span class="h-card"><a href="` + aliceIRI + `" class="u-url mention">@alice</a></span><script>alert(1)</script><img src="x"></p>`,
			"tag":     []any{map[string]any{"type": "Mention", "href": aliceIRI, "name": "@alice@" + cfg.Host}},
		},
	})
	// The inline queue processes the activity before the inbox answers.
	st, err := app.Content.FindStatusByURI(t.Context(), note)
	if err != nil {
		t.Fatalf("FindStatusByURI(%s) = %v", note, err)
	}
	p, err := app.Content.Find(t.Context(), st.ID)
	if err != nil {
		t.Fatalf("Find = %v", err)
	}
	if html := p.Revision.Bodies[0].ContentHTML; strings.Contains(html, "script") || strings.Contains(html, "<img") || !strings.Contains(html, `class="u-url mention"`) {
		t.Errorf("stored HTML = %q, want it sanitized with the mention kept", html)
	}
	ns, err := app.Notifications.List(t.Context(), aliceID, appnotification.Query{})
	if err != nil || len(ns) != 1 || ns[0].Type != notification.TypeMention || ns[0].StatusID != st.ID {
		t.Errorf("notifications of alice = %+v, %v; want the mention", ns, err)
	}

	post(t, map[string]any{"id": note + "#delete", "type": "Delete", "actor": actor, "object": map[string]any{"id": note, "type": "Tombstone"}})
	if _, err := app.Content.Find(t.Context(), st.ID); err == nil {
		t.Errorf("the post is still there after its Delete")
	}
}
