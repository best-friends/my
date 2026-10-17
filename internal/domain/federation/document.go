// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"encoding/json"
	"time"
)

// This file holds the shape of the documents this server publishes and delivers: the JSON-LD
// context and the activity envelope (federation.md §7.4). The parts of an object document that
// depend on its kind are built by the caller and passed as the activity's object.

// LDContext returns the JSON-LD context of the documents this server publishes: ActivityStreams,
// the security vocabulary of publicKey, and the Mastodon extensions in use (ADR-0006). Actor
// documents and outbound activities share it. Each call returns a new value, so callers may not
// change what others send.
func LDContext() []any {
	return []any{
		"https://www.w3.org/ns/activitystreams",
		"https://w3id.org/security/v1",
		map[string]any{
			"manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
			"toot":                      "http://joinmastodon.org/ns#",
			"featured":                  map[string]string{"@id": "toot:featured", "@type": "@id"},
			"featuredTags":              map[string]string{"@id": "toot:featuredTags", "@type": "@id"},
			"discoverable":              "toot:discoverable",
			"indexable":                 "toot:indexable",
			"schema":                    "http://schema.org#",
			"PropertyValue":             "schema:PropertyValue",
			"value":                     "schema:value",
			"sensitive":                 "as:sensitive",
			"Hashtag":                   "as:Hashtag",
			"blurhash":                  "toot:blurhash",
			"focalPoint":                map[string]string{"@container": "@list", "@id": "toot:focalPoint"},
		},
	}
}

// FormatTime formats t as the documents carry times: RFC 3339 in UTC, to the second, as
// Mastodon publishes them.
func FormatTime(t time.Time) string { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }

// ActivityDocument is an outbound activity as it is delivered. Context is set on the top-level
// document only; an activity embedded in another (the Announce of an Undo) has none.
type ActivityDocument struct {
	Context   []any        `json:"@context,omitempty"`
	ID        IRI          `json:"id"`
	Type      ActivityType `json:"type"`
	Actor     IRI          `json:"actor"`
	Published string       `json:"published,omitempty"`
	To        []IRI        `json:"to"`
	Cc        []IRI        `json:"cc"`
	// Object is an IRI or an embedded document.
	Object any `json:"object"`
}

// NewActivityDocument returns the top-level document of an activity with its context. A zero
// published leaves it out.
func NewActivityDocument(id IRI, t ActivityType, actor IRI, published time.Time, aud Audience, object any) ActivityDocument {
	d := ActivityDocument{Context: LDContext(), ID: id, Type: t, Actor: actor, Object: object}
	d.To, d.Cc = nonNil(aud.To), nonNil(aud.Cc)
	if !published.IsZero() {
		d.Published = FormatTime(published)
	}
	return d
}

// Embedded returns d without its context, to embed it in another activity.
func (d ActivityDocument) Embedded() ActivityDocument {
	d.Context = nil
	return d
}

// Marshal returns the JSON of d, the bytes every delivery of it sends.
func (d ActivityDocument) Marshal() ([]byte, error) { return json.Marshal(d) }

// ObjectDocument is the part of an object document that every kind shares: its identity, author,
// and addressing. A Note adds its body in NoteDocument.
type ObjectDocument struct {
	ID           IRI        `json:"id"`
	Type         ObjectType `json:"type"`
	InReplyTo    *IRI       `json:"inReplyTo,omitempty"`
	Published    string     `json:"published,omitempty"`
	Updated      string     `json:"updated,omitempty"`
	URL          string     `json:"url,omitempty"`
	AttributedTo IRI        `json:"attributedTo,omitempty"`
	To           []IRI      `json:"to,omitempty"`
	Cc           []IRI      `json:"cc,omitempty"`
}

// TopLevelObject is an object document served on its own, with its context: the answer to a GET
// of an object IRI (federation.md §10).
type TopLevelObject struct {
	Context []any `json:"@context"`
	ObjectDocument
}

// TopLevel returns o as a top-level document.
func (o ObjectDocument) TopLevel() TopLevelObject {
	return TopLevelObject{Context: LDContext(), ObjectDocument: o}
}

// NoteDocument is the Note of a status (federation.md §7.4): the shared part, then the body of
// one revision. Sensitive, Content, Tag, and Attachment are always present, as Mastodon
// publishes them; Summary and ContentMap are left out when empty.
type NoteDocument struct {
	ObjectDocument
	// Summary is the content warning.
	Summary   string `json:"summary,omitempty"`
	Sensitive bool   `json:"sensitive"`
	// Content is the HTML of the revision's language; ContentMap has the HTML per language
	// (NFR-I18N-5), without the undetermined language "und".
	Content    string               `json:"content"`
	ContentMap map[string]string    `json:"contentMap,omitempty"`
	Attachment []AttachmentDocument `json:"attachment"`
	Tag        []TagDocument        `json:"tag"`
}

// TopLevelNote is a Note served on its own, with its context.
type TopLevelNote struct {
	Context []any `json:"@context"`
	NoteDocument
}

// TopLevel returns n as a top-level document.
func (n NoteDocument) TopLevel() TopLevelNote {
	return TopLevelNote{Context: LDContext(), NoteDocument: n}
}

// AttachmentType is the type of every attachment of a Note: Mastodon publishes images, videos,
// and audio alike as Document, and the other implementations read the kind from MediaType.
const AttachmentType ObjectType = "Document"

// AttachmentDocument is a media attachment of a Note. Name is the alternative text. FocalPoint is x, y from -1.0 to 1.0, left out at
// the center; Width, Height, and Blurhash are left out when unknown.
type AttachmentDocument struct {
	Type       ObjectType `json:"type"`
	MediaType  string     `json:"mediaType"`
	URL        string     `json:"url"`
	Name       string     `json:"name,omitempty"`
	Blurhash   string     `json:"blurhash,omitempty"`
	FocalPoint []float64  `json:"focalPoint,omitempty"`
	Width      int        `json:"width,omitempty"`
	Height     int        `json:"height,omitempty"`
}

// TagType is the type of an entry of a Note's tag.
type TagType string

// Tag types.
const (
	TagMention TagType = "Mention"
	TagHashtag TagType = "Hashtag"
)

// TagDocument is an entry of a Note's tag: a Mention, whose Href is the actor and Name the
// handle "@user@domain", or a Hashtag, whose Href is the tag's page and Name "#name".
type TagDocument struct {
	Type TagType `json:"type"`
	Href string  `json:"href"`
	Name string  `json:"name"`
}

// TombstoneOf returns the object of the Delete of id.
func TombstoneOf(id IRI) ObjectDocument { return ObjectDocument{ID: id, Type: ObjectTombstone} }

func nonNil(s []IRI) []IRI {
	if s == nil {
		return []IRI{}
	}
	return s
}
