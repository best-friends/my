// SPDX-License-Identifier: AGPL-3.0-only
// The rows of the notifications page: the loaded notifications, with the favourites of one post in one
// row and the boosts of one post in another. The client groups them; the API lists every notification on
// its own, and the read marker stays a position among them.
import type { Notification, NotificationType } from "../api/notifications";

/** The types that a row groups by the post they are about. */
const groupedTypes: ReadonlySet<NotificationType> = new Set(["favourite", "boost"]);

/** One row of the page: one notification, or the favourites or the boosts of one post. */
export type NotificationGroup = {
  /**
   * The row's key in the feed: the same while the row gains notifications, so a row that grows when an
   * older page or a newer notification loads stays the same element (and keeps focus).
   */
  key: string;
  /** Newest first; the first one places the row in the list and gives it its time. */
  notifications: readonly [Notification, ...Notification[]];
};

/** The key of the row a notification belongs in. */
function keyOf(notification: Notification): string {
  const { type, status } = notification;
  return groupedTypes.has(type) && status !== undefined ? `${type}:${status.id}` : notification.id;
}

/**
 * Groups `notifications` (newest first) into rows: each favourite or boost of a post joins the row of the
 * newest one of the same type about the same post, wherever it is in the list; other notifications, and
 * those whose post the user may not see, have a row of their own. A row stays where its newest
 * notification is.
 */
export function groupNotifications(notifications: readonly Notification[]): NotificationGroup[] {
  const rows = new Map<string, [Notification, ...Notification[]]>();
  for (const notification of notifications) {
    const key = keyOf(notification);
    const row = rows.get(key);
    if (row === undefined) rows.set(key, [notification]);
    // The same notification twice (a page that overlaps the one before it) counts once.
    else if (!row.some((item) => item.id === notification.id)) row.push(notification);
  }
  return Array.from(rows, ([key, row]) => ({ key, notifications: row }));
}
