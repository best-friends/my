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
// and addressing. A Note's body (content, contentMap, summary, sensitive, tag, attachment) is
// added by the builder of status documents.
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

// TombstoneOf returns the object of the Delete of id.
func TombstoneOf(id IRI) ObjectDocument { return ObjectDocument{ID: id, Type: ObjectTombstone} }

func nonNil(s []IRI) []IRI {
	if s == nil {
		return []IRI{}
	}
	return s
}
