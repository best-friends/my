// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// The local custom emoji (ADR-0051): their registration and management by the administrators
// (the admin API, the CLI, and the admin screen), the list of the picker, and the emoji that the
// text of a local post or profile uses.

// ErrShortcodeTaken reports a new local emoji whose shortcode a local emoji has.
var ErrShortcodeTaken = errors.New("content: a custom emoji has the shortcode")

// CustomEmojiRepository is the persistence port of the custom emoji that the administrators
// manage (data-model §4.4). The database adapters implement it and pass
// testkit.CustomEmojiRepositoryContract. Lookups that find nothing return an error wrapping
// ErrNotFound.
type CustomEmojiRepository interface {
	// InsertCustomEmoji inserts an emoji; it fails with an error wrapping ErrShortcodeTaken when
	// an emoji of its domain has its shortcode.
	InsertCustomEmoji(ctx context.Context, tx uow.Tx, e content.CustomEmoji) error
	InsertCustomEmojiImage(ctx context.Context, tx uow.Tx, img content.CustomEmojiImage) error
	// SetCustomEmojiSettings records the settings of an emoji; the defaults delete its row.
	SetCustomEmojiSettings(ctx context.Context, tx uow.Tx, s content.CustomEmojiSettings) error
	// SetCustomEmojiAliases replaces the aliases of an emoji.
	SetCustomEmojiAliases(ctx context.Context, tx uow.Tx, emojiID string, aliases []string) error
	// DeleteCustomEmoji deletes an emoji with its images, settings, and aliases. The revisions
	// that used it keep their rows, which then refer to nothing.
	DeleteCustomEmoji(ctx context.Context, tx uow.Tx, emojiID string) error

	// FindCustomEmojiRecord returns an emoji with its current image, settings, and aliases.
	FindCustomEmojiRecord(ctx context.Context, id string) (content.CustomEmojiRecord, error)
	// ListCustomEmojiRecords returns up to limit emoji matching q, newest first.
	ListCustomEmojiRecords(ctx context.Context, q CustomEmojiQuery, limit int) ([]content.CustomEmojiRecord, error)
	// ListLocalCustomEmojis returns the local emoji with one of shortcodes, or every local emoji
	// for nil shortcodes, enabled or not, ordered by shortcode.
	ListLocalCustomEmojis(ctx context.Context, shortcodes []string) ([]content.CustomEmojiRecord, error)
}

// CustomEmojiLocality selects the local emoji, the remote ones, or both.
type CustomEmojiLocality int

// Localities of a CustomEmojiQuery.
const (
	AnyEmoji CustomEmojiLocality = iota
	LocalEmoji
	RemoteEmoji
)

// CustomEmojiQuery selects the custom emoji that the administrators list. Zero fields select
// every emoji.
type CustomEmojiQuery struct {
	Locality CustomEmojiLocality
	// Domain keeps the emoji of one remote domain.
	Domain string
	// Shortcode keeps the emoji whose shortcode contains it, ignoring case.
	Shortcode string
	// Before is the ID of the last emoji of the previous page.
	Before string
}

// Audited operations on custom emoji (ADR-0051 §5), named as the audit log names them.
const (
	CustomEmojiCreated       = "custom_emoji_created"
	CustomEmojiImageReplaced = "custom_emoji_image_replaced"
	CustomEmojiUpdated       = "custom_emoji_updated"
	CustomEmojiEnabled       = "custom_emoji_enabled"
	CustomEmojiDisabled      = "custom_emoji_disabled"
	CustomEmojiDeleted       = "custom_emoji_deleted"
)

// EventCustomEmojiAdministered is the event type of CustomEmojiAdministered.
const EventCustomEmojiAdministered = "content.custom_emoji_administered"

