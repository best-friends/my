-- SPDX-License-Identifier: AGPL-3.0-only
-- The custom emoji that the administrators manage and that the picker and local texts use
-- (ADR-0051). Owned by internal/app/content.

-- name: InsertCustomEmoji :exec
INSERT INTO custom_emojis (id, shortcode, domain, uri, created_at)
VALUES (@id, @shortcode, @domain, @uri, @created_at);

-- name: DeleteCustomEmojiRow :exec
DELETE FROM custom_emojis WHERE id = @id;

-- name: DeleteCustomEmojiImages :exec
DELETE FROM custom_emoji_images WHERE emoji_id = @emoji_id;

-- name: UpsertCustomEmojiSettings :exec
INSERT INTO custom_emoji_settings (emoji_id, enabled, visible_in_picker, category)
VALUES (@emoji_id, @enabled, @visible_in_picker, @category)
ON CONFLICT (emoji_id) DO UPDATE SET enabled = excluded.enabled,
  visible_in_picker = excluded.visible_in_picker, category = excluded.category;

-- name: DeleteCustomEmojiSettings :exec
DELETE FROM custom_emoji_settings WHERE emoji_id = @emoji_id;

-- name: GetCustomEmojiSettings :one
SELECT emoji_id, enabled, visible_in_picker, category FROM custom_emoji_settings WHERE emoji_id = @emoji_id;

-- name: InsertCustomEmojiAlias :exec
INSERT INTO custom_emoji_aliases (emoji_id, alias) VALUES (@emoji_id, @alias) ON CONFLICT DO NOTHING;

-- name: DeleteCustomEmojiAliases :exec
DELETE FROM custom_emoji_aliases WHERE emoji_id = @emoji_id;

-- ListCustomEmojiAliases returns the aliases of the emoji with an ID in @emoji_ids.
-- name: ListCustomEmojiAliases :many
SELECT emoji_id, alias FROM custom_emoji_aliases
WHERE emoji_id = ANY(@emoji_ids::text[])
ORDER BY emoji_id, alias;

-- ListAdminCustomEmojis returns a page of the custom emoji, newest first, with their current
-- images and settings. @locality is 0 for every emoji, 1 for the local ones, 2 for the remote
-- ones; an empty @domain and @shortcode select every domain and shortcode, and @shortcode
-- matches a part of the shortcode in lower case.
-- name: ListAdminCustomEmojis :many
SELECT e.id, e.shortcode, e.domain, e.uri, e.created_at,
  i.id AS image_id, i.media_id AS image_media_id, i.revised_at AS image_revised_at,
  COALESCE(s.enabled, TRUE)::boolean AS enabled,
  COALESCE(s.visible_in_picker, TRUE)::boolean AS visible_in_picker,
  COALESCE(s.category, '')::text AS category
FROM custom_emojis e
JOIN custom_emoji_images i ON i.emoji_id = e.id
LEFT JOIN custom_emoji_settings s ON s.emoji_id = e.id
WHERE i.id = (SELECT MAX(l.id) FROM custom_emoji_images l WHERE l.emoji_id = e.id)
  AND (@locality::integer = 0 OR (@locality::integer = 1) = (e.domain = ''))
  AND (@domain::text = '' OR e.domain = @domain::text)
  AND (@shortcode::text = '' OR strpos(lower(e.shortcode), @shortcode::text) > 0)
  AND (@before_id::text = '' OR e.id < @before_id::text)
ORDER BY e.id DESC
LIMIT @row_limit;

-- ListLocalCustomEmojis returns the local custom emoji with their current images and settings,
-- by shortcode: every one, or without @all_shortcodes those with a shortcode in @shortcodes.
-- name: ListLocalCustomEmojis :many
SELECT e.id, e.shortcode, e.domain, e.uri, e.created_at,
  i.id AS image_id, i.media_id AS image_media_id, i.revised_at AS image_revised_at,
  COALESCE(s.enabled, TRUE)::boolean AS enabled,
  COALESCE(s.visible_in_picker, TRUE)::boolean AS visible_in_picker,
  COALESCE(s.category, '')::text AS category
FROM custom_emojis e
JOIN custom_emoji_images i ON i.emoji_id = e.id
LEFT JOIN custom_emoji_settings s ON s.emoji_id = e.id
WHERE e.domain = ''
  AND i.id = (SELECT MAX(l.id) FROM custom_emoji_images l WHERE l.emoji_id = e.id)
  AND (@all_shortcodes::boolean OR e.shortcode = ANY(@shortcodes::text[]))
ORDER BY e.shortcode;

-- ListUnreferencedRemoteEmojis returns, in ID order after the cursor, the IDs of the remote custom
-- emoji that no status revision and no reaction refers to (the purge of ADR-0051).
-- name: ListUnreferencedRemoteEmojis :many
SELECT e.id FROM custom_emojis e
WHERE e.domain <> '' AND e.id > CAST(@after AS TEXT)
  AND NOT EXISTS (SELECT 1 FROM status_revision_emojis r WHERE r.emoji_id = e.id)
  AND NOT EXISTS (SELECT 1 FROM reactions x WHERE x.custom_emoji_id = e.id)
ORDER BY e.id
LIMIT @row_limit;

-- name: ListCustomEmojiImageMediaIDs :many
SELECT DISTINCT media_id FROM custom_emoji_images WHERE emoji_id = @emoji_id ORDER BY media_id;

-- DeleteUnreferencedRemoteEmoji deletes the row of a remote custom emoji while no status
-- revision and no reaction refers to it.
-- name: DeleteUnreferencedRemoteEmoji :execrows
DELETE FROM custom_emojis
WHERE custom_emojis.id = sqlc.arg(id) AND custom_emojis.domain <> ''
  AND NOT EXISTS (SELECT 1 FROM status_revision_emojis r WHERE r.emoji_id = sqlc.arg(id))
  AND NOT EXISTS (SELECT 1 FROM reactions x WHERE x.custom_emoji_id = sqlc.arg(id));

-- DeleteUnreferencedRemoteMedia deletes the remote subtype of a remote media file while no
-- custom emoji image and no attachment uses it.
-- name: DeleteUnreferencedRemoteMedia :execrows
DELETE FROM remote_media
WHERE remote_media.media_id = sqlc.arg(media_id)
  AND NOT EXISTS (SELECT 1 FROM custom_emoji_images c WHERE c.media_id = sqlc.arg(media_id))
  AND NOT EXISTS (SELECT 1 FROM status_revision_attachments a WHERE a.media_id = sqlc.arg(media_id));

-- name: DeleteRemoteMediaFile :exec
DELETE FROM media_files WHERE media_files.id = sqlc.arg(id) AND media_files.origin = 'remote';
