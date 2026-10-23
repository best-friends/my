// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"net/http"

	"github.com/best-friends/my/internal/app/kernel"

	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/domain/federation"
)

// emojisPrefix is the path of the ids of local custom emoji before the UUID (ADR-0051 §3).
const emojisPrefix = "/ap/emojis/"

// emoji serves GET /ap/emojis/{id}: the Emoji of a local custom emoji with its context, which
// Mastodon resolves by its id (ADR-0051 §3). An unknown, deleted, or disabled emoji, a remote
// one, and a request that prefers HTML get the same 404. The emoji and its image are public, so
// authorized fetch does not apply.
func (h *Handler) emoji(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept")
	id := r.PathValue("id")
	if !kernel.ValidID(id) || h.o.EmojiTags == nil || parseAccept(r.Header.Get("Accept")).prefersHTML() {
		h.notFound(w, r)
		return
	}
	tags, err := h.o.EmojiTags(r.Context(), []string{id})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if len(tags) != 1 || !tags[0].Local {
		h.notFound(w, r)
		return
	}
	doc := federation.TopLevelEmojiOf(appfederation.EmojiTagOf(tags[0], h.o.Host))
	h.writeJSON(w, r, typeActivityJSON, doc)
}
