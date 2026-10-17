// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	appfederation "github.com/best-friends/my/internal/app/federation"
)

// object serves GET /ap/objects/{id} (the Note of a local status) and GET
// /ap/objects/{id}/activity (its Create). The request's HTTP signature decides the requester,
// and the federation module's visibility decision answers: a status the requester may not see
// gets the same 404 as a missing one, and a deleted one that it could see gets 410 with a
// Tombstone (ADR-0018). A request that prefers HTML is redirected to the status page with 303
// when an anonymous visitor may see the status.
func (h *Handler) object(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept")
	id := r.PathValue("id")
	if !validID(id) {
		h.notFound(w, r)
		return
	}
	activity := strings.HasSuffix(r.URL.Path, "/"+activitySegment)
	html := parseAccept(r.Header.Get("Accept")).prefersHTML()

	var req appfederation.Requester
	if !html {
		origin := &url.URL{Scheme: "https", Host: h.o.Host}
		var err error
		if req, err = h.o.Objects.Authenticate(r.Context(), appfederation.NewSignedRequest(origin, r, nil)); err != nil {
			h.o.Logger.ErrorContext(r.Context(), "activitypub authenticate failed", slog.Any("error", err))
			h.o.Problem(w, r, http.StatusInternalServerError)
			return
		}
	}
	get := h.o.Objects.Status
	if activity {
		get = h.o.Objects.StatusActivity
	}
	obj, err := get(r.Context(), req, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if html {
		if obj.Gone || obj.URL == "" {
			h.notFound(w, r)
			return
		}
		redirect(w, r, obj.URL, http.StatusSeeOther)
		return
	}
	status, cache := http.StatusOK, cachePrivate
	if obj.Gone {
		status = http.StatusGone
	}
	if obj.Public {
		cache = cachePublic
	}
	writeBody(w, typeActivityJSON, cache, status, obj.Document)
}
