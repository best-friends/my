// SPDX-License-Identifier: AGPL-3.0-only

package instance

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/i18n"
)

// Deps are the ports a Service uses.
type Deps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	Clock      kernel.Clock
	IDs        kernel.IDGenerator
}

// Service runs the instance module's use cases for settings and texts.
//
// Settings are the instance policies that administrators change at run time; they live in the
// database, versioned. Deployment configuration (adapters, host, secrets) lives in the
// environment and the config file (internal/platform/config, ADR-0010). The two never share a
// key, so neither overrides the other (docs/reference/configuration.md).
type Service struct {
	d Deps
}

// NewService returns a Service on d.
func NewService(d Deps) *Service { return &Service{d: d} }

// Settings returns the current value of every setting, defaults included.
func (s *Service) Settings(ctx context.Context) (instance.Settings, error) {
	cur, err := s.d.Repository.ListCurrentSettings(ctx)
	if err != nil {
		return instance.Settings{}, err
	}
	return instance.NewSettings(cur), nil
}

// DefaultLocale returns the current value of the default_locale setting.
func (s *Service) DefaultLocale(ctx context.Context) (string, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return "", err
	}
	return settings.DefaultLocale(), nil
}

// MaxStatusLength returns the current value of the max_status_length setting (FR-101).
func (s *Service) MaxStatusLength(ctx context.Context) (int, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return 0, err
	}
	return settings.MaxStatusLength(), nil
}

// SetSettingInput describes a change of a setting.
type SetSettingInput struct {
	Key   instance.SettingKey
	Value string
	// RevisedByUserID is the user making the change, or "" for the CLI.
	RevisedByUserID string
}

// SetSetting validates the value against the key's definition and records it as a new
// revision, which becomes the current value. A locale value is canonicalized first
// ("ja_jp" becomes "ja-JP"). It fails with an error wrapping instance.ErrUnknownKey or
// instance.ErrInvalidValue for bad input. Setting the current value again still records a
// revision.
func (s *Service) SetSetting(ctx context.Context, in SetSettingInput) (instance.SettingRevision, error) {
	def, err := instance.Lookup(in.Key)
	if err != nil {
		return instance.SettingRevision{}, err
	}
	raw := in.Value
	if def.Kind == instance.KindLocale {
		raw = i18n.Canonicalize(raw)
	}
	value, err := def.Parse(raw)
	if err != nil {
		return instance.SettingRevision{}, err
	}
	rev := instance.SettingRevision{
		ID: s.d.IDs.NewID(), Key: in.Key, Value: value, RevisedByUserID: in.RevisedByUserID, RevisedAt: s.d.Clock.Now(),
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertSettingRevision(ctx, tx, rev); err != nil {
			return err
		}
		return tx.Emit(ctx, SettingRevised{Revision: rev})
	})
	if err != nil {
		return instance.SettingRevision{}, err
	}
	return rev, nil
}

// SettingHistory returns the revisions of a setting, newest first, at most limit of them.
func (s *Service) SettingHistory(ctx context.Context, key instance.SettingKey, limit int) ([]instance.SettingRevision, error) {
	if _, err := instance.Lookup(key); err != nil {
		return nil, err
	}
	if err := checkLimit(limit); err != nil {
		return nil, err
	}
	return s.d.Repository.ListSettingHistory(ctx, key, limit)
}

// SetTextInput describes a change of a text in one locale.
type SetTextInput struct {
	Key    instance.TextKey
	Locale string
	// Body is the new text; "" clears the text in Locale.
	Body string
	// RevisedByUserID is the user making the change, or "" for the CLI.
	RevisedByUserID string
}

