// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// Limits on a received document (federation.md §4). A document over any of them cannot be
// normalized.
const (
	MaxDepth         = 16        // nesting depth of JSON objects and arrays
	MaxTags          = 100       // entries of one tag property
	MaxRecipients    = 1000      // to and cc together, per object or activity
	MaxBodyBytes     = 100 << 10 // one HTML value: content, a contentMap entry, a summary
	MaxAttachments   = 4         // attachments kept per object; the rest are dropped
	UndeterminedLang = "und"     // BCP 47 "undetermined"
)

var (
	// ErrMalformed reports a document that is not a JSON object or has no usable type.
	ErrMalformed = errors.New("federation: malformed AS2 document")
	// ErrLimitExceeded reports a document over one of the limits.
	ErrLimitExceeded = errors.New("federation: AS2 document exceeds a limit")
)

// IRI is an absolute IRI with a lower-case scheme and host. The empty IRI means "absent".
type IRI string

// PublicIRI is the special collection that addresses everyone (ActivityPub §5.6). Every
// spelling a document uses for it is normalized to this value.
const PublicIRI IRI = "https://www.w3.org/ns/activitystreams#Public"

// Origin returns scheme://host[:port] of i, or "" when i is not an absolute IRI.
func (i IRI) Origin() string {
	u, err := url.Parse(string(i))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// Host returns the host (without port) of i, or "".
func (i IRI) Host() string {
	u, err := url.Parse(string(i))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// SameOrigin reports whether a and b are absolute IRIs with the same origin.
func SameOrigin(a, b IRI) bool {
	o := a.Origin()
	return o != "" && o == b.Origin()
}

// ActivityType is a known activity type (federation.md §3.4).
type ActivityType string

// Known activity types.
const (
	ActivityCreate     ActivityType = "Create"
	ActivityUpdate     ActivityType = "Update"
	ActivityDelete     ActivityType = "Delete"
	ActivityFollow     ActivityType = "Follow"
	ActivityAccept     ActivityType = "Accept"
	ActivityReject     ActivityType = "Reject"
	ActivityUndo       ActivityType = "Undo"
	ActivityAnnounce   ActivityType = "Announce"
	ActivityLike       ActivityType = "Like"
	ActivityEmojiReact ActivityType = "EmojiReact"
	ActivityBlock      ActivityType = "Block"
	ActivityFlag       ActivityType = "Flag"
	ActivityAdd        ActivityType = "Add"
	ActivityRemove     ActivityType = "Remove"
	ActivityMove       ActivityType = "Move"
	// ActivityUnknown is any other type of a document with an actor; RawType keeps the name.
	ActivityUnknown ActivityType = ""
)

// ObjectType is a known object type.
type ObjectType string

// Known object types. Article, Page, Event, Image, Video, and Audio are converted to a Note-like
// body (Object.Converted).
const (
	ObjectNote      ObjectType = "Note"
	ObjectQuestion  ObjectType = "Question"
	ObjectArticle   ObjectType = "Article"
	ObjectPage      ObjectType = "Page"
	ObjectEvent     ObjectType = "Event"
	ObjectImage     ObjectType = "Image"
	ObjectVideo     ObjectType = "Video"
	ObjectAudio     ObjectType = "Audio"
	ObjectTombstone ObjectType = "Tombstone"
	// ObjectUnknown is any other type of a document without an actor; RawType keeps the name.
	ObjectUnknown ObjectType = ""
)

// ActorType is a known actor type.
type ActorType string

// Known actor types.
const (
	ActorPerson       ActorType = "Person"
	ActorService      ActorType = "Service"
	ActorApplication  ActorType = "Application"
	ActorGroup        ActorType = "Group"
	ActorOrganization ActorType = "Organization"
)

// Document is a normalized AS2 document: exactly one of the fields is set.
type Document struct {
	Activity *Activity
	Object   *Object
	Actor    *Actor
}

// ID returns the id of whichever document is set.
func (d *Document) ID() IRI {
	switch {
	case d == nil:
		return ""
	case d.Activity != nil:
		return d.Activity.ID
	case d.Object != nil:
		return d.Object.ID
	case d.Actor != nil:
		return d.Actor.ID
	}
	return ""
}

// Ref is a property that refers to another document: an IRI, a Link, or an embedded document.
// ID is the referred IRI in every case. The embedded document is kept with a flag saying whether
// it has the same origin as the document that embeds it (or no id of its own); only then may its
// contents be trusted (federation.md §4, ActivityPub appendix B.1). Otherwise it must be fetched
// from ID.
type Ref struct {
	ID         IRI
	embedded   *Document
	sameOrigin bool
}

// Trusted returns the embedded document when its contents may be used, or nil.
func (r Ref) Trusted() *Document {
	if r.sameOrigin {
		return r.embedded
	}
	return nil
}

// Untrusted returns the embedded document regardless of its origin, or nil. Use it only for
// hints that are verified elsewhere, such as matching an Accept to a pending Follow.
func (r Ref) Untrusted() *Document { return r.embedded }

// Audience is the addressing of an object or activity. bto and bcc are ignored: a sender must
// strip them before delivery.
type Audience struct {
	To []IRI
	Cc []IRI
}

// Activity is a normalized activity.
type Activity struct {
	ID        IRI
	Type      ActivityType
	RawType   string // the first type name of the document, for logs
	Actor     IRI
	Objects   []Ref // object; Flag carries several
	Target    IRI
	Audience  Audience
	Published time.Time // zero when absent
	// Content is the plain text of content: the comment of a Flag, the emoji of an EmojiReact.
	Content string
	// Reaction is the emoji of a Misskey reaction (_misskey_reaction on a Like) or of an
	// EmojiReact; empty for a plain Like.
	Reaction string
	Emojis   []Emoji
}

// Object returns the first object of a, or the zero Ref.
func (a *Activity) Object() Ref {
	if len(a.Objects) == 0 {
		return Ref{}
	}
	return a.Objects[0]
}

// Body is the HTML of a post in one language. HTML is as received; it must go through the
// server-side sanitizer before it is stored (federation.md §5).
type Body struct {
	Language string // canonical BCP 47 tag, or UndeterminedLang
	HTML     string
}

// Mention is a Mention tag.
type Mention struct {
	Href IRI
	Name string
}

// Emoji is a custom emoji tag.
type Emoji struct {
	ID        IRI
	Shortcode string // without colons
	IconURL   IRI
}

// Link is a normalized AS2 Link (or a bare IRI used where a Link is allowed).
type Link struct {
	Href      IRI
	MediaType string
	Name      string
}

// Attachment is a media attachment.
type Attachment struct {
	Type       string // Document, Image, Video, or Audio
	URL        IRI
	MediaType  string
	Name       string // alt text
	Blurhash   string
	FocalPoint *[2]float64
	Width      int
	Height     int
}

// PollOption is one choice of a Question.
type PollOption struct {
	Name  string
	Votes int
}

// Poll is the poll of a Question.
type Poll struct {
	Multiple    bool // anyOf instead of oneOf
	Options     []PollOption
	EndTime     time.Time
	Closed      time.Time
	VotersCount int
}

// Object is a normalized object: a post, a converted post, or a Tombstone.
type Object struct {
	ID           IRI
	Type         ObjectType
	RawType      string
	Converted    bool // Type is one of the converted types; Bodies holds the converted body
	AttributedTo IRI
	InReplyTo    IRI
	Audience     Audience
	Published    time.Time
	Updated      time.Time
	URL          IRI    // display URL (the first text/html url)
	Bodies       []Body // one per language, in document order
	SpoilerText  string // plain text
	Sensitive    bool
	Mentions     []Mention
	Hashtags     []string // without the leading '#'
	Emojis       []Emoji
	Attachments  []Attachment
	Poll         *Poll
}

// PublicKey is a publicKey entry of an actor.
type PublicKey struct {
	ID    IRI
	Owner IRI
	PEM   string
}

// Field is a PropertyValue of an actor's profile. Value is HTML as received.
type Field struct {
	Name  string
	Value string
}

// Actor is a normalized actor.
type Actor struct {
	ID                        IRI
	Type                      ActorType
	PreferredUsername         string
	Name                      string // plain text (AS2 name carries no markup)
	Summary                   string // HTML as received
	URL                       IRI
	Inbox                     IRI
	Outbox                    IRI
	SharedInbox               IRI
	Followers                 IRI
	Following                 IRI
	Featured                  IRI
	Icon                      IRI
	Image                     IRI
	ManuallyApprovesFollowers bool
	Discoverable              bool
	PublicKeys                []PublicKey
	Fields                    []Field
	Emojis                    []Emoji
	Published                 time.Time
}

// Bot reports whether the actor is an automated account.
func (a *Actor) Bot() bool { return a.Type == ActorService || a.Type == ActorApplication }

// normalizeIRI parses s as an absolute IRI and lower-cases its scheme and host. It returns ""
// when s is not an absolute IRI with a host.
func normalizeIRI(s string) IRI {
	s = strings.TrimSpace(s)
	switch s {
	case "as:Public", "Public", string(PublicIRI):
		return PublicIRI
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return IRI(u.String())
}
