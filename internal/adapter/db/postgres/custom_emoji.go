// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// CustomEmojiRepository implements content.CustomEmojiRepository on PostgreSQL.
type CustomEmojiRepository struct {
	db *DB
}

var _ appcontent.CustomEmojiRepository = (*CustomEmojiRepository)(nil)

// CustomEmojiRepository returns the content module's repository of the custom emoji that the
// administrators manage on d.
func (d *DB) CustomEmojiRepository() *CustomEmojiRepository { return &CustomEmojiRepository{db: d} }

// InsertCustomEmoji implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) InsertCustomEmoji(ctx context.Context, tx uow.Tx, e content.CustomEmoji) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertCustomEmoji(ctx, postgresgen.InsertCustomEmojiParams{
		ID: e.ID, Shortcode: e.Shortcode, Domain: e.Domain, Uri: toNullString(e.URI), CreatedAt: utc(e.CreatedAt),
	})
	if isUniqueViolation(err, "custom_emojis_shortcode_domain") {
		return appcontent.ErrShortcodeTaken
	}
	return wrap(err, "insert custom emoji")
}

// InsertCustomEmojiImage implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) InsertCustomEmojiImage(ctx context.Context, tx uow.Tx, img content.CustomEmojiImage) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertCustomEmojiImage(ctx, postgresgen.InsertCustomEmojiImageParams{
		ID: img.ID, EmojiID: img.EmojiID, MediaID: img.MediaID, RevisedAt: utc(img.RevisedAt),
	})
	return wrap(err, "insert custom emoji image")
}

// SetCustomEmojiSettings implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) SetCustomEmojiSettings(ctx context.Context, tx uow.Tx, s content.CustomEmojiSettings) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	if s.IsDefault() {
		return wrap(q.DeleteCustomEmojiSettings(ctx, s.EmojiID), "delete custom emoji settings")
	}
	err = q.UpsertCustomEmojiSettings(ctx, postgresgen.UpsertCustomEmojiSettingsParams{
		EmojiID: s.EmojiID, Enabled: s.Enabled, VisibleInPicker: s.VisibleInPicker, Category: toNullString(s.Category),
	})
	return wrap(err, "upsert custom emoji settings")
}

// SetCustomEmojiAliases implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) SetCustomEmojiAliases(ctx context.Context, tx uow.Tx, emojiID string, aliases []string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	if err := q.DeleteCustomEmojiAliases(ctx, emojiID); err != nil {
		return wrap(err, "delete custom emoji aliases")
	}
	for _, a := range aliases {
		if err := q.InsertCustomEmojiAlias(ctx, postgresgen.InsertCustomEmojiAliasParams{EmojiID: emojiID, Alias: a}); err != nil {
			return wrap(err, "insert custom emoji alias")
		}
	}
	return nil
}

// DeleteCustomEmoji implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) DeleteCustomEmoji(ctx context.Context, tx uow.Tx, emojiID string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	for _, del := range []func(context.Context, string) error{
		q.DeleteCustomEmojiAliases, q.DeleteCustomEmojiSettings, q.DeleteCustomEmojiImages, q.DeleteCustomEmojiRow,
	} {
		if err := del(ctx, emojiID); err != nil {
			return wrap(err, "delete custom emoji")
		}
	}
	return nil
}

// FindCustomEmojiRecord implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) FindCustomEmojiRecord(ctx context.Context, id string) (content.CustomEmojiRecord, error) {
	q := r.db.reader()
	row, err := q.GetCustomEmoji(ctx, id)
	if err != nil {
		return content.CustomEmojiRecord{}, notFound(err, appcontent.ErrNotFound, "custom emoji "+id)
	}
	img, err := q.GetLatestCustomEmojiImage(ctx, id)
	if err != nil {
		return content.CustomEmojiRecord{}, notFound(err, appcontent.ErrNotFound, "custom emoji image "+id)
	}
	out := content.CustomEmojiRecord{CurrentCustomEmoji: content.CurrentCustomEmoji{
		Emoji: customEmojiFrom(row), Image: customEmojiImageFrom(img), Settings: content.DefaultCustomEmojiSettings(id),
	}}
	switch s, err := q.GetCustomEmojiSettings(ctx, id); {
	case err == nil:
		out.Settings = content.CustomEmojiSettings{EmojiID: id, Enabled: s.Enabled, VisibleInPicker: s.VisibleInPicker, Category: fromNullString(s.Category)}
	case !errors.Is(err, pgx.ErrNoRows):
		return content.CustomEmojiRecord{}, wrap(err, "get custom emoji settings")
	}
	aliases, err := r.aliases(ctx, []string{id})
	if err != nil {
		return content.CustomEmojiRecord{}, err
	}
	out.Aliases = aliases[id]
	return out, nil
}

