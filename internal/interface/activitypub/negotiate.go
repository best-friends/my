// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// activityStreamsProfile is the JSON-LD profile that marks application/ld+json as
// ActivityStreams.
const activityStreamsProfile = "https://www.w3.org/ns/activitystreams"

// preference is how much an Accept header wants ActivityPub JSON and HTML, as q-values.
// A media range that names the type exactly takes precedence over a wildcard (RFC 9110 §12.5.1).
type preference struct {
	ap, html         float64 // from exact media ranges, valid when the matching *Set is true
	apSet, htmlSet   bool
	apWild, htmlWild float64 // from */*, application/*, and text/*
}

// parseAccept reads the media ranges of an Accept header that matter here:
// application/activity+json, application/ld+json with the ActivityStreams profile (or none),
// and application/json for ActivityPub; text/html and application/xhtml+xml for HTML.
func parseAccept(header string) preference {
	var p preference
	for part := range strings.SplitSeq(header, ",") {
		mt, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		q := 1.0
		if s, ok := params["q"]; ok {
			if v, err := strconv.ParseFloat(s, 64); err == nil && v >= 0 && v <= 1 {
				q = v
			}
		}
		switch mt {
		case "application/activity+json", "application/json":
			p.ap, p.apSet = max(p.ap, q), true
		case "application/ld+json":
			if profile, ok := params["profile"]; !ok || hasProfile(profile, activityStreamsProfile) {
				p.ap, p.apSet = max(p.ap, q), true
			}
		case "text/html", "application/xhtml+xml":
			p.html, p.htmlSet = max(p.html, q), true
		case "*/*":
			p.apWild, p.htmlWild = max(p.apWild, q), max(p.htmlWild, q)
		case "application/*":
			p.apWild = max(p.apWild, q)
		case "text/*":
			p.htmlWild = max(p.htmlWild, q)
		}
	}
	return p
}

// hasProfile reports whether a space-separated profile parameter lists want.
func hasProfile(profiles, want string) bool {
	for _, p := range strings.Fields(profiles) {
		if p == want {
			return true
		}
	}
	return false
}

func (p preference) apQ() float64 {
	if p.apSet {
		return p.ap
	}
	return p.apWild
}

func (p preference) htmlQ() float64 {
	if p.htmlSet {
		return p.html
	}
	return p.htmlWild
}

// prefersHTML reports whether a request for an ActivityPub URI wants HTML: it ranks HTML above
// ActivityPub JSON. Requests without a preference get JSON.
func (p preference) prefersHTML() bool { return p.htmlQ() > p.apQ() }

// asksForActivityPub reports whether a request for a profile page names an ActivityPub type
// and ranks it at least as high as HTML. Wildcards alone ask for HTML.
func (p preference) asksForActivityPub() bool {
	return p.apSet && p.ap > 0 && p.ap >= p.htmlQ()
}

// AsksForActivityPub reports whether r names an ActivityPub media type in its Accept header
// and ranks it at least as high as HTML. Browsers, which send wildcards, ask for HTML. Pages
// that have no ActivityPub representation answer such requests with 404 instead of HTML.
func AsksForActivityPub(r *http.Request) bool {
	return parseAccept(r.Header.Get("Accept")).asksForActivityPub()
}
