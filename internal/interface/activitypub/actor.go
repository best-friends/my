// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"errors"
	"net/http"
	"strings"

	appaccount "github.com/best-friends/my/internal/app/account"
	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/federation"
)

// instanceActorUsername is the preferredUsername and WebFinger username of the instance actor:
// the account domain, as Mastodon names its instance actor. Local usernames cannot contain
// dots, so it never collides with a user unless the domain has no dot.
func (h *Handler) instanceActorUsername() string { return h.o.AccountDomain }

// actorDocument builds the document of a visible local actor.
func (h *Handler) actorDocument(a appaccount.Actor) federation.ActorDocument {
	return appfederation.LocalActorDocument(h.o.Host, a, h.instanceActorUsername())
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
