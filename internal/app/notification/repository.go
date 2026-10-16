// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"context"
	"errors"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/notification"
)

// ErrNotFound reports that no row matches a lookup, or a notification that does not belong to
// the account acting on it.
var ErrNotFound = errors.New("notification: not found")

// Repository is the persistence port of the notification module: notifications, their
// dismissals, read markers, the exclusions projected from the graph module's blocks and mutes,
// and the statuses that the content module deleted (data-model §4.7). The database adapters implement it and pass
// testkit.NotificationContract.
//
// Methods that take a uow.Tx run inside that transaction; the others read committed data outside
// any transaction. Lookups that find nothing return an error wrapping ErrNotFound.
type Repository interface {
	// InsertNotification inserts a notification and reports false, writing nothing, when the
	// recipient already has one with the same dedupe key.
	InsertNotification(ctx context.Context, tx uow.Tx, n notification.Notification) (bool, error)
	// Notification returns a notification by ID.
	Notification(ctx context.Context, tx uow.Tx, id string) (notification.Notification, error)
	// InsertDismissal inserts a dismissal and reports false, writing nothing, when the
	// notification is already dismissed.
	InsertDismissal(ctx context.Context, tx uow.Tx, d notification.Dismissal) (bool, error)
	// AdvanceReadMarker sets the account's marker to m.LastReadID unless it is already there or
	// past it, and reports whether it moved.
	AdvanceReadMarker(ctx context.Context, tx uow.Tx, m notification.ReadMarker) (bool, error)

	// Projections, written only by this module's projectors.

	// UpsertExclusion inserts an exclusion, or replaces the expiry of the one with the same
	// viewer, target, and reason.
	UpsertExclusion(ctx context.Context, tx uow.Tx, x notification.Exclusion) error
	// DeleteExclusion deletes the exclusion of the viewer, target, and reason, if any.
	DeleteExclusion(ctx context.Context, tx uow.Tx, viewerAccountID, targetAccountID string, reason notification.ExclusionReason) error
	// InsertDeletedStatus records that a status is deleted, so that the notifications about it
	// are no longer visible. Recording it again changes nothing.
	InsertDeletedStatus(ctx context.Context, tx uow.Tx, statusID string) error

	// Lookups.

	// ListNotifications returns the recipient's visible notifications, newest first, starting
	// after the notification ID before ("" for the newest) and returning at most limit
	// notifications. A non-empty types keeps only notifications of those types. A notification
	// is visible when it is not dismissed, no exclusion of the recipient that is in force at now
	// has its source as the target, and its status, if any, is not recorded as deleted.
	ListNotifications(ctx context.Context, recipientAccountID string, types []notification.Type, before string, limit int, now time.Time) ([]notification.Notification, error)
	// CountNotificationsAfter counts the recipient's visible notifications, as in
	// ListNotifications, whose ID is greater than after ("" counts them all).
	CountNotificationsAfter(ctx context.Context, recipientAccountID, after string, now time.Time) (int64, error)
	// FindReadMarker returns an account's read marker; ErrNotFound means it has read nothing.
	FindReadMarker(ctx context.Context, accountID string) (notification.ReadMarker, error)
}
