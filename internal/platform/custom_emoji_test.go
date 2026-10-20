// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/best-friends/my/internal/app/kernel"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/moderation"
	"github.com/best-friends/my/internal/interface/clientapi"
	"github.com/best-friends/my/internal/interface/clientapi/apitest"
	"github.com/best-friends/my/internal/platform"
	"github.com/best-friends/my/pkg/testkit"
)

// fixtureNet is a kernel.HTTPClient that serves documents by URL and answers WebFinger for the
// handles of the actors it serves.
type fixtureNet struct {
	mu   sync.Mutex
	docs map[string][]byte
	// selves are the actor ids by acct:username@host.
	selves map[string]string
}

func (n *fixtureNet) Do(_ context.Context, req *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	respond := func(status int, contentType string, body []byte) (*kernel.HTTPResponse, error) {
		h := http.Header{}
		if contentType != "" {
			h.Set("Content-Type", contentType)
		}
		return &kernel.HTTPResponse{StatusCode: status, Header: h, URL: req.URL, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		return respond(http.StatusNotFound, "", nil)
	}
	if u.Path == "/.well-known/webfinger" {
		resource := u.Query().Get("resource")
		self, ok := n.selves[resource]
		if !ok {
			return respond(http.StatusNotFound, "", nil)
		}
		jrd, _ := json.Marshal(map[string]any{
			"subject": resource,
			"links":   []map[string]string{{"rel": "self", "type": "application/activity+json", "href": self}},
		})
		return respond(http.StatusOK, "application/jrd+json", jrd)
	}
	if doc, ok := n.docs[req.URL]; ok {
		return respond(http.StatusOK, "application/activity+json", doc)
	}
	return respond(http.StatusNotFound, "", nil)
}

// serveActor serves an actor fixture at id, the attributedTo of a note fixture, in place of its
// own id.
func (n *fixtureNet) serveActor(t *testing.T, doc []byte, id string) {
	t.Helper()
	var a struct {
		ID                string `json:"id"`
		PreferredUsername string `json:"preferredUsername"`
	}
	if err := json.Unmarshal(doc, &a); err != nil {
		t.Fatal(err)
	}
	doc = []byte(strings.ReplaceAll(string(doc), a.ID, id))
	u, _ := url.Parse(id)
	n.docs[id] = doc
	n.selves["acct:"+a.PreferredUsername+"@"+u.Host] = id
}

// serveNote serves the note of a fixture (the object of a Create, or the note itself) at its id
// and returns the id and the note's author.
func (n *fixtureNet) serveNote(t *testing.T, doc []byte) (id, author string) {
	t.Helper()
	var env struct {
		Type   string          `json:"type"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(doc, &env); err != nil {
		t.Fatal(err)
	}
	if env.Type == "Create" {
		doc = env.Object
	}
	var note struct {
		ID           string `json:"id"`
		AttributedTo string `json:"attributedTo"`
	}
	if err := json.Unmarshal(doc, &note); err != nil {
		t.Fatal(err)
	}
	n.docs[note.ID] = doc
	return note.ID, note.AttributedTo
}

func federationFixture(t *testing.T, path string) []byte {
	t.Helper()
	fixtures, err := testkit.FederationFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		if f.Path() == path {
			return f.JSON
		}
	}
	t.Fatalf("no fixture %s", path)
	return nil
}

// TestClientAPIPresentsRemoteCustomEmoji resolves the notes with custom emoji of Mastodon,
// Misskey, and Pleroma / Akkoma, written by actors with custom emoji in their profiles, and checks
// that the client API returns the emoji of the status and of its account behind the media proxy
// (ADR-0051 §4), and none of a domain whose media the instance rejects (FR-702).
func TestClientAPIPresentsRemoteCustomEmoji(t *testing.T) {
	t.Parallel()
	network := &fixtureNet{docs: map[string][]byte{}, selves: map[string]string{}}
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true), "MY_QUEUE": "inline"})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler), HTTPClient: network})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	h, spec := app.Handler(), apitest.Load(t)
	proxy := "https://" + cfg.Host + "/media/proxy/"

	cases := []struct {
		note, actor   string
		statusEmojis  []string
		accountEmojis []string
	}{
		{"mastodon/4.7/create-note-emoji", "mastodon/4.8/actor-emoji", []string{"blobcat"}, []string{"blobcat"}},
		{"misskey/2026.10/create-note-emoji", "misskey/2025.4/actor-emoji", []string{"blobcat"}, []string{"blobcat"}},
		{"akkoma/circa-2021/note-emoji-summary", "akkoma/3.20/actor-emoji", []string{"joker_disapprove", "joker_smile"}, []string{"blobcat"}},
	}
	get := func(t *testing.T, statusID string) clientapi.Status {
		t.Helper()
		res := spec.Serve(t, h, httptest.NewRequest(http.MethodGet, clientapi.BasePath+"/statuses/"+statusID, nil))
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET status = %d", res.StatusCode)
		}
		var st clientapi.Status
		if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
			t.Fatal(err)
		}
		return st
	}
	shortcodes := func(t *testing.T, emojis []clientapi.CustomEmoji) []string {
		t.Helper()
		out := []string{}
		for _, e := range emojis {
			if !strings.HasPrefix(e.Url, proxy) || e.StaticUrl == "" {
				t.Errorf("emoji %+v is not behind the media proxy", e)
			}
			out = append(out, e.Shortcode)
		}
		return out
	}
	ids := map[string]string{}
	for _, c := range cases {
		t.Run(c.note, func(t *testing.T) {
			note, author := network.serveNote(t, federationFixture(t, c.note))
			network.serveActor(t, federationFixture(t, c.actor), author)
			resolved, err := app.Resolver.Resolve(t.Context(), note)
			if err != nil || resolved.Status == nil {
				t.Fatalf("Resolve(%s) = %+v, %v", note, resolved, err)
			}
			ids[c.note] = resolved.Status.ID
			st := get(t, resolved.Status.ID)
			if got := shortcodes(t, st.Emojis); strings.Join(got, ",") != strings.Join(c.statusEmojis, ",") {
				t.Errorf("status emojis = %v, want %v", got, c.statusEmojis)
			}
			if got := shortcodes(t, st.Account.Emojis); strings.Join(got, ",") != strings.Join(c.accountEmojis, ",") {
				t.Errorf("account emojis = %v, want %v", got, c.accountEmojis)
			}
		})
	}

	// The instance rejects the media of misskey.example: its emoji are no longer listed, and the
	// others still are.
	if _, err := app.Moderation.IssueDomainAction(t.Context(), appmoderation.DomainActionInput{Domain: "misskey.example", Kind: moderation.DomainRejectMedia}); err != nil {
		t.Fatal(err)
	}
	if st := get(t, ids["misskey/2026.10/create-note-emoji"]); len(st.Emojis) != 0 || len(st.Account.Emojis) != 0 {
		t.Errorf("emojis of a rejected domain = %+v, %+v; want none", st.Emojis, st.Account.Emojis)
	}
	if st := get(t, ids["mastodon/4.7/create-note-emoji"]); len(st.Emojis) != 1 || len(st.Account.Emojis) != 1 {
		t.Errorf("emojis of another domain = %+v, %+v; want them", st.Emojis, st.Account.Emojis)
	}
}
