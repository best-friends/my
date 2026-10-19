// SPDX-License-Identifier: AGPL-3.0-only

package htmlsanitizer

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// documentPolicy is the allowlist of the instance's documents (the terms of service and the
// privacy policy), which internal/adapter/markdown renders from the administrators' Markdown.
// It is wider than the policy of remote HTML: documents keep their headings, rules, and tables.
// Images, styles, classes, and every other attribute are dropped; links keep only http, https,
// and mailto URLs or relative ones, and get rel="nofollow noreferrer".
var documentPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"p", "br", "hr", "h2", "h3", "h4", "h5", "h6", "strong", "em", "del", "code", "pre", "blockquote",
		"ul", "ol", "li", "table", "thead", "tbody", "tr", "th", "td",
	)
	p.AllowAttrs("start").Matching(regexp.MustCompile(`^[0-9]{1,9}$`)).OnElements("ol")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	return p
}()

// SanitizeDocument returns the HTML fragment s reduced to the allowlist of the instance's
// documents. internal/adapter/markdown calls it on the HTML it renders, so that bluemonday stays
// in this package.
func SanitizeDocument(s string) string { return documentPolicy.Sanitize(s) }