// SetText records a new revision of a text in one locale, which becomes its current text.
// The locale is canonicalized first. It fails with an error wrapping instance.ErrUnknownKey or
// instance.ErrInvalidValue for bad input.
func (s *Service) SetText(ctx context.Context, in SetTextInput) (instance.TextRevision, error) {
	def, err := instance.LookupText(in.Key)
	if err != nil {
		return instance.TextRevision{}, err
	}
	locale := i18n.Canonicalize(in.Locale)
	if err := instance.ValidateLocale(locale); err != nil {
		return instance.TextRevision{}, err
	}
	if err := def.Validate(in.Body); err != nil {
		return instance.TextRevision{}, err
	}
	rev := instance.TextRevision{
		ID: s.d.IDs.NewID(), Key: in.Key, Locale: locale, Body: in.Body,
		RevisedByUserID: in.RevisedByUserID, RevisedAt: s.d.Clock.Now(),
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertTextRevision(ctx, tx, rev); err != nil {
			return err
		}
		return tx.Emit(ctx, TextRevised{Revision: rev})
	})
	if err != nil {
		return instance.TextRevision{}, err
	}
	return rev, nil
}

// Text returns the text of key for a reader of locale, following the locale fallback chain
// (ADR-0009): locale and its parents, then the instance's default locale and its parents, then
// i18n.SourceLocale ("ja-JP" -> "ja" -> default -> "en"). A locale whose text is cleared is
// skipped. It reports false when no locale of the chain has the text.
func (s *Service) Text(ctx context.Context, key instance.TextKey, locale string) (instance.Text, bool, error) {
	if _, err := instance.LookupText(key); err != nil {
		return instance.Text{}, false, err
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return instance.Text{}, false, err
	}
	texts, err := s.d.Repository.ListCurrentTexts(ctx, key)
	if err != nil {
		return instance.Text{}, false, err
	}
	t, ok := instance.PickText(texts, TextFallbackChain(locale, settings.DefaultLocale()))
	return t, ok, nil
}

// TextFallbackChain returns the locales consulted for an instance text read in locale, most
// specific first: i18n.FallbackChain(locale), without the source locale it appends when
// locale is not a variant of it, then i18n.FallbackChain(defaultLocale), without duplicates.
func TextFallbackChain(locale, defaultLocale string) []string {
	var chain []string
	if canonical := i18n.Canonicalize(locale); canonical != "" {
		chain = i18n.FallbackChain(canonical)
		if lang, _, _ := strings.Cut(canonical, "-"); lang != i18n.SourceLocale {
			chain = slices.DeleteFunc(chain, func(l string) bool { return l == i18n.SourceLocale })
		}
	}
	for _, l := range i18n.FallbackChain(defaultLocale) {
		if !slices.Contains(chain, l) {
			chain = append(chain, l)
		}
	}
	return chain
}

// Texts returns every current, non-empty text, by key and locale.
func (s *Service) Texts(ctx context.Context) ([]instance.Text, error) {
	all, err := s.d.Repository.ListAllCurrentTexts(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(t instance.Text) bool { return t.Body == "" }), nil
}

// TextHistory returns the revisions of a text in one locale, newest first, at most limit of them.
func (s *Service) TextHistory(ctx context.Context, key instance.TextKey, locale string, limit int) ([]instance.TextRevision, error) {
	if _, err := instance.LookupText(key); err != nil {
		return nil, err
	}
	if err := checkLimit(limit); err != nil {
		return nil, err
	}
	canonical := i18n.Canonicalize(locale)
	if err := instance.ValidateLocale(canonical); err != nil {
		return nil, fmt.Errorf("text history: %w", err)
	}
	return s.d.Repository.ListTextHistory(ctx, key, canonical, limit)
}

// MaxHistoryLimit bounds the limit of SettingHistory and TextHistory.
const MaxHistoryLimit = 1000

func checkLimit(limit int) error {
	if limit < 1 || limit > MaxHistoryLimit {
		return fmt.Errorf("history limit %d is outside 1..%d: %w", limit, MaxHistoryLimit, instance.ErrInvalidValue)
	}
	return nil
}
