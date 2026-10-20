// SPDX-License-Identifier: AGPL-3.0-only

package activitypub_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appfederation "github.com/best-friends/my/internal/app/federation"
	appgraph "github.com/best-friends/my/internal/app/graph"
	apptimeline "github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/interface/activitypub"
)

const (
	noteStatusID = "0192f3b0-2c3d-7e4f-8a00-1122334455aa"
	bobID        = "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e80" // a remote account on remote.example
)

// noteWorld is the content, account, and graph modules behind appfederation.Objects with one
// public status of alice: a content warning, a mention of a remote account, a hashtag, and an
// image, the parts that the peers' create-note.json documents have.
type noteWorld struct {
	actors map[string]appaccount.Actor
	status appcontent.Status
	rev    appcontent.Revision
	parts  appcontent.RevisionParts
}

func newNoteWorld() *noteWorld {
	alice := appaccount.Actor{
		Account: account.Account{ID: aliceID, URI: account.ActorURI(host, aliceID), ActorType: account.ActorPerson},
		Local:   &account.LocalAccount{AccountID: aliceID, Username: "alice", UsernameNormalized: "alice"},
		Handle:  &account.Handle{AccountID: aliceID, Username: "alice", UsernameNormalized: "alice"},
	}
	bob := appaccount.Actor{
		Account: account.Account{ID: bobID, URI: "https://remote.example/users/bob", Domain: "remote.example", ActorType: account.ActorPerson},
		Handle:  &account.Handle{AccountID: bobID, Username: "bob", UsernameNormalized: "bob", Domain: "remote.example"},
	}
	published := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	body := `<p><span class="h-card"><a href="https://remote.example/@bob" class="u-url mention">@<span>bob</span></a></span> hello <a href="https://` + host + `/tags/fediverse" class="mention hashtag" rel="tag">#<span>fediverse</span></a></p>`
	return &noteWorld{
		actors: map[string]appaccount.Actor{aliceID: alice, bobID: bob},
		status: appcontent.Status{
			ID: noteStatusID, URI: appcontent.StatusURI(host, noteStatusID),
			AuthorAccountID: aliceID, IsLocal: true, Visibility: appcontent.VisibilityPublic, PublishedAt: published,
		},
		rev: appcontent.Revision{
			ID: noteStatusID + "-r", StatusID: noteStatusID, Seq: 1, SpoilerText: "a content warning", Language: "en", RevisedAt: published,
			Bodies: []content.Body{{Language: "en", ContentHTML: body}}, MentionedAccountIDs: []string{bobID},
		},
		parts: appcontent.RevisionParts{
			Mentions: []appaccount.Actor{bob},
			Tags:     []content.Tag{{ID: "t1", Name: "fediverse", NameNormalized: "fediverse"}},
			Attachments: []appcontent.MediaView{{
				Media: appcontent.Media{
					Result: &content.MediaProcessingResult{
						Outcome: content.MediaReady, MediaType: content.MediaTypeImage, Width: 1280, Height: 960, Blurhash: "UFHV6nWB2yk8pyo0adR*.7kCMdnjt7jtxuWB",
						Variants: []content.MediaVariant{{Kind: content.VariantFull, ContentType: "image/webp", Width: 1280, Height: 960}},
					},
					Focus: &content.MediaFocus{X: 0.5, Y: -0.25},
				},
				Description: "A cat on a sofa", URL: "https://files." + host + "/media/full.webp",
			}},
		},
	}
}

func (w *noteWorld) ObjectStatus(_ context.Context, id string) (appcontent.Status, appcontent.Revision, bool, error) {
	if id != w.status.ID {
		return appcontent.Status{}, appcontent.Revision{}, false, appcontent.ErrNotFound
	}
	return w.status, w.rev, false, nil
}

func (w *noteWorld) AuthorRestrictions(context.Context, string) (appcontent.AuthorRestrictions, error) {
	return appcontent.AuthorRestrictions{}, nil
}

func (w *noteWorld) PostCount(context.Context, string) (int64, error) { return 1, nil }

func (w *noteWorld) FindByID(_ context.Context, id string) (appaccount.Actor, error) {
	a, ok := w.actors[id]
	if !ok {
		return appaccount.Actor{}, appaccount.ErrNotFound
	}
	return a, nil
}

func (w *noteWorld) FindByURI(context.Context, string) (appaccount.Actor, error) {
	return appaccount.Actor{}, appaccount.ErrNotFound
}

func (w *noteWorld) AnyOnDomain(context.Context, string, []string) (bool, error) { return false, nil }

func (w *noteWorld) ActiveFollowerIDs(context.Context, string, string, int) ([]string, string, error) {
	return nil, "", nil
}

