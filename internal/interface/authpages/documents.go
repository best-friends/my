// SPDX-License-Identifier: AGPL-3.0-only

package authpages

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/i18n"
)

// The instance's documents: the terms of service and the privacy policy that administrators
// write in Markdown (instance texts terms and privacy_policy), once, in any language: every
// locale shows the same text. Their public pages need no session and no JavaScript; the
// registration page asks applicants to agree to them, and every page links to them in its
// footer. The commerce disclosures of the supporter plans (instance text commerce_disclosure,
// ADR-0072 §13) are a document page too, which the web client links to instead. A document that
// the instance has not published is a 404, and no page links to it.

// Paths of the document pages.
const (
	PathTerms         = "/auth/terms"
	PathPrivacyPolicy = "/auth/privacy"
	// PathCommerceDisclosure is the page of the commerce disclosures of the supporter plans
	// (ADR-0072 §13), which the web client's /about and the subscription screens link to, and
	// which the operators give the payment provider; the server never vouches for them.
	PathCommerceDisclosure = "/about/commerce"
)

// cacheDocument lets shared caches keep a document page briefly; it carries no cookie and
// varies only with Accept-Language.
const cacheDocument = "public, max-age=60"

// Documents reads the instance's documents; *appinstance.Service implements it.
type Documents interface {
	// Document returns the document key rendered, and false when the instance has not published
	// it.
	Document(ctx context.Context, key instance.TextKey) (appinstance.Document, bool, error)
	// HasDocument reports whether Document would find the document.
	HasDocument(ctx context.Context, key instance.TextKey) (bool, error)
}

// documentPages are the document pages: the text key, the path, the title, the message of the
// 404 when the instance has not published it, and whether the footer and the registration page
// link to it (the commerce disclosures are not terms that applicants agree to).
var documentPages = []struct {
	key     instance.TextKey
	path    string
	title   func(*i18n.Localizer) string
	missing func(*i18n.Localizer) string
	linked  bool
}{
	{instance.TextTerms, PathTerms, (*i18n.Localizer).ServerAuthDocumentTermsTitle, (*i18n.Localizer).ServerAuthDocumentTermsMissing, true},
	{instance.TextPrivacyPolicy, PathPrivacyPolicy, (*i18n.Localizer).ServerAuthDocumentPrivacyTitle, (*i18n.Localizer).ServerAuthDocumentPrivacyMissing, true},
	{
		instance.TextCommerceDisclosure, PathCommerceDisclosure, (*i18n.Localizer).ServerAuthDocumentCommerceTitle,
		(*i18n.Localizer).ServerAuthDocumentCommerceMissing, false,
	},
}

// documentView is a document on its page.
type documentView struct {
	HTML template.HTML
	// Updated is the date of the revision, in UTC, as YYYY-MM-DD.
	Updated string
}

// documentLink is a link to a document page in the footer and on the registration page.
type documentLink struct {
	Path, Title string
}

// documentPage serves the page of a document.
func (h *Handler) documentPage(i int) http.HandlerFunc {
	d := documentPages[i]
	return func(w http.ResponseWriter, r *http.Request) {
		p := h.newPage(r, d.title)
		var doc appinstance.Document
		ok := false
		if h.o.Documents != nil {
			var err error
			doc, ok, err = h.o.Documents.Document(r.Context(), d.key)
			if err != nil {
				h.internalError(w, r, err)
				return
			}
		}
		if !ok {
			h.renderError(w, r, http.StatusNotFound, d.missing)
			return
		}
		p.Document = &documentView{
			// The renderer sanitizes the HTML (instance.DocumentRenderer).
			HTML:    template.HTML(doc.HTML), //nolint:gosec // G203: sanitized by the document allowlist
			Updated: doc.RevisedAt.UTC().Format("2006-01-02"),
		}
		p.cache = cacheDocument
		h.render(w, r, http.StatusOK, pageDocument, p)
	}
}

// documentLinks returns the links to the documents that the instance has published, titled in
// the page's locale.
// A failure to read them is logged and leaves them out, so that it never breaks a page.
func (h *Handler) documentLinks(r *http.Request, l *i18n.Localizer) []documentLink {
	if h.o.Documents == nil {
		return nil
	}
	var out []documentLink
	for _, d := range documentPages {
		if !d.linked {
			continue
		}
		ok, err := h.o.Documents.HasDocument(r.Context(), d.key)
		if err != nil {
			h.o.Logger.WarnContext(r.Context(), "read an instance document", slog.String("key", string(d.key)), slog.Any("error", err))
			continue
		}
		if ok {
			out = append(out, documentLink{Path: d.path, Title: d.title(l)})
		}
	}
	return out
}
