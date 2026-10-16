// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/instance"
)

// InstanceRepository implements instance.Repository on PostgreSQL.
type InstanceRepository struct {
	db *DB
}

var _ appinstance.Repository = (*InstanceRepository)(nil)

// InstanceRepository returns the instance module's repository on d.
func (d *DB) InstanceRepository() *InstanceRepository { return &InstanceRepository{db: d} }

// InsertSettingRevision implements instance.Repository.
func (r *InstanceRepository) InsertSettingRevision(ctx context.Context, tx uow.Tx, rev instance.SettingRevision) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertInstanceSettingRevision(ctx, postgresgen.InsertInstanceSettingRevisionParams{
		ID: rev.ID, Key: string(rev.Key), Value: rev.Value,
		RevisedByUserID: toNullString(rev.RevisedByUserID), RevisedAt: rev.RevisedAt,
	})
	return wrap(err, "insert instance setting revision")
}

// CurrentSetting implements instance.Repository.
func (r *InstanceRepository) CurrentSetting(ctx context.Context, tx uow.Tx, key instance.SettingKey) (instance.CurrentSetting, error) {
	q, err := queries(tx)
	if err != nil {
		return instance.CurrentSetting{}, err
	}
	row, err := q.GetInstanceSetting(ctx, string(key))
	if err != nil {
		return instance.CurrentSetting{}, notFound(err, appinstance.ErrNotFound, "instance setting "+string(key))
	}
	return currentSetting(row), nil
}

// UpsertCurrentSetting implements instance.Repository.
func (r *InstanceRepository) UpsertCurrentSetting(ctx context.Context, tx uow.Tx, c instance.CurrentSetting) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.UpsertInstanceSetting(ctx, postgresgen.UpsertInstanceSettingParams{Key: string(c.Key), RevisionID: c.RevisionID, Value: c.Value})
	return wrap(err, "upsert instance setting")
}

// DeleteAllCurrentSettings implements instance.Repository.
func (r *InstanceRepository) DeleteAllCurrentSettings(ctx context.Context, tx uow.Tx) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.DeleteAllInstanceSettings(ctx), "delete instance settings")
}

// ListSettingRevisions implements instance.Repository.
func (r *InstanceRepository) ListSettingRevisions(ctx context.Context, tx uow.Tx) ([]instance.SettingRevision, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListInstanceSettingRevisions(ctx)
	if err != nil {
		return nil, wrap(err, "list instance setting revisions")
	}
	return settingRevisions(rows), nil
}

// InsertTextRevision implements instance.Repository.
func (r *InstanceRepository) InsertTextRevision(ctx context.Context, tx uow.Tx, rev instance.TextRevision) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertInstanceTextRevision(ctx, postgresgen.InsertInstanceTextRevisionParams{
		ID: rev.ID, Key: string(rev.Key), Locale: rev.Locale, Body: rev.Body,
		RevisedByUserID: toNullString(rev.RevisedByUserID), RevisedAt: rev.RevisedAt,
	})
	return wrap(err, "insert instance text revision")
}

// CurrentText implements instance.Repository.
func (r *InstanceRepository) CurrentText(ctx context.Context, tx uow.Tx, key instance.TextKey, locale string) (instance.CurrentText, error) {
	q, err := queries(tx)
	if err != nil {
		return instance.CurrentText{}, err
	}
	row, err := q.GetInstanceText(ctx, postgresgen.GetInstanceTextParams{Key: string(key), Locale: locale})
	if err != nil {
		return instance.CurrentText{}, notFound(err, appinstance.ErrNotFound, "instance text "+string(key)+" in "+locale)
	}
	return instance.CurrentText{Key: instance.TextKey(row.Key), Locale: row.Locale, RevisionID: row.RevisionID}, nil
}

// UpsertCurrentText implements instance.Repository.
func (r *InstanceRepository) UpsertCurrentText(ctx context.Context, tx uow.Tx, c instance.CurrentText) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.UpsertInstanceText(ctx, postgresgen.UpsertInstanceTextParams{Key: string(c.Key), Locale: c.Locale, RevisionID: c.RevisionID})
	return wrap(err, "upsert instance text")
}

// DeleteAllCurrentTexts implements instance.Repository.
func (r *InstanceRepository) DeleteAllCurrentTexts(ctx context.Context, tx uow.Tx) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.DeleteAllInstanceTexts(ctx), "delete instance texts")
}