// CustomEmojiAdministered is emitted, in the transaction of the operation, for every operation
// of an administrator on a custom emoji. The moderation module writes its audit log entry from
// it (ADR-0037); the webhooks of ADR-0060 (custom_emoji.created, .updated, .deleted) are to
// follow it too. Recording a remote emoji from a received document emits nothing.
type CustomEmojiAdministered struct {
	// AuditID is the ID of the audit log entry, generated before the transaction so that a
	// retry writes the same entry.
	AuditID string
	// Action is one of CustomEmojiCreated and the other operations.
	Action string
	Emoji  content.CustomEmoji
	// SourceEventID is the ID of the row the operation wrote (the image), "" when it has none.
	SourceEventID string
	// ByUserID is the administrator, "" for the CLI.
	ByUserID string
	At       time.Time
}

// EventType implements uow.Event.
func (CustomEmojiAdministered) EventType() string { return EventCustomEmojiAdministered }

// CustomEmojiDeps are the ports a CustomEmojiService uses.
type CustomEmojiDeps struct {
	UnitOfWork uow.UnitOfWork
	Repository CustomEmojiRepository
	// Media reads the uploads that become images, and presents the images.
	Media *MediaService
	Clock kernel.Clock
	IDs   kernel.IDGenerator
	// MaxBytes is the largest image of a local emoji as uploaded (custom_emoji_max_bytes); zero
	// means content.DefaultCustomEmojiMaxBytes.
	MaxBytes int64
}

// CustomEmojiService runs the use cases of the custom emoji that the administrators manage
// (FR-111, FR-602, ADR-0051). Every operation is audited.
type CustomEmojiService struct {
	d CustomEmojiDeps
}

// NewCustomEmojiService returns a CustomEmojiService on d.
func NewCustomEmojiService(d CustomEmojiDeps) *CustomEmojiService {
	if d.MaxBytes <= 0 {
		d.MaxBytes = content.DefaultCustomEmojiMaxBytes
	}
	return &CustomEmojiService{d: d}
}

// MaxBytes returns the limit of the size of the image of a local emoji.
func (s *CustomEmojiService) MaxBytes() int64 { return s.d.MaxBytes }

// AdminCustomEmoji is a custom emoji as the administrators see it: the record and, when it can
// be shown, its image.
type AdminCustomEmoji struct {
	content.CustomEmojiRecord
	// View is the image, nil when it cannot be shown.
	View *CustomEmojiView
}

// CreateCustomEmojiInput describes a new local custom emoji.
type CreateCustomEmojiInput struct {
	Shortcode string
	// MediaID is an upload of OwnerAccountID that finished processing.
	MediaID         string
	OwnerAccountID  string
	Category        string
	Aliases         []string
	VisibleInPicker bool
	// ByUserID is the administrator, "" for the CLI.
	ByUserID string
}

// Create registers a local emoji (ADR-0051 §1, §2). It fails with a *content.CustomEmojiError
// for a shortcode, alias, category, or image that breaks a rule, or a shortcode that a local
// emoji has.
func (s *CustomEmojiService) Create(ctx context.Context, in CreateCustomEmojiInput) (AdminCustomEmoji, error) {
	if !content.ValidLocalShortcode(in.Shortcode) {
		return AdminCustomEmoji{}, &content.CustomEmojiError{Reason: content.EmojiInvalidShortcode, Value: in.Shortcode}
	}
	category, err := content.NormalizeCustomEmojiCategory(in.Category)
	if err != nil {
		return AdminCustomEmoji{}, err
	}
	aliases, err := content.NormalizeCustomEmojiAliases(in.Shortcode, in.Aliases)
	if err != nil {
		return AdminCustomEmoji{}, err
	}
	if err := s.checkImage(ctx, in.OwnerAccountID, in.MediaID); err != nil {
		return AdminCustomEmoji{}, err
	}
	now := s.d.Clock.Now()
	e := content.CustomEmoji{ID: s.d.IDs.NewID(), Shortcode: in.Shortcode, CreatedAt: now}
	img := content.CustomEmojiImage{ID: s.d.IDs.NewID(), EmojiID: e.ID, MediaID: in.MediaID, RevisedAt: now}
	settings := content.DefaultCustomEmojiSettings(e.ID)
	settings.VisibleInPicker, settings.Category = in.VisibleInPicker, category
	event := s.event(CustomEmojiCreated, e, img.ID, in.ByUserID, now)
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertCustomEmoji(ctx, tx, e); err != nil {
			return err
		}
		if err := s.d.Repository.InsertCustomEmojiImage(ctx, tx, img); err != nil {
			return err
		}
		if err := s.d.Repository.SetCustomEmojiSettings(ctx, tx, settings); err != nil {
			return err
		}
		if err := s.d.Repository.SetCustomEmojiAliases(ctx, tx, e.ID, aliases); err != nil {
			return err
		}
		return tx.Emit(ctx, event)
	})
	if errors.Is(err, ErrShortcodeTaken) {
		return AdminCustomEmoji{}, &content.CustomEmojiError{Reason: content.EmojiShortcodeTaken, Value: in.Shortcode}
	}
	if err != nil {
		return AdminCustomEmoji{}, fmt.Errorf("content: create custom emoji :%s:: %w", in.Shortcode, err)
	}
	return s.Get(ctx, e.ID)
}