func (w *noteWorld) Relationship(_ context.Context, accountID, targetID string) (appgraph.Relationship, error) {
	return appgraph.Relationship{AccountID: accountID, TargetAccountID: targetID}, nil
}

func (w *noteWorld) Profile(context.Context, appcontent.Viewer, string, apptimeline.ProfileFilter, apptimeline.Query) (apptimeline.Page, error) {
	return apptimeline.Page{}, nil
}

func (w *noteWorld) Views(context.Context, appcontent.Viewer, []string) ([]appcontent.StatusView, error) {
	return nil, nil
}

func (w *noteWorld) Pinned(context.Context, appcontent.Viewer, string) ([]appcontent.StatusView, error) {
	return nil, nil
}

func (w *noteWorld) Parts(context.Context, appcontent.Revision) (appcontent.RevisionParts, error) {
	return w.parts, nil
}

// noteRequired lists the properties of a Create and its Note that each implementation reads
// when it imports a remote status (Mastodon: ActivityPub::Activity::Create and
// ActivityPub::Parser::StatusParser / MediaAttachmentParser; GoToSocial: internal/ap/extract.go;
// Misskey: ApNoteService.createNote).
var noteRequired = map[string][]string{
	"mastodon": {
		"@context", "id", "type", "actor", "published", "to", "cc",
		"object.id", "object.type", "object.attributedTo", "object.url", "object.published", "object.to", "object.cc",
		"object.summary", "object.sensitive", "object.content", "object.contentMap",
		"object.attachment[].type", "object.attachment[].mediaType", "object.attachment[].url", "object.attachment[].name",
		"object.attachment[].blurhash", "object.attachment[].focalPoint", "object.attachment[].width", "object.attachment[].height",
		"object.tag[].type", "object.tag[].href", "object.tag[].name",
	},
	"gotosocial": {
		"@context", "id", "type", "actor", "to", "cc",
		"object.id", "object.type", "object.attributedTo", "object.url", "object.published", "object.to", "object.cc",
		"object.summary", "object.sensitive", "object.content", "object.contentMap",
		"object.attachment[].mediaType", "object.attachment[].url", "object.attachment[].name",
		"object.attachment[].blurhash", "object.attachment[].focalPoint",
		"object.tag[].type", "object.tag[].href", "object.tag[].name",
	},
	"misskey": {
		"@context", "id", "type", "actor", "published", "to", "cc",
		"object.id", "object.type", "object.attributedTo", "object.published", "object.to", "object.cc",
		"object.summary", "object.sensitive", "object.content",
		"object.attachment[].type", "object.attachment[].mediaType", "object.attachment[].url", "object.attachment[].name",
		"object.tag[].type", "object.tag[].href", "object.tag[].name",
	},
}

// TestCreateNoteMatchesPeers checks the Create of a local status, as the object GET serves it
// with the Note builder that the fan-out delivers too (federation.md §7.4, §10), against the
// Create that each implementation publishes.
func TestCreateNoteMatchesPeers(t *testing.T) {
	w := newNoteWorld()
	objects := appfederation.NewObjects(appfederation.ObjectsDeps{
		Statuses: w, Accounts: w, Graph: w, Timeline: w, Reader: w, Host: host, AccountDomain: accountDomain,
	})
	s := &server{mux: http.NewServeMux(), accounts: newAccounts(t), objects: newObjects(), graph: &fakeGraph{}}
	activitypub.New(activitypub.Options{
		Host: host, AccountDomain: accountDomain, SoftwareVersion: "0.1.0", SourceURL: "https://github.com/best-friends/my",
		Accounts: s.accounts, Objects: objects, Graph: s.graph, Problem: problem,
	}).Register(s.mux)

	rec := s.get(t, "/ap/objects/"+noteStatusID+"/activity", acceptMastodon)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET the Create = %d %s", rec.Code, rec.Body)
	}
	ours := decode(t, rec.Body.Bytes())
	for _, peer := range peers {
		checkShape(t, peer, ours, readPeer(t, peer, "create-note.json"), noteRequired[peer])
	}
	note := ours["object"].(map[string]any)
	tags := note["tag"].([]any)
	if m := tags[0].(map[string]any); m["type"] != "Mention" || m["name"] != "@bob@remote.example" || m["href"] != "https://remote.example/users/bob" {
		t.Errorf("mention = %v", m)
	}
	if h := tags[1].(map[string]any); h["type"] != "Hashtag" || h["name"] != "#fediverse" || h["href"] != "https://"+host+"/tags/fediverse" {
		t.Errorf("hashtag = %v", h)
	}
}
