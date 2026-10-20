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
WHERE emoji_id IN (SELECT value FROM json_each(CAST(@emoji_ids AS TEXT)))
ORDER BY emoji_id, alias;

-- ListAdminCustomEmojis returns a page of the custom emoji, newest first, with their current
-- images and settings. @locality is 0 for every emoji, 1 for the local ones, 2 for the remote
-- ones; an empty @domain and @shortcode select every domain and shortcode, and @shortcode
-- matches a part of the shortcode in lower case.
-- name: ListAdminCustomEmojis :many
SELECT e.id, e.shortcode, e.domain, e.uri, e.created_at,
  i.id AS image_id, i.media_id AS image_media_id, i.revised_at AS image_revised_at,
  CAST(COALESCE(s.enabled, 1) AS INTEGER) AS enabled,
  CAST(COALESCE(s.visible_in_picker, 1) AS INTEGER) AS visible_in_picker,
  CAST(COALESCE(s.category, '') AS TEXT) AS category
FROM custom_emojis e
JOIN custom_emoji_images i ON i.emoji_id = e.id
LEFT JOIN custom_emoji_settings s ON s.emoji_id = e.id
WHERE i.id = (SELECT MAX(l.id) FROM custom_emoji_images l WHERE l.emoji_id = e.id)
  AND (CAST(@locality AS INTEGER) = 0 OR (CAST(@locality AS INTEGER) = 1) = (e.domain = ''))
  AND (CAST(@domain AS TEXT) = '' OR e.domain = CAST(@domain AS TEXT))
  AND (CAST(@shortcode AS TEXT) = '' OR instr(lower(e.shortcode), CAST(@shortcode AS TEXT)) > 0)
  AND (CAST(@before_id AS TEXT) = '' OR e.id < CAST(@before_id AS TEXT))
ORDER BY e.id DESC
LIMIT @row_limit;

-- ListLocalCustomEmojis returns the local custom emoji with their current images and settings,
-- by shortcode: every one, or without @all_shortcodes those with a shortcode in @shortcodes (a
-- JSON array).
-- name: ListLocalCustomEmojis :many
SELECT e.id, e.shortcode, e.domain, e.uri, e.created_at,
  i.id AS image_id, i.media_id AS image_media_id, i.revised_at AS image_revised_at,
  CAST(COALESCE(s.enabled, 1) AS INTEGER) AS enabled,
  CAST(COALESCE(s.visible_in_picker, 1) AS INTEGER) AS visible_in_picker,
  CAST(COALESCE(s.category, '') AS TEXT) AS category
FROM custom_emojis e
JOIN custom_emoji_images i ON i.emoji_id = e.id
LEFT JOIN custom_emoji_settings s ON s.emoji_id = e.id
WHERE e.domain = ''
  AND i.id = (SELECT MAX(l.id) FROM custom_emoji_images l WHERE l.emoji_id = e.id)
  AND (CAST(@all_shortcodes AS INTEGER) = 1
    OR e.shortcode IN (SELECT value FROM json_each(CAST(@shortcodes AS TEXT))))
ORDER BY e.shortcode;
