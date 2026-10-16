// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"fmt"
	"slices"
	"time"

	"golang.org/x/text/language"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/i18n"
)

// PreferencesRepository is the persistence of user preferences (data-model §4.1,
// user_preferences and user_timeline_languages, current values). It is part of Repository.
type PreferencesRepository interface {
	// InsertPreferences stores the preferences of a user who has none yet.
	InsertPreferences(ctx context.Context, tx uow.Tx, p identity.Preferences) error
	// UpdatePreferences replaces the stored preferences of p.UserID. It fails with an error
	// wrapping ErrNotFound when the user has none.
	UpdatePreferences(ctx context.Context, tx uow.Tx, p identity.Preferences) error
	// ReplaceTimelineLanguages makes languages the user's timeline languages; an empty list
	// removes them all.
	ReplaceTimelineLanguages(ctx context.Context, tx uow.Tx, userID string, languages []string) error

	// FindPreferences returns the preferences of a user.
	FindPreferences(ctx context.Context, userID string) (identity.Preferences, error)
	// ListTimelineLanguages returns the timeline languages of a user, sorted; none means all.
	ListTimelineLanguages(ctx context.Context, userID string) ([]string, error)
}

// UserPreferences are a user's preferences with the languages their timelines show.
type UserPreferences struct {
	identity.Preferences
	// TimelineLanguages are the languages timelines show, sorted; empty shows every language (FR-208).
	TimelineLanguages []string
}

// PreferencesInput are the settable preferences of SetPreferences; every field is required.
type PreferencesInput struct {
	Locale                  string
	TimeZone                string
	DefaultVisibility       identity.Visibility
	DefaultPostLanguage     string
	RequireMediaDescription bool
}

// Preferences returns the preferences of a user. It fails with an error wrapping ErrNotFound
// when the user has none.
func (s *Service) Preferences(ctx context.Context, userID string) (UserPreferences, error) {
	p, err := s.d.Repository.FindPreferences(ctx, userID)
	if err != nil {
		return UserPreferences{}, err
	}
	langs, err := s.d.Repository.ListTimelineLanguages(ctx, userID)
	if err != nil {
		return UserPreferences{}, err
	}
	slices.Sort(langs) // byte order, whatever the database's collation
	return UserPreferences{Preferences: p, TimelineLanguages: langs}, nil
}

// SetPreferences validates in, canonicalizes its language tags, and replaces the preferences of
// a user. It fails with errors wrapping identity.ErrInvalidLanguageTag,
// identity.ErrInvalidTimeZone, or identity.ErrInvalidVisibility for invalid input, and with
// ErrNotFound when the user has no preferences.
func (s *Service) SetPreferences(ctx context.Context, userID string, in PreferencesInput) (identity.Preferences, error) {
	locale, err := CanonicalLocale(in.Locale)
	if err != nil {
		return identity.Preferences{}, err
	}
	postLang, err := CanonicalLanguage(in.DefaultPostLanguage)
	if err != nil {
		return identity.Preferences{}, err
	}
	if err := identity.ValidateTimeZone(in.TimeZone); err != nil {
		return identity.Preferences{}, err
	}
	if err := identity.ValidateVisibility(in.DefaultVisibility); err != nil {
		return identity.Preferences{}, err
	}
	p := identity.Preferences{
		UserID:                  userID,
		Locale:                  locale,
		TimeZone:                in.TimeZone,
		DefaultVisibility:       in.DefaultVisibility,
		DefaultPostLanguage:     postLang,
		RequireMediaDescription: in.RequireMediaDescription,
		UpdatedAt:               s.d.Clock.Now(),
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.d.Repository.UpdatePreferences(ctx, tx, p) })
	if err != nil {
		return identity.Preferences{}, err
	}
	return p, nil
}

// SetTimelineLanguages replaces the languages a user's timelines show with languages,
// canonicalized, deduplicated, and sorted, and returns them; an empty list shows every language
// (FR-208). It fails with errors wrapping identity.ErrInvalidLanguageTag or
// identity.ErrTooManyLanguages for invalid input, and with ErrNotFound when the user has no
// preferences.
func (s *Service) SetTimelineLanguages(ctx context.Context, userID string, languages []string) ([]string, error) {
	if len(languages) > identity.MaxTimelineLanguageCount {
		return nil, fmt.Errorf("%w: at most %d", identity.ErrTooManyLanguages, identity.MaxTimelineLanguageCount)
	}
	out := make([]string, 0, len(languages))
	for _, l := range languages {
		c, err := CanonicalLanguage(l)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if _, err := s.d.Repository.FindPreferences(ctx, userID); err != nil {
		return nil, err
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return s.d.Repository.ReplaceTimelineLanguages(ctx, tx, userID, out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CanonicalLanguage returns the canonical form of a BCP 47 language tag ("ja-jp" -> "ja-JP",
// "iw" -> "he"). It accepts identity.UndeterminedLanguage and rejects tags that are malformed,
// that use unknown subtags, or that name no language explicitly (such as "und-JP" or private-use "x-foo"), with
// an error wrapping identity.ErrInvalidLanguageTag.
func CanonicalLanguage(tag string) (string, error) {
	if len(tag) > identity.MaxLanguageTagLength {
		return "", fmt.Errorf("%w: longer than %d bytes", identity.ErrInvalidLanguageTag, identity.MaxLanguageTagLength)
	}
	t, err := language.Parse(tag)
	if err != nil {
		return "", fmt.Errorf("%w: %q", identity.ErrInvalidLanguageTag, tag)
	}
	c := t.String()
	// Base guesses a language for "und-JP" or "x-foo"; only an explicit one is Exact.
	if _, conf := t.Base(); conf != language.Exact && c != identity.UndeterminedLanguage {
		return "", fmt.Errorf("%w: %q has no language subtag", identity.ErrInvalidLanguageTag, tag)
	}
	return c, nil
}

// CanonicalLocale is CanonicalLanguage for a UI locale, which also rejects
// identity.UndeterminedLanguage.
func CanonicalLocale(tag string) (string, error) {
	c, err := CanonicalLanguage(tag)
	if err != nil {
		return "", err
	}
	if c == identity.UndeterminedLanguage {
		return "", fmt.Errorf("%w: a locale must name a language", identity.ErrInvalidLanguageTag)
	}
	return c, nil
}

// defaultPreferences returns the preferences of a new user: the registration's locale or else
// the instance default, UTC, and public posts in an undetermined language.
func (s *Service) defaultPreferences(ctx context.Context, userID, locale string, now time.Time) (identity.Preferences, error) {
	if locale == "" && s.d.DefaultLocale != nil {
		var err error
		if locale, err = s.d.DefaultLocale(ctx); err != nil {
			return identity.Preferences{}, fmt.Errorf("instance default locale: %w", err)
		}
	}
	if locale == "" {
		locale = i18n.SourceLocale
	}
	locale, err := CanonicalLocale(locale)
	if err != nil {
		return identity.Preferences{}, err
	}
	return identity.Preferences{
		UserID:              userID,
		Locale:              locale,
		TimeZone:            identity.DefaultTimeZone,
		DefaultVisibility:   identity.DefaultPostVisibility,
		DefaultPostLanguage: identity.UndeterminedLanguage,
		UpdatedAt:           now,
	}, nil
}
