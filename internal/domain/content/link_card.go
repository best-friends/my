// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"net"
	"net/url"
	"strings"
	"time"
)

// LinkCardRefetchAfter is how long a link card stays fresh (ADR-0062 §6): a status that links
// the URL after it has the card fetched again.
const LinkCardRefetchAfter = 14 * 24 * time.Hour

// Bounds of the values of a link card (ADR-0062 §2), in characters (grapheme clusters) unless
// noted otherwise.
const (
	MaxLinkCardTitle        = 200
	MaxLinkCardDescription  = 500
	MaxLinkCardSiteName     = 100
	MaxLinkCardAuthorName   = 100
	MaxLinkCardImageAlt     = 1500
	MaxLinkCardImageURLSize = 2048 // bytes
)

// LinkCard is the card of a URL (link_cards, ADR-0062): the title, description, and image that
// its page declares, shared by every status that links the URL.
type LinkCard struct {
	// URL is the requested URL, normalized (NormalizeLinkURL); it keys the card.
	URL string
	// CardURL is where the card links: the final URL of the fetch, or the og:url of the page
	// when it has the same origin.
	CardURL     string
	Title       string
	Description string // "" when the page has none
	SiteName    string // "" when the page has none
	AuthorName  string // "" when the page names no author
	// PublishedAt is when the page says it was published; nil when it does not say.
	PublishedAt *time.Time
	// ImageMediaID is the media file of the image (link_card_media), "" for none.
	ImageMediaID string
	// CreatorAccountID is the account of the page's verified fediverse:creator, "" for none.
	CreatorAccountID string
	// Language is the BCP 47 language of the page (<html lang>), "" when unknown.
	Language  string
	FetchedAt time.Time
}

// Stale reports whether the card was fetched LinkCardRefetchAfter ago or longer at now.
func (c LinkCard) Stale(now time.Time) bool {
	return !now.Before(c.FetchedAt.Add(LinkCardRefetchAfter))
}

// LinkCardMedia is the image of link cards (link_card_media, ADR-0062 §3), a subtype of a media
// file whose owner is no account. The file was fetched once to check its format and read its
// size and blurhash; the media proxy serves it.
type LinkCardMedia struct {
	MediaID   string
	RemoteURL string
	// ContentType is the media type detected from the content.
	ContentType   string
	Width, Height int
	Blurhash      string
	// Description is the og:image:alt, "" for none.
	Description string
}

// StatusLinkCard is the URL whose card a status shows (status_link_cards).
type StatusLinkCard struct {
	StatusID string
	URL      string
}

// NormalizeLinkURL returns the form of an http or https URL that keys its link card (ADR-0062
// §6): the scheme and host lowercase, without the default port, the fragment, and the user
// information. The query stays as it is. It reports false for anything that is not an absolute
// http or https URL with a host.
func NormalizeLinkURL(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Opaque != "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", false
	}
	if port := u.Port(); port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	u.Scheme, u.Host, u.User, u.Fragment, u.RawFragment = scheme, host, nil, "", ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), true
}

// LinkHost returns the host of an http or https URL, lowercase and without a port, or "" when
// raw has none.
func LinkHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