// checkImage checks that mediaID is an upload of ownerAccountID that finished processing and
// qualifies as the image of a local emoji, and has it processed for the emoji's variants when it
// was processed for another purpose.
func (s *CustomEmojiService) checkImage(ctx context.Context, ownerAccountID, mediaID string) error {
	m, err := s.d.Media.Media(ctx, mediaID)
	if errors.Is(err, ErrNotFound) || err == nil && (m.Upload == nil || m.File.OwnerAccountID != ownerAccountID) {
		return &content.CustomEmojiError{Reason: content.EmojiMediaNotFound, Value: mediaID}
	}
	if err != nil {
		return err
	}
	if !m.Ready() {
		if m.Result != nil {
			// A failed file: its content was not an image the server can read.
			return &content.CustomEmojiError{Reason: content.EmojiUnsupportedFormat, Value: mediaID}
		}
		return &content.CustomEmojiError{Reason: content.EmojiMediaNotReady, Value: mediaID}
	}
	facts := content.CustomEmojiImageFacts{
		ContentType: m.Upload.ContentType, FileSize: m.Upload.FileSize, Width: m.Result.Width, Height: m.Result.Height,
	}
	if err := content.CheckCustomEmojiImage(facts, s.d.MaxBytes); err != nil {
		return err
	}
	return s.d.Media.RequirePurpose(ctx, m, content.PurposeEmoji)
}

// event returns the event of an operation on e, with a new audit log entry ID.
func (s *CustomEmojiService) event(action string, e content.CustomEmoji, source, byUserID string, at time.Time) CustomEmojiAdministered {
	return CustomEmojiAdministered{
		AuditID: s.d.IDs.NewID(), Action: action, Emoji: e, SourceEventID: source, ByUserID: byUserID, At: at,
	}
}

// Get returns a custom emoji, local or remote, with its image. It fails with ErrNotFound for an
// unknown ID.
func (s *CustomEmojiService) Get(ctx context.Context, id string) (AdminCustomEmoji, error) {
	r, err := s.d.Repository.FindCustomEmojiRecord(ctx, id)
	if err != nil {
		return AdminCustomEmoji{}, err
	}
	out, err := s.present(ctx, []content.CustomEmojiRecord{r})
	if err != nil {
		return AdminCustomEmoji{}, err
	}
	return out[0], nil
}

