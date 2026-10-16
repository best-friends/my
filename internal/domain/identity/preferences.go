// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"errors"
	"fmt"
	"time"
)

// Preferences are a user's settings (data-model §4.1, user_preferences; FR-007). Language tags
// are canonical BCP 47 tags and TimeZone is an IANA time zone name.
type Preferences struct {
	UserID string
	// Locale is the UI language. It is never UndeterminedLanguage.
	Locale string
	// TimeZone is the zone dates are shown in, such as "Asia/Tokyo" or "UTC".
	TimeZone string
	// DefaultVisibility is the visibility a new post starts with.
	DefaultVisibility Visibility
	// DefaultPostLanguage is the language a new post starts with; UndeterminedLanguage leaves it
	// to the client.
	DefaultPostLanguage string
	// RequireMediaDescription makes clients ask for a description before attaching media.
	RequireMediaDescription bool
	UpdatedAt               time.Time
}

// Defaults of a new user's preferences (data-model §4.1). The locale comes from the registration
// or the instance default.
const (
	DefaultTimeZone          = "UTC"
	DefaultPostVisibility    = VisibilityPublic
	UndeterminedLanguage     = "und" // BCP 47 "undetermined" (data-model §1.2, lang)
	MaxLanguageTagLength     = 64    // longer tags are rejected before parsing
	MaxTimeZoneLength        = 64
	MaxTimelineLanguageCount = 64
)

// Visibility is the audience of a post (ADR-0017). Preferences use it for the default of new posts.
type Visibility string

// The visibility levels, widest first.
const (
	VisibilityPublic   Visibility = "public"
	VisibilityLocal    Visibility = "local"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPrivate  Visibility = "private"
	VisibilityDirect   Visibility = "direct"
)

// Errors that preference validation reports.
var (
	ErrInvalidLanguageTag = errors.New("invalid language tag")
	ErrInvalidTimeZone    = errors.New("invalid time zone")
	ErrInvalidVisibility  = errors.New("invalid visibility")
	ErrTooManyLanguages   = errors.New("too many timeline languages")
)

// ValidateVisibility accepts the five visibility levels.
func ValidateVisibility(v Visibility) error {
	switch v {
	case VisibilityPublic, VisibilityLocal, VisibilityUnlisted, VisibilityPrivate, VisibilityDirect:
		return nil
	}
	return fmt.Errorf("%w: %q", ErrInvalidVisibility, v)
}

// ValidateTimeZone accepts an IANA time zone name that the time zone database knows, such as
// "Asia/Tokyo" or "UTC". It rejects "" and "Local", which time.LoadLocation maps to UTC and to
// the host's zone. The server binary embeds the database (time/tzdata), so the result does not
// depend on the host.
func ValidateTimeZone(name string) error {
	if name == "" || name == "Local" || len(name) > MaxTimeZoneLength {
		return fmt.Errorf("%w: %q", ErrInvalidTimeZone, name)
	}
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidTimeZone, name)
	}
	return nil
}
