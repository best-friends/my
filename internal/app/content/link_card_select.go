// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/mycroblog/my/internal/domain/content"
)

// LinkCardURL returns the URL whose card a revision shows (ADR-0062 §1), normalized
// (content.NormalizeLinkURL), or "" for none: the first http or https link of its text that is
// not a mention, a hashtag, or a URL of localHosts. A revision with attachments or a poll shows
// no card. A local revision is read from the text its author typed (ParseText); a remote one
// from its sanitized HTML, where the links that the classes mention, hashtag, or u-url or
// rel="tag" mark, and those whose text starts with @ or #, are mentions and hashtags.
func LinkCardURL(rev content.Revision, localHosts []string) string {
	if len(rev.Attachments) > 0 || rev.Poll != nil {
		return ""
	}
	accept := func(raw string) string {
		u, ok := content.NormalizeLinkURL(raw)
		if !ok || slices.Contains(localHosts, content.LinkHost(u)) {
			return ""
		}
		return u
	}
	if rev.Source != nil {
		for _, s := range ParseText(rev.Source.Text).Segments {
			if s.Kind != SegmentURL {
				continue
			}
			if u := accept(s.Href); u != "" {
				return u
			}
		}
		return ""
	}
	// The bodies of a contentMap are translations of one text: the first one decides.
	if len(rev.Bodies) == 0 {
		return ""
	}
	for _, href := range htmlLinks(rev.Bodies[0].ContentHTML) {
		if u := accept(href); u != "" {
			return u
		}
	}
	return ""
}

// htmlLinks returns the href of each link of an HTML fragment that is not a mention or a
// hashtag, in document order.
func htmlLinks(fragment string) []string {
	var out []string
	z := html.NewTokenizer(strings.NewReader(fragment))
	var href string
	var inLink, skip bool
	var text strings.Builder
	finish := func() {
		if inLink && !skip {
			t := strings.TrimSpace(text.String())
			if !strings.HasPrefix(t, "@") && !strings.HasPrefix(t, "#") {
				out = append(out, href)
			}
		}
		inLink, skip = false, false
		text.Reset()
	}
	for {
		switch z.Next() {
		case html.ErrorToken:
			finish()
			return out
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			if atom.Lookup(name) != atom.A {
				continue
			}
			finish()
			inLink, href = true, ""
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = z.TagAttr()
				switch string(k) {
				case "href":
					href = string(v)
				case "class":
					for _, c := range strings.Fields(string(v)) {
						if c == "mention" || c == "hashtag" || c == "u-url" {
							skip = true
						}
					}
				case "rel":
					if slices.Contains(strings.Fields(strings.ToLower(string(v))), "tag") {
						skip = true
					}
				}
			}
		case html.TextToken:
			if inLink {
				text.Write(z.Text())
			}
		case html.EndTagToken:
			if name, _ := z.TagName(); atom.Lookup(name) == atom.A {
				finish()
			}
		}
	}
}
