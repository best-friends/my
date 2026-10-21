// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"regexp"
	"strings"

	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/domain/account"
)

// actorsPrefix is the path of local actor ids before the UUID (ADR-0018).
const actorsPrefix = "/ap/actors/"

// objectsPrefix is the path of local object ids before the UUID, and activitySegment the last
// segment of the IRI of a status's Create (ADR-0018).
const (
	objectsPrefix   = "/ap/objects/"
	activitySegment = "activity"
)

// quoteAuthorizationsPrefix is the path of the IRIs of the QuoteAuthorizations of this server
// before the UUID (ADR-0069 §1).
const quoteAuthorizationsPrefix = "/ap/quote_authorizations/"

// uuidPattern matches the canonical form of a UUIDv7: 36 lowercase characters.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// validID reports whether s is an id that a local URI may carry. ADR-0018 parses local URIs
// strictly, so a UUID in another case or form does not name anything.
func validID(s string) bool { return uuidPattern.MatchString(s) }

// Collections of an actor, the last path segment under {actor}/ (ADR-0018). The inbox only
// takes POSTs.
const (
	collInbox       = appfederation.CollectionInbox
	collOutbox      = appfederation.CollectionOutbox
	collFollowers   = appfederation.CollectionFollowers
	collFollowing   = appfederation.CollectionFollowing
	collFeatured    = appfederation.CollectionFeatured
	collFeaturedTag = appfederation.CollectionFeaturedTags
)

// sharedInboxPath is the path of the instance's shared inbox.
const sharedInboxPath = "/ap/inbox"

// profileURL returns the human-readable profile URL of a local username on host.
func profileURL(host, username string) string { return appfederation.ProfileURL(host, username) }

// parseLocalURI returns the actor id or the username that a local https URI names: an actor
// id (https://{host}/ap/actors/{uuid}) or a profile URL (https://{host}/@{username}).
func parseLocalURI(host, uri string) (id, username string, ok bool) {
	origin := "https://" + host + "/"
	if len(uri) < len(origin) || !strings.EqualFold(uri[:len(origin)], origin) {
		return "", "", false
	}
	rest := uri[len(origin):]
	if strings.ContainsAny(rest, "?#") {
		return "", "", false
	}
	if s, found := strings.CutPrefix(rest, "ap/actors/"); found && validID(s) {
		return s, "", true
	}
	if s, found := strings.CutPrefix(rest, "@"); found && account.ValidateUsername(s) == nil {
		return "", s, true
	}
	return "", "", false
}
