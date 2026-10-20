// SPDX-License-Identifier: AGPL-3.0-only

package instance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
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
	// Media checks the media files that media settings (preview_image) refer to; nil checks
	// only the shape of their IDs.
	Media MediaFiles
	// Accounts checks the local account that the contact_account setting names; nil checks
	// only the shape of its username.
	Accounts LocalAccounts
	// Documents renders the document texts (the terms of service and the privacy policy) to
	// HTML; nil publishes no documents.
	Documents DocumentRenderer
}

// DocumentRenderer renders the Markdown of a document text to an HTML fragment that is safe to
// insert into a page as it is. The adapter lives in internal/adapter/markdown and passes
// testkit.DocumentRendererContract.
type DocumentRenderer interface {
	// RenderDocument returns the HTML of the Markdown src, reduced to the document allowlist:
	// raw HTML is dropped, links keep only http, https, and mailto URLs, and the headings start
	// at <h2>, below the page's own <h1>.
	RenderDocument(src string) string
}

// MediaFiles checks media files for the media settings. The content module implements it.
type MediaFiles interface {
	// CheckImage fails when mediaID names no media file that is, or is being processed into,
	// a still image.
	CheckImage(ctx context.Context, mediaID string) error
}

// LocalAccounts checks the local accounts that account settings (contact_account) name. The
// account module implements it.
type LocalAccounts interface {
	// CheckLocalAccount fails when username names no local account.
	CheckLocalAccount(ctx context.Context, username string) error
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

// RegistrationMode returns the current value of the registration_mode setting.
func (s *Service) RegistrationMode(ctx context.Context) (instance.RegistrationMode, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return "", err
	}
	return settings.RegistrationMode(), nil
}

// MaxStatusLength returns the current value of the max_status_length setting (FR-101).
func (s *Service) MaxStatusLength(ctx context.Context) (int, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return 0, err
	}
	return settings.MaxStatusLength(), nil
}

// AuthorizedFetch returns the current value of the authorized_fetch setting (FR-408).
func (s *Service) AuthorizedFetch(ctx context.Context) (bool, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return false, err
	}
	return settings.AuthorizedFetch(), nil
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
// revision. The change is audited in the same transaction.
func (s *Service) SetSetting(ctx context.Context, in SetSettingInput) (instance.SettingRevision, error) {
	revs, err := s.SetSettings(ctx, []SetSettingInput{in})
	if err != nil {
		return instance.SettingRevision{}, err
	}
	return revs[0], nil
}

// SettingError reports the input of SetSettings that its key rejects. It wraps an error
// wrapping instance.ErrUnknownKey or instance.ErrInvalidValue.
type SettingError struct {
	Key instance.SettingKey
	Err error
}

func (e *SettingError) Error() string { return e.Err.Error() }

func (e *SettingError) Unwrap() error { return e.Err }

