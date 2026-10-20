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
