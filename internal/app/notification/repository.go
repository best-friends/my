// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"context"
	"errors"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/notification"
)

// ErrNotFound reports that no row matches a lookup, or a notification that does not belong to
// the account acting on it.
var ErrNotFound = errors.New("notification: not found")

// Repository is the persistence port of the notification module: notifications, their
// dismissals, and read markers (data-model §4.7). The database adapters implement it and pass
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

	// Lookups.

	// ListNotifications returns the recipient's notifications that are not dismissed, newest
	// first, starting after the notification ID before ("" for the newest) and returning at
	// most limit notifications. A non-empty types keeps only notifications of those types.
	ListNotifications(ctx context.Context, recipientAccountID string, types []notification.Type, before string, limit int) ([]notification.Notification, error)
	// CountNotificationsAfter counts the recipient's notifications that are not dismissed and
	// whose ID is greater than after ("" counts them all).
	CountNotificationsAfter(ctx context.Context, recipientAccountID, after string) (int64, error)
	// FindReadMarker returns an account's read marker; ErrNotFound means it has read nothing.
	FindReadMarker(ctx context.Context, accountID string) (notification.ReadMarker, error)
}
