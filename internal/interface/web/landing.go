// SPDX-License-Identifier: AGPL-3.0-only

package web

import (
	"context"
	"net/http"
	"strings"
)

// This file serves the shell of the landing page (/) with the meta tags of a link preview of the
// instance itself (ADR-0028): its name, its one-paragraph summary, and its preview image. The page
// is the same for every visitor; the web client shows the landing page to visitors without a
// session and the home timeline to signed-in users.

// serveLanding serves the shell of the landing page.
func (h *Handler) serveLanding(w http.ResponseWriter, r *http.Request) {
	doc, err := h.renderLanding(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.shellHeaders(w.Header())
	doc.serve(w, r)
}

// renderLanding renders the shell of the landing page in the instance's default locale.
func (h *Handler) renderLanding(ctx context.Context) (document, error) {
	lang := h.instanceLocale(ctx)
	name := h.instanceName(ctx, lang)
	summary := ""
	if h.o.InstanceDescription != nil {
		summary = description(strings.TrimSpace(h.o.InstanceDescription(ctx, lang)))
	}
	img, err := h.instanceImage(ctx, lang)
	if err != nil {
		return document{}, err
	}
	meta := previewMeta{
		Type: ogTypeWebsite, SiteName: name, Title: name, URL: "https://" + h.o.Host + "/",
		Description: summary, Locale: ogLocale(lang), Image: img, Card: card(img),
	}
	return h.renderPreview(ctx, lang, meta, []string{"landing", summary, imageVersion(img)})
}
