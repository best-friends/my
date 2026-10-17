-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertUserPreferences :exec
INSERT INTO user_preferences (
  user_id, locale, time_zone, default_visibility, default_post_language, require_media_description,
  email_moderator_notifications, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateUserPreferences :execrows
UPDATE user_preferences SET
  locale = $2, time_zone = $3, default_visibility = $4, default_post_language = $5,
  require_media_description = $6, email_moderator_notifications = $7, updated_at = $8
WHERE user_id = $1;

-- name: GetUserPreferences :one
SELECT user_id, locale, time_zone, default_visibility, default_post_language, require_media_description,
  email_moderator_notifications, updated_at
FROM user_preferences WHERE user_id = $1;

-- name: DeleteUserTimelineLanguages :exec
DELETE FROM user_timeline_languages WHERE user_id = $1;

-- name: InsertUserTimelineLanguage :exec
INSERT INTO user_timeline_languages (user_id, language) VALUES ($1, $2);

-- name: ListUserTimelineLanguages :many
SELECT language FROM user_timeline_languages WHERE user_id = $1 ORDER BY language;