// SetSettings changes several settings at once, each as SetSetting changes one. It validates
// every value first and records nothing when one is rejected (a *SettingError naming the first
// such key); otherwise it records the revisions and their audit entries in one transaction, in
// the order of ins, so that a form of several settings is saved whole or not at all.
func (s *Service) SetSettings(ctx context.Context, ins []SetSettingInput) ([]instance.SettingRevision, error) {
	now := s.d.Clock.Now()
	revs := make([]instance.SettingRevision, 0, len(ins))
	audits := make([]appmoderation.AuditRecorded, 0, len(ins))
	for _, in := range ins {
		value, err := s.parseSetting(ctx, in)
		if err != nil {
			return nil, &SettingError{Key: in.Key, Err: err}
		}
		rev := instance.SettingRevision{ID: s.d.IDs.NewID(), Key: in.Key, Value: value, RevisedByUserID: in.RevisedByUserID, RevisedAt: now}
		revs = append(revs, rev)
		audits = append(audits, s.audit(appmoderation.AuditSettingChanged, appmoderation.AuditTargetSetting, string(in.Key), string(in.Key), rev.ID, in.RevisedByUserID, now))
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		for i, rev := range revs {
			if err := s.d.Repository.InsertSettingRevision(ctx, tx, rev); err != nil {
				return err
			}
			if err := tx.Emit(ctx, SettingRevised{Revision: rev}); err != nil {
				return err
			}
			if err := tx.Emit(ctx, audits[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return revs, nil
}

// parseSetting returns the canonical value of in (instance.Definition.Parse), with a locale
// canonicalized first and a media file checked. It fails with an error wrapping
// instance.ErrUnknownKey or instance.ErrInvalidValue.
func (s *Service) parseSetting(ctx context.Context, in SetSettingInput) (string, error) {
	def, err := instance.Lookup(in.Key)
	if err != nil {
		return "", err
	}
	raw := in.Value
	if def.Kind == instance.KindLocale {
		raw = i18n.Canonicalize(raw)
	}
	value, err := def.Parse(raw)
	if err != nil {
		return "", err
	}
	if def.Kind == instance.KindMedia && value != "" && s.d.Media != nil {
		if err := s.d.Media.CheckImage(ctx, value); err != nil {
			return "", fmt.Errorf("setting %s: media %s: %w: %w", in.Key, value, instance.ErrInvalidValue, err)
		}
	}
	if def.Kind == instance.KindAccount && value != "" && s.d.Accounts != nil {
		if err := s.d.Accounts.CheckLocalAccount(ctx, value); err != nil {
			return "", fmt.Errorf("setting %s: account %s: %w: %w", in.Key, value, instance.ErrInvalidValue, err)
		}
	}
	return value, nil
}

// IsMediaReferenced reports whether any revision of a media setting, current or not, refers
// to a media file. It implements content.MediaReferrer, so that the cleanup of unattached
// uploads keeps the instance's images and a setting can go back to an earlier revision.
func (s *Service) IsMediaReferenced(ctx context.Context, mediaID string) (bool, error) {
	if mediaID == "" {
		return false, nil // the cleared value
	}
	for _, key := range instance.MediaKeys() {
		used, err := s.d.Repository.IsSettingValueUsed(ctx, key, mediaID)
		if err != nil || used {
			return used, err
		}
	}
	return false, nil
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
// The locale is canonicalized first, and the change is audited in the same transaction. It fails
// with an error wrapping instance.ErrUnknownKey or instance.ErrInvalidValue for bad input.
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
	// The texts of a key in every locale are one target; the label names the locale.
	audit := s.audit(appmoderation.AuditTextChanged, appmoderation.AuditTargetSetting, string(in.Key),
		string(in.Key)+" ["+locale+"]", rev.ID, in.RevisedByUserID, rev.RevisedAt)
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertTextRevision(ctx, tx, rev); err != nil {
			return err
		}
		if err := tx.Emit(ctx, TextRevised{Revision: rev}); err != nil {
			return err
		}
		return tx.Emit(ctx, audit)
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

// Document is a document text rendered for a reader.
type Document struct {
	Key instance.TextKey
	// Locale is the locale of the text, which may differ from the reader's (the fallback chain).
	Locale string
	// HTML is the sanitized HTML of the text (DocumentRenderer).
	HTML      string
	RevisedAt time.Time
}

// Document returns the document text key (instance.TextDefinition.Document) rendered for a
// reader of locale, along the fallback chain of Text. It reports false when no locale of the
// chain has the text, and always without Deps.Documents. A key that is not a document is an
// error wrapping instance.ErrUnknownKey.
func (s *Service) Document(ctx context.Context, key instance.TextKey, locale string) (Document, bool, error) {
	def, err := instance.LookupText(key)
	if err != nil {
		return Document{}, false, err
	}
	if !def.Document {
		return Document{}, false, fmt.Errorf("text %q is not a document: %w", key, instance.ErrUnknownKey)
	}
	if s.d.Documents == nil {
		return Document{}, false, nil
	}
	t, ok, err := s.Text(ctx, key, locale)
	if err != nil || !ok {
		return Document{}, false, err
	}
	return Document{Key: key, Locale: t.Locale, HTML: s.d.Documents.RenderDocument(t.Body), RevisedAt: t.RevisedAt}, true, nil
}

// HasDocument reports whether a reader of locale finds the document text key: what Document
// reports, without rendering it.
func (s *Service) HasDocument(ctx context.Context, key instance.TextKey, locale string) (bool, error) {
	def, err := instance.LookupText(key)
	if err != nil {
		return false, err
	}
	if !def.Document {
		return false, fmt.Errorf("text %q is not a document: %w", key, instance.ErrUnknownKey)
	}
	if s.d.Documents == nil {
		return false, nil
	}
	_, ok, err := s.Text(ctx, key, locale)
	return ok, err
}

// Image is one of the instance's images, a media setting with its localized alt text.
type Image struct {
	// MediaID is the value of the setting, "" when unset.
	MediaID string
	// Alt is the alt text, "" when no locale of the chain has it.
	Alt string
}

// PreviewImage returns the instance's link preview image (ADR-0028): the preview_image setting
// with its alt text, the preview_image_alt text, for a reader of locale.
func (s *Service) PreviewImage(ctx context.Context, locale string) (Image, error) {
	return s.image(ctx, instance.SettingPreviewImage, instance.TextPreviewImageAlt, locale)
}

// Thumbnail returns the image that represents the instance: the thumbnail setting with its alt
// text, the thumbnail_alt text, for a reader of locale.
func (s *Service) Thumbnail(ctx context.Context, locale string) (Image, error) {
	return s.image(ctx, instance.SettingThumbnail, instance.TextThumbnailAlt, locale)
}

func (s *Service) image(ctx context.Context, key instance.SettingKey, alt instance.TextKey, locale string) (Image, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return Image{}, err
	}
	out := Image{}
	out.MediaID, _ = settings.Value(key)
	if out.MediaID == "" {
		return out, nil
	}
	t, ok, err := s.Text(ctx, alt, locale)
	if err != nil {
		return Image{}, err
	}
	if ok {
		out.Alt = t.Body
	}
	return out, nil
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

// audit returns the audit log entry of one of this module's operations (ADR-0037), with a new
// ID, performed by byUserID ("" for the CLI). The operation emits it in its transaction.
func (s *Service) audit(action appmoderation.AuditAction, kind appmoderation.AuditTargetKind, targetID, label, sourceID, byUserID string, at time.Time) appmoderation.AuditRecorded {
	actor, user := appmoderation.AuditActor(byUserID)
	return appmoderation.AuditRecorded{Entry: appmoderation.AuditEntry{
		ID: s.d.IDs.NewID(), ActorKind: actor, ActorUserID: user, Action: action, TargetKind: kind,
		TargetID: targetID, TargetLabel: label, SourceEventID: sourceID, OccurredAt: at,
	}}
}
