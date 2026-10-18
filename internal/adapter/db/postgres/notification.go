// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"time"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/notification"
)

// NotificationRepository implements notification.Repository on PostgreSQL.
type NotificationRepository struct {
	db *DB
}

var _ appnotification.Repository = (*NotificationRepository)(nil)

// NotificationRepository returns the notification module's repository on d.
func (d *DB) NotificationRepository() *NotificationRepository { return &NotificationRepository{db: d} }

// InsertNotification implements notification.Repository.
func (r *NotificationRepository) InsertNotification(ctx context.Context, tx uow.Tx, n notification.Notification) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	rows, err := q.InsertNotification(ctx, postgresgen.InsertNotificationParams{
		ID: n.ID, RecipientAccountID: n.RecipientAccountID, Type: string(n.Type),
		SourceAccountID: toNullString(n.SourceAccountID), StatusID: toNullString(n.StatusID),
		ModerationActionID: toNullString(n.ModerationActionID), ReportID: toNullString(n.ReportID),
		SubjectUserID: toNullString(n.SubjectUserID), AppealID: toNullString(n.AppealID), DedupeKey: n.DedupeKey, CreatedAt: n.CreatedAt,
	})
	if err != nil {
		return false, wrap(err, "insert notification")
	}
	return rows == 1, nil
}

// Notification implements notification.Repository.
func (r *NotificationRepository) Notification(ctx context.Context, tx uow.Tx, id string) (notification.Notification, error) {
	q, err := queries(tx)
	if err != nil {
		return notification.Notification{}, err
	}
	row, err := q.GetNotification(ctx, id)
	if err != nil {
		return notification.Notification{}, notFound(err, appnotification.ErrNotFound, "notification "+id)
	}
	return notificationFrom(row), nil
}

// InsertDismissal implements notification.Repository.
func (r *NotificationRepository) InsertDismissal(ctx context.Context, tx uow.Tx, d notification.Dismissal) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	rows, err := q.InsertNotificationDismissal(ctx, postgresgen.InsertNotificationDismissalParams{
		NotificationID: d.NotificationID, DismissedAt: d.DismissedAt,
	})
	if err != nil {
		return false, wrap(err, "insert notification dismissal")
	}
	return rows == 1, nil
}

// AdvanceReadMarker implements notification.Repository.
func (r *NotificationRepository) AdvanceReadMarker(ctx context.Context, tx uow.Tx, m notification.ReadMarker) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	rows, err := q.AdvanceNotificationReadMarker(ctx, postgresgen.AdvanceNotificationReadMarkerParams{
		AccountID: m.AccountID, LastReadID: m.LastReadID,
	})
	if err != nil {
		return false, wrap(err, "advance notification read marker")
	}
	return rows == 1, nil
}

// UpsertExclusion implements notification.Repository.
func (r *NotificationRepository) UpsertExclusion(ctx context.Context, tx uow.Tx, x notification.Exclusion) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.UpsertNotificationExclusion(ctx, postgresgen.UpsertNotificationExclusionParams{
		ViewerAccountID: x.ViewerAccountID, TargetAccountID: x.TargetAccountID, Reason: string(x.Reason), ExpiresAt: x.ExpiresAt,
	}), "upsert notification exclusion")
}

// DeleteExclusion implements notification.Repository.
func (r *NotificationRepository) DeleteExclusion(ctx context.Context, tx uow.Tx, viewerAccountID, targetAccountID string, reason notification.ExclusionReason) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.DeleteNotificationExclusion(ctx, postgresgen.DeleteNotificationExclusionParams{
		ViewerAccountID: viewerAccountID, TargetAccountID: targetAccountID, Reason: string(reason),
	}), "delete notification exclusion")
}

// InsertDeletedStatus implements notification.Repository.
func (r *NotificationRepository) InsertDeletedStatus(ctx context.Context, tx uow.Tx, statusID string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.InsertNotificationDeletedStatus(ctx, statusID), "insert notification deleted status")
}

// ListNotifications implements notification.Repository.
func (r *NotificationRepository) ListNotifications(ctx context.Context, recipientAccountID string, types, hidden []notification.Type, before string, limit int, now time.Time) ([]notification.Notification, error) {
	n, err := toInt32(limit, "page limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.db.reader().ListNotifications(ctx, postgresgen.ListNotificationsParams{
		RecipientAccountID: recipientAccountID, Before: before, AllTypes: len(types) == 0, Types: typeNames(types), HiddenTypes: typeNames(hidden), Now: now, PageLimit: n,
	})
	if err != nil {
		return nil, wrap(err, "list notifications")
	}
	out := make([]notification.Notification, len(rows))
	for i, row := range rows {
		out[i] = notificationFrom(row)
	}
	return out, nil
}

// CountNotificationsAfter implements notification.Repository.
func (r *NotificationRepository) CountNotificationsAfter(ctx context.Context, recipientAccountID, after string, hidden []notification.Type, now time.Time) (int64, error) {
	n, err := r.db.reader().CountNotificationsAfter(ctx, postgresgen.CountNotificationsAfterParams{
		RecipientAccountID: recipientAccountID, After: after, HiddenTypes: typeNames(hidden), Now: now,
	})
	return n, wrap(err, "count notifications")
}

// FindReadMarker implements notification.Repository.
func (r *NotificationRepository) FindReadMarker(ctx context.Context, accountID string) (notification.ReadMarker, error) {
	row, err := r.db.reader().GetNotificationReadMarker(ctx, accountID)
	if err != nil {
		return notification.ReadMarker{}, notFound(err, appnotification.ErrNotFound, "notification read marker of "+accountID)
	}
	return notification.ReadMarker{AccountID: row.AccountID, LastReadID: row.LastReadID}, nil
}

func notificationFrom(row postgresgen.Notification) notification.Notification {
	return notification.Notification{
		ID: row.ID, RecipientAccountID: row.RecipientAccountID, Type: notification.Type(row.Type),
		SourceAccountID: fromNullString(row.SourceAccountID), StatusID: fromNullString(row.StatusID),
		ModerationActionID: fromNullString(row.ModerationActionID), ReportID: fromNullString(row.ReportID),
		SubjectUserID: fromNullString(row.SubjectUserID), AppealID: fromNullString(row.AppealID), DedupeKey: row.DedupeKey, CreatedAt: utc(row.CreatedAt),
	}
}

// typeNames returns the types as the queries take them; none is an empty array.
func typeNames(types []notification.Type) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return names
}
