// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/federation"
)

// Collections of a local actor, the last path segment under {actor}/ (ADR-0018).
const (
	CollectionInbox        = "inbox"
	CollectionOutbox       = "outbox"
	CollectionFollowers    = "followers"
	CollectionFollowing    = "following"
	CollectionFeatured     = "featured"
	CollectionFeaturedTags = "featured-tags"
)

// SharedInboxURI returns the shared inbox of the instance on host.
func SharedInboxURI(host string) string { return "https://" + host + "/ap/inbox" }

// ProfileURL returns the human-readable profile URL of a local username on host.
func ProfileURL(host, username string) string { return "https://" + host + "/@" + username }

// LocalActorDocument returns the document of a local actor served on host: the answer to a GET
// of its id (federation.md §10) and the object of the Update its profile revisions send
// (§7.1). The instance actor's document is the minimal one that signs the instance's requests;
// its preferredUsername is instanceUsername.
func LocalActorDocument(host string, a appaccount.Actor, instanceUsername string) federation.ActorDocument {
	id := federation.IRI(a.Account.URI)
	doc := federation.ActorDocument{
		Context:    federation.LDContext(),
		ID:         id,
		Type:       federation.ActorType(a.ActivityStreamsType()),
		Following:  id + "/" + CollectionFollowing,
		Followers:  id + "/" + CollectionFollowers,
		Inbox:      id + "/" + CollectionInbox,
		Outbox:     id + "/" + CollectionOutbox,
		Published:  federation.FormatTime(a.Account.CreatedAt),
		Tag:        []any{},
		Attachment: []federation.PropertyValue{},
		Endpoints:  map[string]federation.IRI{"sharedInbox": federation.IRI(SharedInboxURI(host))},
	}
	if k, ok := a.PublishedKey(); ok {
		doc.PublicKey = &federation.PublicKeyDocument{ID: k.KeyID, Owner: id, PublicKeyPEM: k.PublicKeyPEM}
	}
	if a.IsInstanceActor() {
		// The instance actor signs the instance's own requests and is followed by no one.
		doc.PreferredUsername = instanceUsername
		doc.ManuallyApprovesFollowers = true
		return doc
	}
	doc.Featured = id + "/" + CollectionFeatured
	doc.FeaturedTags = id + "/" + CollectionFeaturedTags
	if a.Local != nil {
		doc.PreferredUsername = a.Local.Username
		doc.URL = ProfileURL(host, a.Local.Username)
	}
	if p := a.Profile; p != nil {
		doc.Name, doc.Summary = &p.DisplayName, &p.NoteHTML
		doc.ManuallyApprovesFollowers = p.Locked
		doc.Discoverable, doc.Indexable = &p.Discoverable, &p.Indexable
		for _, f := range p.Fields {
			doc.Attachment = append(doc.Attachment, federation.PropertyValue{Type: "PropertyValue", Name: f.Name, Value: f.ValueHTML})
		}
	}
	return doc
}
