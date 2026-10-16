-- SPDX-License-Identifier: AGPL-3.0-only

-- Notifications (event) and their dismissals (event).

-- InsertNotification inserts nothing when the recipient already has a notification with the
-- dedupe key.
-- name: InsertNotification :execrows
INSERT INTO notifications (id, recipient_account_id, type, source_account_id, status_id, moderation_action_id, dedupe_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (recipient_account_id, dedupe_key) DO NOTHING;

-- name: GetNotification :one
SELECT id, recipient_account_id, type, source_account_id, status_id, moderation_action_id, dedupe_key, created_at
FROM notifications WHERE id = $1;

-- InsertNotificationDismissal inserts nothing when the notification is already dismissed.
-- name: InsertNotificationDismissal :execrows
INSERT INTO notification_dismissals (notification_id, dismissed_at) VALUES ($1, $2)
ON CONFLICT (notification_id) DO NOTHING;

-- ListNotifications pages the notifications of a recipient that are not dismissed, newest
-- first; an empty before starts at the newest. With all_types false only the listed types count.
-- name: ListNotifications :many
SELECT n.id, n.recipient_account_id, n.type, n.source_account_id, n.status_id, n.moderation_action_id, n.dedupe_key, n.created_at
FROM notifications n
WHERE n.recipient_account_id = sqlc.arg(recipient_account_id)
  AND (sqlc.arg(before)::text = '' OR n.id < sqlc.arg(before)::text)
  AND (sqlc.arg(all_types)::boolean OR n.type = ANY(sqlc.arg(types)::text[]))
  AND NOT EXISTS (SELECT 1 FROM notification_dismissals d WHERE d.notification_id = n.id)
ORDER BY n.id DESC
LIMIT sqlc.arg(page_limit);

-- CountNotificationsAfter counts the notifications of a recipient after the ID after that are
-- not dismissed; an empty after counts them all.
-- name: CountNotificationsAfter :one
SELECT COUNT(*) FROM notifications n
WHERE n.recipient_account_id = sqlc.arg(recipient_account_id)
  AND n.id > sqlc.arg(after)::text
  AND NOT EXISTS (SELECT 1 FROM notification_dismissals d WHERE d.notification_id = n.id);

-- Read markers (current value).

-- name: GetNotificationReadMarker :one
SELECT account_id, last_read_id FROM notification_read_markers WHERE account_id = $1;

-- AdvanceNotificationReadMarker moves the marker forward only: it changes nothing when the
-- marker is already at or past last_read_id.
-- name: AdvanceNotificationReadMarker :execrows
INSERT INTO notification_read_markers (account_id, last_read_id) VALUES ($1, $2)
ON CONFLICT (account_id) DO UPDATE SET last_read_id = excluded.last_read_id
WHERE excluded.last_read_id > notification_read_markers.last_read_id;