// List returns up to limit custom emoji matching q, newest first, with their images.
func (s *CustomEmojiService) List(ctx context.Context, q CustomEmojiQuery, limit int) ([]AdminCustomEmoji, error) {
	q.Shortcode = strings.ToLower(q.Shortcode)
	records, err := s.d.Repository.ListCustomEmojiRecords(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return s.present(ctx, records)
}

func (s *CustomEmojiService) present(ctx context.Context, records []content.CustomEmojiRecord) ([]AdminCustomEmoji, error) {
	current := make([]content.CurrentCustomEmoji, len(records))
	for i, r := range records {
		current[i] = r.CurrentCustomEmoji
	}
	views, err := s.d.Media.customEmojiViews(ctx, current)
	if err != nil {
		return nil, err
	}
	out := make([]AdminCustomEmoji, len(records))
	for i, r := range records {
		out[i] = AdminCustomEmoji{CustomEmojiRecord: r}
		if v, ok := views[r.Emoji.ID]; ok {
			out[i].View = &v
		}
	}
	return out, nil
}

// CustomEmojiUpdate changes the settings of a custom emoji; nil fields keep their value.
type CustomEmojiUpdate struct {
	Enabled         *bool
	VisibleInPicker *bool
	// Category is trimmed; "" removes it.
	Category *string
	Aliases  *[]string
	// ByUserID is the administrator, "" for the CLI.
	ByUserID string
}

// Update changes the settings of a custom emoji. A remote emoji can only be enabled and
// disabled; another change fails with a *content.CustomEmojiError (content.EmojiRemote). Each
// kind of change is audited: enabling, disabling, and the rest as one update. A change that
// changes nothing records nothing. It fails with ErrNotFound for an unknown ID.
func (s *CustomEmojiService) Update(ctx context.Context, id string, u CustomEmojiUpdate) (AdminCustomEmoji, error) {
	r, err := s.d.Repository.FindCustomEmojiRecord(ctx, id)
	if err != nil {
		return AdminCustomEmoji{}, err
	}
	if !r.Emoji.Local() && (u.VisibleInPicker != nil || u.Category != nil || u.Aliases != nil) {
		return AdminCustomEmoji{}, &content.CustomEmojiError{Reason: content.EmojiRemote, Value: id}
	}
	settings, aliases := r.Settings, r.Aliases
	if u.Enabled != nil {
		settings.Enabled = *u.Enabled
	}
	if u.VisibleInPicker != nil {
		settings.VisibleInPicker = *u.VisibleInPicker
	}
	if u.Category != nil {
		if settings.Category, err = content.NormalizeCustomEmojiCategory(*u.Category); err != nil {
			return AdminCustomEmoji{}, err
		}
	}
	if u.Aliases != nil {
		if aliases, err = content.NormalizeCustomEmojiAliases(r.Emoji.Shortcode, *u.Aliases); err != nil {
			return AdminCustomEmoji{}, err
		}
	}
	now := s.d.Clock.Now()
	var events []CustomEmojiAdministered
	if settings.Enabled != r.Settings.Enabled {
		action := CustomEmojiDisabled
		if settings.Enabled {
			action = CustomEmojiEnabled
		}
		events = append(events, s.event(action, r.Emoji, "", u.ByUserID, now))
	}
	aliasesChanged := !slices.Equal(aliases, r.Aliases)
	if settings.VisibleInPicker != r.Settings.VisibleInPicker || settings.Category != r.Settings.Category || aliasesChanged {
		events = append(events, s.event(CustomEmojiUpdated, r.Emoji, "", u.ByUserID, now))
	}
	if len(events) == 0 {
		return s.Get(ctx, id)
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.SetCustomEmojiSettings(ctx, tx, settings); err != nil {
			return err
		}
		if aliasesChanged {
			if err := s.d.Repository.SetCustomEmojiAliases(ctx, tx, id, aliases); err != nil {
				return err
			}
		}
		for _, e := range events {
			if err := tx.Emit(ctx, e); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return AdminCustomEmoji{}, fmt.Errorf("content: update custom emoji %s: %w", id, err)
	}
	return s.Get(ctx, id)
}

// ReplaceImage makes an upload of ownerAccountID the new image of a local emoji, checked as the
// image of a new one; the emoji's updated time changes, so that the servers that have it fetch
// it again. A remote emoji fails with a *content.CustomEmojiError (content.EmojiRemote). It
// fails with ErrNotFound for an unknown ID.
func (s *CustomEmojiService) ReplaceImage(ctx context.Context, id, ownerAccountID, mediaID, byUserID string) (AdminCustomEmoji, error) {
	r, err := s.d.Repository.FindCustomEmojiRecord(ctx, id)
	if err != nil {
		return AdminCustomEmoji{}, err
	}
	if !r.Emoji.Local() {
		return AdminCustomEmoji{}, &content.CustomEmojiError{Reason: content.EmojiRemote, Value: id}
	}
	if err := s.checkImage(ctx, ownerAccountID, mediaID); err != nil {
		return AdminCustomEmoji{}, err
	}
	now := s.d.Clock.Now()
	img := content.CustomEmojiImage{ID: s.d.IDs.NewID(), EmojiID: id, MediaID: mediaID, RevisedAt: now}
	event := s.event(CustomEmojiImageReplaced, r.Emoji, img.ID, byUserID, now)
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertCustomEmojiImage(ctx, tx, img); err != nil {
			return err
		}
		return tx.Emit(ctx, event)
	})
	if err != nil {
		return AdminCustomEmoji{}, fmt.Errorf("content: replace the image of custom emoji %s: %w", id, err)
	}
	return s.Get(ctx, id)
}