// aliases returns the aliases of the emoji with an ID in ids, keyed by ID.
func (r *CustomEmojiRepository) aliases(ctx context.Context, ids []string) (map[string][]string, error) {
	rows, err := r.db.reader().ListCustomEmojiAliases(ctx, ids)
	if err != nil {
		return nil, wrap(err, "list custom emoji aliases")
	}
	out := map[string][]string{}
	for _, row := range rows {
		out[row.EmojiID] = append(out[row.EmojiID], row.Alias)
	}
	return out, nil
}

// emojiRecordRow is a row of ListAdminCustomEmojis or ListLocalCustomEmojis.
type emojiRecordRow struct {
	ID              string
	Shortcode       string
	Domain          string
	Uri             *string //nolint:revive // the name of the generated rows, which convert to this type
	CreatedAt       time.Time
	ImageID         string
	ImageMediaID    string
	ImageRevisedAt  time.Time
	Enabled         bool
	VisibleInPicker bool
	Category        string
}

func (r *CustomEmojiRepository) records(ctx context.Context, rows []emojiRecordRow) ([]content.CustomEmojiRecord, error) {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	aliases, err := r.aliases(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]content.CustomEmojiRecord, len(rows))
	for i, row := range rows {
		out[i] = content.CustomEmojiRecord{
			CurrentCustomEmoji: content.CurrentCustomEmoji{
				Emoji: content.CustomEmoji{
					ID: row.ID, Shortcode: row.Shortcode, Domain: row.Domain, URI: fromNullString(row.Uri), CreatedAt: utc(row.CreatedAt),
				},
				Image: content.CustomEmojiImage{ID: row.ImageID, EmojiID: row.ID, MediaID: row.ImageMediaID, RevisedAt: utc(row.ImageRevisedAt)},
				Settings: content.CustomEmojiSettings{
					EmojiID: row.ID, Enabled: row.Enabled, VisibleInPicker: row.VisibleInPicker, Category: row.Category,
				},
			},
			Aliases: aliases[row.ID],
		}
	}
	return out, nil
}

// ListCustomEmojiRecords implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) ListCustomEmojiRecords(ctx context.Context, q appcontent.CustomEmojiQuery, limit int) ([]content.CustomEmojiRecord, error) {
	n, err := toInt32(limit, "limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.db.reader().ListAdminCustomEmojis(ctx, postgresgen.ListAdminCustomEmojisParams{
		Locality: int32(q.Locality), Domain: q.Domain, Shortcode: q.Shortcode, BeforeID: q.Before, RowLimit: n, //nolint:gosec // a locality is 0 to 2
	})
	if err != nil {
		return nil, wrap(err, "list custom emoji")
	}
	out := make([]emojiRecordRow, len(rows))
	for i, row := range rows {
		out[i] = emojiRecordRow(row)
	}
	return r.records(ctx, out)
}

// ListLocalCustomEmojis implements content.CustomEmojiRepository.
func (r *CustomEmojiRepository) ListLocalCustomEmojis(ctx context.Context, shortcodes []string) ([]content.CustomEmojiRecord, error) {
	params := postgresgen.ListLocalCustomEmojisParams{AllShortcodes: shortcodes == nil, Shortcodes: shortcodes}
	if params.Shortcodes == nil {
		params.Shortcodes = []string{}
	}
	rows, err := r.db.reader().ListLocalCustomEmojis(ctx, params)
	if err != nil {
		return nil, wrap(err, "list local custom emoji")
	}
	out := make([]emojiRecordRow, len(rows))
	for i, row := range rows {
		out[i] = emojiRecordRow(row)
	}
	return r.records(ctx, out)
}