// ListTextRevisionHeads implements instance.Repository.
func (r *InstanceRepository) ListTextRevisionHeads(ctx context.Context, tx uow.Tx) ([]instance.CurrentText, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListInstanceTextRevisionHeads(ctx)
	if err != nil {
		return nil, wrap(err, "list instance text revisions")
	}
	out := make([]instance.CurrentText, len(rows))
	for i, row := range rows {
		out[i] = instance.CurrentText{Key: instance.TextKey(row.Key), Locale: row.Locale, RevisionID: row.ID}
	}
	return out, nil
}

// ListCurrentSettings implements instance.Repository.
func (r *InstanceRepository) ListCurrentSettings(ctx context.Context) ([]instance.CurrentSetting, error) {
	rows, err := r.db.reader().ListInstanceSettings(ctx)
	if err != nil {
		return nil, wrap(err, "list instance settings")
	}
	out := make([]instance.CurrentSetting, len(rows))
	for i, row := range rows {
		out[i] = currentSetting(row)
	}
	return out, nil
}

// ListSettingHistory implements instance.Repository.
func (r *InstanceRepository) ListSettingHistory(ctx context.Context, key instance.SettingKey, limit int) ([]instance.SettingRevision, error) {
	n, err := toInt32(limit, "limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.db.reader().ListInstanceSettingHistory(ctx, postgresgen.ListInstanceSettingHistoryParams{Key: string(key), Limit: n})
	if err != nil {
		return nil, wrap(err, "list instance setting history")
	}
	return settingRevisions(rows), nil
}

// ListCurrentTexts implements instance.Repository.
func (r *InstanceRepository) ListCurrentTexts(ctx context.Context, key instance.TextKey) ([]instance.Text, error) {
	rows, err := r.db.reader().ListCurrentInstanceTexts(ctx, string(key))
	if err != nil {
		return nil, wrap(err, "list instance texts")
	}
	out := make([]instance.Text, len(rows))
	for i, row := range rows {
		out[i] = currentText(postgresgen.ListAllCurrentInstanceTextsRow(row))
	}
	return out, nil
}

// ListAllCurrentTexts implements instance.Repository.
func (r *InstanceRepository) ListAllCurrentTexts(ctx context.Context) ([]instance.Text, error) {
	rows, err := r.db.reader().ListAllCurrentInstanceTexts(ctx)
	if err != nil {
		return nil, wrap(err, "list instance texts")
	}
	out := make([]instance.Text, len(rows))
	for i, row := range rows {
		out[i] = currentText(row)
	}
	return out, nil
}

// ListTextHistory implements instance.Repository.
func (r *InstanceRepository) ListTextHistory(ctx context.Context, key instance.TextKey, locale string, limit int) ([]instance.TextRevision, error) {
	n, err := toInt32(limit, "limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.db.reader().ListInstanceTextHistory(ctx, postgresgen.ListInstanceTextHistoryParams{
		Key: string(key), Locale: locale, Limit: n,
	})
	if err != nil {
		return nil, wrap(err, "list instance text history")
	}
	out := make([]instance.TextRevision, len(rows))
	for i, row := range rows {
		out[i] = instance.TextRevision{
			ID: row.ID, Key: instance.TextKey(row.Key), Locale: row.Locale, Body: row.Body,
			RevisedByUserID: fromNullString(row.RevisedByUserID), RevisedAt: utc(row.RevisedAt),
		}
	}
	return out, nil
}

func currentSetting(row postgresgen.InstanceSetting) instance.CurrentSetting {
	return instance.CurrentSetting{Key: instance.SettingKey(row.Key), RevisionID: row.RevisionID, Value: row.Value}
}

func currentText(row postgresgen.ListAllCurrentInstanceTextsRow) instance.Text {
	return instance.Text{
		Key: instance.TextKey(row.Key), Locale: row.Locale, Body: row.Body,
		RevisionID: row.RevisionID, RevisedAt: utc(row.RevisedAt),
	}
}

func settingRevisions(rows []postgresgen.InstanceSettingRevision) []instance.SettingRevision {
	out := make([]instance.SettingRevision, len(rows))
	for i, row := range rows {
		out[i] = instance.SettingRevision{
			ID: row.ID, Key: instance.SettingKey(row.Key), Value: row.Value,
			RevisedByUserID: fromNullString(row.RevisedByUserID), RevisedAt: utc(row.RevisedAt),
		}
	}
	return out
}

// IsSettingValueUsed implements instance.Repository.
func (r *InstanceRepository) IsSettingValueUsed(ctx context.Context, key instance.SettingKey, value string) (bool, error) {
	ok, err := r.db.reader().IsInstanceSettingValueUsed(ctx, postgresgen.IsInstanceSettingValueUsedParams{Key: string(key), Value: value})
	return ok, wrap(err, "look up instance setting value")
}
