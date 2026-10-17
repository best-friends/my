// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"html"
	"strings"
)

// canonicalLanguage normalizes the case and separators of a BCP 47 tag ("zh_hant_tw" →
// "zh-Hant-TW") and returns UndeterminedLang for an empty or malformed one (NFR-I18N-5).
func canonicalLanguage(tag string) string {
	tag = strings.TrimSpace(strings.ReplaceAll(tag, "_", "-"))
	if tag == "" {
		return UndeterminedLang
	}
	parts := strings.Split(tag, "-")
	if l := len(parts[0]); l < 2 || l > 8 || !allLetters(parts[0]) {
		return UndeterminedLang
	}
	extension := false // subtags after a singleton are lower case
	for i, p := range parts {
		if p == "" || len(p) > 8 || !alnum(p) {
			return UndeterminedLang
		}
		switch {
		case i == 0 || extension:
			parts[i] = strings.ToLower(p)
		case len(p) == 1:
			extension = true
			parts[i] = strings.ToLower(p)
		case len(p) == 4 && !isDigit(p[0]):
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		case len(p) == 2 || (len(p) == 3 && isDigit(p[0])):
			parts[i] = strings.ToUpper(p)
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "-")
}

func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

func allLetters(s string) bool {
	for i := range len(s) {
		if !isLetter(s[i]) {
			return false
		}
	}
	return true
}

func alnum(s string) bool {
	for i := range len(s) {
		if !isLetter(s[i]) && !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// plainText turns an HTML fragment into plain text: tags are removed, <br> and the end of a
// block become line breaks, and character references are decoded. It is for values stored as
// plain text (a CW, a display name); it is not a sanitizer.
func plainText(s string) string {
	if !strings.ContainsAny(s, "<&") {
		return strings.TrimSpace(s)
	}
	var b strings.Builder
	for len(s) > 0 {
		i := strings.IndexByte(s, '<')
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		s = s[i:]
		j := strings.IndexByte(s, '>')
		if j < 0 {
			break // an unterminated tag: drop the rest
		}
		name := tagName(s[1:j])
		switch name {
		case "br", "/p", "/div", "/li", "/blockquote", "/h1", "/h2", "/h3", "/h4", "/h5", "/h6":
			b.WriteByte('\n')
		}
		s = s[j+1:]
	}
	return strings.TrimSpace(html.UnescapeString(b.String()))
}

// tagName returns the lower-case name of a tag body such as "br/", "/p", or `a href="x"`.
func tagName(body string) string {
	body = strings.TrimSpace(body)
	end := strings.IndexAny(body, " \t\n\r/>")
	if strings.HasPrefix(body, "/") {
		rest := strings.TrimSpace(body[1:])
		if e := strings.IndexAny(rest, " \t\n\r>"); e >= 0 {
			rest = rest[:e]
		}
		return "/" + strings.ToLower(rest)
	}
	if end >= 0 {
		body = body[:end]
	}
	return strings.ToLower(body)
}

// convertedBody builds the body of a converted type the way Mastodon does: the name as a bold
// paragraph, the summary as a paragraph, and a link to the URL. Every part is escaped.
func convertedBody(name, summary string, link IRI) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("<p><strong>" + html.EscapeString(name) + "</strong></p>")
	}
	if summary != "" {
		b.WriteString("<p>" + strings.ReplaceAll(html.EscapeString(summary), "\n", "<br>") + "</p>")
	}
	if link != "" {
		l := html.EscapeString(string(link))
		b.WriteString(`<p><a href="` + l + `">` + l + "</a></p>")
	}
	return b.String()
}