// Delete deletes a local emoji with its images, settings, and aliases (ADR-0051 §1). The
// revisions that used it keep their references, which then show :shortcode: as text; the media
// files of its images become unattached uploads, which the cleanup of unattached uploads deletes
// with their blobs. A remote emoji fails with a *content.CustomEmojiError (content.EmojiRemote).
// It fails with ErrNotFound for an unknown ID.
func (s *CustomEmojiService) Delete(ctx context.Context, id, byUserID string) error {
	r, err := s.d.Repository.FindCustomEmojiRecord(ctx, id)
	if err != nil {
		return err
	}
	if !r.Emoji.Local() {
		return &content.CustomEmojiError{Reason: content.EmojiRemote, Value: id}
	}
	event := s.event(CustomEmojiDeleted, r.Emoji, "", byUserID, s.d.Clock.Now())
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.DeleteCustomEmoji(ctx, tx, id); err != nil {
			return err
		}
		return tx.Emit(ctx, event)
	})
	if err != nil {
		return fmt.Errorf("content: delete custom emoji %s: %w", id, err)
	}
	return nil
}

// PickerCustomEmoji is a local custom emoji that the picker offers.
type PickerCustomEmoji struct {
	CustomEmojiView
	Category string
	Aliases  []string
}

// Picker returns the custom emoji that the picker offers (ADR-0051 §4): the enabled local emoji
// in the picker whose image can be shown, by category (those without one first) and shortcode.
func (s *CustomEmojiService) Picker(ctx context.Context) ([]PickerCustomEmoji, error) {
	records, err := s.d.Repository.ListLocalCustomEmojis(ctx, nil)
	if err != nil {
		return nil, err
	}
	records = slices.DeleteFunc(records, func(r content.CustomEmojiRecord) bool {
		return !r.Settings.Enabled || !r.Settings.VisibleInPicker
	})
	shown, err := s.present(ctx, records)
	if err != nil {
		return nil, err
	}
	out := []PickerCustomEmoji{}
	for _, e := range shown {
		if e.View != nil {
			out = append(out, PickerCustomEmoji{CustomEmojiView: *e.View, Category: e.Settings.Category, Aliases: e.Aliases})
		}
	}
	slices.SortFunc(out, func(a, b PickerCustomEmoji) int {
		return cmp.Or(strings.Compare(a.Category, b.Category), strings.Compare(a.Shortcode, b.Shortcode))
	})
	return out, nil
}

// LocalEmojiIDs returns the IDs, sorted, of the enabled local emoji that texts use as
// :shortcode: (ADR-0051 §3): the emoji that a local post or profile records with its revision.
// A shortcode of no enabled local emoji stays text.
func (s *CustomEmojiService) LocalEmojiIDs(ctx context.Context, texts ...string) ([]string, error) {
	codes := content.ScanShortcodes(texts...)
	if len(codes) == 0 {
		return nil, nil
	}
	records, err := s.d.Repository.ListLocalCustomEmojis(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("content: the custom emoji %v: %w", codes, err)
	}
	var ids []string
	for _, r := range records {
		if r.Settings.Enabled && slices.Contains(codes, r.Emoji.Shortcode) {
			ids = append(ids, r.Emoji.ID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}
