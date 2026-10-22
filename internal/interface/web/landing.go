// SPDX-License-Identifier: AGPL-3.0-only

package web

import (
	"context"
	"net/http"
	"strings"
)

// This file serves the shells of the pages about the instance itself, the landing page (/) and
// the information page (/about), with the meta tags of a link preview of the instance (ADR-0028):
// its name, its one-paragraph summary, and its preview image. The pages are the same for every
// visitor; the web client shows the landing page to visitors without a session and the home
// timeline to signed-in users, and the information page to everyone.

// Paths of the pages about the instance.
const (
	pathLanding = "/"
	pathAbout   = "/about"
)

// Paths of the web client's pages of the instance's documents, which GET /instance links to.
// They are plain SPA routes: the generic shell serves them.
const (
	PathAboutTerms              = "/about/terms"
	PathAboutPrivacyPolicy      = "/about/privacy"
	PathAboutCommerceDisclosure = "/about/commerce"
)

// serveInstancePage serves the shell of the page about the instance at path, pathLanding or
// pathAbout.
func (h *Handler) serveInstancePage(w http.ResponseWriter, r *http.Request, path string) {
	doc, err := h.renderInstancePage(r.Context(), path)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.shellHeaders(w.Header())
	doc.serve(w, r)
}

// renderInstancePage renders the shell of the page about the instance at path in the instance's
// default locale. The landing page is titled with the instance's name, the information page
// "About {name}".
func (h *Handler) renderInstancePage(ctx context.Context, path string) (document, error) {
	lang := h.instanceLocale(ctx)
	name := h.instanceName(ctx)
	title := name
	if path == pathAbout {
		title = h.o.Messages.Localizer(lang).ServerPreviewAboutTitle(name)
	}
	summary := ""
	if h.o.InstanceDescription != nil {
		summary = description(strings.TrimSpace(h.o.InstanceDescription(ctx)))
	}
	img, err := h.instanceImage(ctx, lang)
	if err != nil {
		return document{}, err
	}
	meta := previewMeta{
		Type: ogTypeWebsite, SiteName: name, Title: title, URL: "https://" + h.o.Host + path,
		Description: summary, Locale: ogLocale(lang), Image: img, Card: card(img),
	}
	// The pages about the instance state the instance's own policy (ADR-0057).
	settings, err := h.settings(ctx)
	if err != nil {
		return document{}, err
	}
	return h.renderPreview(ctx, lang, meta, settings.CrawlerPolicy(), []string{"instance", path, title, summary, imageVersion(img)})
}
