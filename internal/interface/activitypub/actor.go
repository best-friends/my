// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"errors"
	"net/http"
	"strings"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/federation"
)

// actorDocument is an ActivityStreams actor in the shape Mastodon publishes, which Misskey,
// GoToSocial, and Akkoma also read. Fields are in Mastodon's order.
type actorDocument struct {
	Context                   []any             `json:"@context"`
	ID                        string            `json:"id"`
	Type                      string            `json:"type"`
	Following                 string            `json:"following"`
	Followers                 string            `json:"followers"`
	Inbox                     string            `json:"inbox"`
	Outbox                    string            `json:"outbox"`
	Featured                  string            `json:"featured,omitempty"`
	FeaturedTags              string            `json:"featuredTags,omitempty"`
	PreferredUsername         string            `json:"preferredUsername"`
	Name                      *string           `json:"name,omitempty"`
	Summary                   *string           `json:"summary,omitempty"`
	URL                       string            `json:"url,omitempty"`
	ManuallyApprovesFollowers bool              `json:"manuallyApprovesFollowers"`
	Discoverable              *bool             `json:"discoverable,omitempty"`
	Indexable                 *bool             `json:"indexable,omitempty"`
	Published                 string            `json:"published"`
	PublicKey                 *publicKey        `json:"publicKey,omitempty"`
	Tag                       []any             `json:"tag"`
	Attachment                []propertyValue   `json:"attachment"`
	Endpoints                 map[string]string `json:"endpoints"`
}

type publicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPEM string `json:"publicKeyPem"`
}

// propertyValue is a profile field (FR-006) as Mastodon's schema:PropertyValue.
type propertyValue struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// activityTypes maps actor types to ActivityStreams types.
var activityTypes = map[account.ActorType]string{
	account.ActorPerson:       "Person",
	account.ActorService:      "Service",
	account.ActorApplication:  "Application",
	account.ActorGroup:        "Group",
	account.ActorOrganization: "Organization",
}

// instanceActorUsername is the preferredUsername and WebFinger username of the instance actor:
// the account domain, as Mastodon names its instance actor. Local usernames cannot contain
// dots, so it never collides with a user unless the domain has no dot.
func (h *Handler) instanceActorUsername() string { return h.o.AccountDomain }

// actorDocument builds the document of a visible local actor.
func (h *Handler) actorDocument(a appaccount.Actor) actorDocument {
	id := a.Account.URI
	doc := actorDocument{
		Context:   federation.LDContext(),
		ID:        id,
		Type:      activityTypes[a.Account.ActorType],
		Following: id + "/" + collFollowing,
		Followers: id + "/" + collFollowers,
		Inbox:     id + "/" + collInbox,
		Outbox:    id + "/" + collOutbox,
		Published: a.Account.CreatedAt.UTC().Truncate(time.Second).Format(time.RFC3339),
		Tag:       []any{},
		Endpoints: map[string]string{"sharedInbox": sharedInboxURI(h.o.Host)},
	}
	for _, k := range a.Keys {
		if k.Algorithm == account.KeyRSASHA256 && k.KeyID == account.KeyIRI(id, account.KeyRSASHA256) {
			doc.PublicKey = &publicKey{ID: k.KeyID, Owner: id, PublicKeyPEM: k.PublicKeyPEM}
		}
	}
	doc.Attachment = []propertyValue{}
	if a.Account.ActorType == account.ActorApplication {
		// The instance actor signs the instance's own requests and is followed by no one.
		doc.PreferredUsername = h.instanceActorUsername()
		doc.ManuallyApprovesFollowers = true
		return doc
	}
	doc.Featured = id + "/" + collFeatured
	doc.FeaturedTags = id + "/" + collFeaturedTag
	if a.Local != nil {
		doc.PreferredUsername = a.Local.Username
		doc.URL = profileURL(h.o.Host, a.Local.Username)
	}
	if p := a.Profile; p != nil {
		if p.Bot && a.Account.ActorType == account.ActorPerson {
			doc.Type = activityTypes[account.ActorService] // Mastodon's bot flag
		}
		doc.Name, doc.Summary = &p.DisplayName, &p.NoteHTML
		doc.ManuallyApprovesFollowers = p.Locked
		doc.Discoverable, doc.Indexable = &p.Discoverable, &p.Indexable
		for _, f := range p.Fields {
			doc.Attachment = append(doc.Attachment, propertyValue{Type: "PropertyValue", Name: f.Name, Value: f.ValueHTML})
		}
	}
	return doc
}

// actor serves GET /ap/actors/{id}: the actor document, or for a request that prefers HTML
// a 303 redirect to the profile page (ADR-0018).
func (h *Handler) actor(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept")
	a, err := h.lookupID(r, r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	doc := h.actorDocument(a)
	if doc.URL != "" && parseAccept(r.Header.Get("Accept")).prefersHTML() {
		redirect(w, r, doc.URL, http.StatusSeeOther)
		return
	}
	h.writeJSON(w, r, typeActivityJSON, doc)
}

// lookupID finds the visible local actor with an id from a URI.
func (h *Handler) lookupID(r *http.Request, id string) (appaccount.Actor, error) {
	if !validID(id) {
		return appaccount.Actor{}, appaccount.ErrNotFound
	}
	return findVisible(h.o.Accounts.FindByID(r.Context(), id))
}

// lookupUsername finds the visible local actor with a username: a user, or the instance actor
// under instanceActorUsername.
func (h *Handler) lookupUsername(r *http.Request, username string) (appaccount.Actor, error) {
	a, err := h.o.Accounts.FindByHandle(r.Context(), username, account.LocalDomain)
	if errors.Is(err, appaccount.ErrNotFound) && strings.EqualFold(username, h.instanceActorUsername()) {
		a, err = h.o.Accounts.InstanceActor(r.Context())
	}
	return findVisible(a, err)
}

// profilePage serves GET /@{username}. An ActivityPub request is redirected to the actor id
// with 302 (ADR-0018), so that pasting a profile URL into a remote search finds the actor;
// any other request gets the HTML of the web client. Single-segment paths without "@" are
// the web client's.
func (h *Handler) profilePage(w http.ResponseWriter, r *http.Request) {
	username, ok := cutAt(r.PathValue("name"))
	if !ok {
		h.html(w, r)
		return
	}
	w.Header().Add("Vary", "Accept")
	if !AsksForActivityPub(r) {
		h.html(w, r)
		return
	}
	if account.ValidateUsername(username) != nil {
		h.notFound(w, r)
		return
	}
	a, err := h.lookupUsername(r, username)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if a.Local == nil { // the instance actor has no profile page
		h.notFound(w, r)
		return
	}
	redirect(w, r, a.Account.URI, http.StatusFound)
}

// html serves the page of the web client, or 404 without one.
func (h *Handler) html(w http.ResponseWriter, r *http.Request) {
	if h.o.HTML == nil {
		h.notFound(w, r)
		return
	}
	h.o.HTML.ServeHTTP(w, r)
}

// cutAt returns the username of a profile page path segment "@username".
func cutAt(segment string) (string, bool) {
	s, ok := strings.CutPrefix(segment, "@")
	return s, ok && s != ""
}
