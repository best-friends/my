// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import "net/http"

const contextActivityStreams = "https://www.w3.org/ns/activitystreams"

// orderedCollection is an empty ActivityStreams collection. Posts, follows, and pins do not
// exist yet, so every collection an actor names is empty and fits on its own page.
type orderedCollection struct {
	Context      string `json:"@context"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	TotalItems   int    `json:"totalItems"`
	OrderedItems *[]any `json:"orderedItems,omitempty"`
	Items        *[]any `json:"items,omitempty"`
}

// collection serves GET /ap/actors/{id}/{collection} for the collections an actor document
// names, except the inbox.
func (h *Handler) collection(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("collection")
	switch name {
	case collOutbox, collFollowers, collFollowing, collFeatured, collFeaturedTag:
	default:
		h.notFound(w, r)
		return
	}
	a, err := h.lookupID(r, r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	isInstanceActor := a.Local == nil
	if isInstanceActor && (name == collFeatured || name == collFeaturedTag) {
		h.notFound(w, r) // its document does not name them
		return
	}
	c := orderedCollection{Context: contextActivityStreams, ID: a.Account.URI + "/" + name, Type: "OrderedCollection"}
	switch name {
	case collFeatured:
		// Mastodon reads pinned posts from orderedItems without paging.
		c.OrderedItems = &[]any{}
	case collFeaturedTag:
		// Mastodon publishes featured tags as an unordered Collection of Hashtag items.
		c.Type, c.Items = "Collection", &[]any{}
	}
	h.writeJSON(w, r, typeActivityJSON, c)
}
