// SPDX-License-Identifier: AGPL-3.0-only

package instance

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// TextKey names a localized instance text (data-model §4.9, instance_text_revisions.key).
type TextKey string

// Text keys. Adding one means adding its definition below and its value to the CHECK
// constraint of instance_text_revisions.key in schema/<dialect>.sql.
const (
	TextName             TextKey = "name"
	TextShortDescription TextKey = "short_description"
	TextDescription      TextKey = "description"
	TextTerms            TextKey = "terms"
	TextPrivacyPolicy    TextKey = "privacy_policy"
	TextPreviewImageAlt  TextKey = "preview_image_alt"
)

// TextDefinition describes a text key.
type TextDefinition struct {
	Key TextKey
	// MaxLength bounds the body, in characters.
	MaxLength int
	// SingleLine keys reject line breaks.
	SingleLine bool
	Doc        string
}

var textDefinitions = []TextDefinition{
	{Key: TextName, MaxLength: 100, SingleLine: true, Doc: "Name of the instance."},
	{Key: TextShortDescription, MaxLength: 500, Doc: "One-paragraph summary of the instance."},
	{Key: TextDescription, MaxLength: 20000, Doc: "Full description of the instance."},
	{Key: TextTerms, MaxLength: 100000, Doc: "Terms of service."},
	{Key: TextPrivacyPolicy, MaxLength: 100000, Doc: "Privacy policy."},
	{Key: TextPreviewImageAlt, MaxLength: 1500, Doc: "Alt text of the preview_image setting's image (og:image:alt)."},
}

// TextDefinitions returns the definitions of every text key, in a fixed order.
func TextDefinitions() []TextDefinition { return slices.Clone(textDefinitions) }

// LookupText returns the definition of key, or an error wrapping ErrUnknownKey.
func LookupText(key TextKey) (TextDefinition, error) {
	for _, d := range textDefinitions {
		if d.Key == key {
			return d, nil
		}
	}
	return TextDefinition{}, fmt.Errorf("text %q: %w", key, ErrUnknownKey)
}

// Validate checks body against d. An empty body is valid: it clears the text in its locale,
// so lookups fall back to the next locale of the chain.
func (d TextDefinition) Validate(body string) error {
	invalid := func(why string) error { return fmt.Errorf("text %s: %s: %w", d.Key, why, ErrInvalidValue) }
	switch {
	case !utf8.ValidString(body):
		return invalid("not valid UTF-8")
	case strings.ContainsRune(body, 0):
		return invalid("contains NUL") // data-model §1.2
	case utf8.RuneCountInString(body) > d.MaxLength:
		return invalid(fmt.Sprintf("longer than %d characters", d.MaxLength))
	case d.SingleLine && strings.ContainsAny(body, "\r\n"):
		return invalid("contains a line break")
	}
	return nil
}

// ValidateLocale checks the shape of a canonical BCP 47 tag used as a text's locale.
func ValidateLocale(locale string) error {
	if !wellFormedTag(locale) {
		return fmt.Errorf("locale %q is not a BCP 47 language tag: %w", locale, ErrInvalidValue)
	}
	return nil
}

// TextRevision is one version of a text in one locale (instance_text_revisions). The
// revision with the greatest ID is the current text of its key and locale; an empty Body
// means the text is cleared in that locale.
type TextRevision struct {
	ID     string
	Key    TextKey
	Locale string
	Body   string
	// RevisedByUserID is the user who made the change, or "" for a change from the CLI.
	RevisedByUserID string
	RevisedAt       time.Time
}

// CurrentText points at the current revision of a text in one locale (the instance_texts
// projection).
type CurrentText struct {
	Key        TextKey
	Locale     string
	RevisionID string
}

// Supersedes reports whether r should replace the current text c, which is the zero value
// when the key has no text in r's locale yet.
func (r TextRevision) Supersedes(c CurrentText) bool {
	return c.RevisionID == "" || r.ID > c.RevisionID
}

// Text is the current, non-empty text of a key in one locale.
type Text struct {
	Key        TextKey
	Locale     string
	Body       string
	RevisionID string
	RevisedAt  time.Time
}

// PickText returns the text whose locale comes first in chain, among texts of one key.
// It reports false when no text matches a locale of the chain.
func PickText(texts []Text, chain []string) (Text, bool) {
	for _, l := range chain {
		for _, t := range texts {
			if t.Locale == l && t.Body != "" {
				return t, true
			}
		}
	}
	return Text{}, false
}
