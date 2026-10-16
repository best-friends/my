-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertUserPreferences :exec
INSERT INTO user_preferences (
  user_id, locale, time_zone, default_visibility, default_post_language, require_media_description, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateUserPreferences :execrows
UPDATE user_preferences SET
  locale = ?, time_zone = ?, default_visibility = ?, default_post_language = ?,
  require_media_description = ?, updated_at = ?
WHERE user_id = ?;

-- name: GetUserPreferences :one
SELECT user_id, locale, time_zone, default_visibility, default_post_language, require_media_description, updated_at
FROM user_preferences WHERE user_id = ?;

-- name: DeleteUserTimelineLanguages :exec
DELETE FROM user_timeline_languages WHERE user_id = ?;

-- name: InsertUserTimelineLanguage :exec
INSERT INTO user_timeline_languages (user_id, language) VALUES (?, ?);

-- name: ListUserTimelineLanguages :many
SELECT language FROM user_timeline_languages WHERE user_id = ? ORDER BY language;
