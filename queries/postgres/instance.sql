-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertInstanceSettingRevision :exec
INSERT INTO instance_setting_revisions (id, key, value, revised_by_user_id, revised_at) VALUES ($1, $2, $3, $4, $5);

-- name: GetInstanceSetting :one
SELECT key, revision_id, value FROM instance_settings WHERE key = $1;

-- name: UpsertInstanceSetting :exec
INSERT INTO instance_settings (key, revision_id, value) VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET revision_id = excluded.revision_id, value = excluded.value;

-- name: DeleteAllInstanceSettings :exec
DELETE FROM instance_settings;

-- name: ListInstanceSettingRevisions :many
SELECT id, key, value, revised_by_user_id, revised_at FROM instance_setting_revisions ORDER BY id;

-- name: ListInstanceSettings :many
SELECT key, revision_id, value FROM instance_settings ORDER BY key;

-- name: ListInstanceSettingHistory :many
SELECT id, key, value, revised_by_user_id, revised_at FROM instance_setting_revisions
WHERE key = $1 ORDER BY id DESC LIMIT $2;

-- name: InsertInstanceTextRevision :exec
INSERT INTO instance_text_revisions (id, key, locale, body, revised_by_user_id, revised_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetInstanceText :one
SELECT key, locale, revision_id FROM instance_texts WHERE key = $1 AND locale = $2;

-- name: UpsertInstanceText :exec
INSERT INTO instance_texts (key, locale, revision_id) VALUES ($1, $2, $3)
ON CONFLICT (key, locale) DO UPDATE SET revision_id = excluded.revision_id;

-- name: DeleteAllInstanceTexts :exec
DELETE FROM instance_texts;

-- name: ListInstanceTextRevisionHeads :many
SELECT key, locale, id FROM instance_text_revisions ORDER BY id;

-- name: ListCurrentInstanceTexts :many
SELECT t.key, t.locale, t.revision_id, r.body, r.revised_at
FROM instance_texts t JOIN instance_text_revisions r ON r.id = t.revision_id
WHERE t.key = $1 ORDER BY t.locale;

-- name: ListAllCurrentInstanceTexts :many
SELECT t.key, t.locale, t.revision_id, r.body, r.revised_at
FROM instance_texts t JOIN instance_text_revisions r ON r.id = t.revision_id
ORDER BY t.key, t.locale;

-- name: ListInstanceTextHistory :many
SELECT id, key, locale, body, revised_by_user_id, revised_at FROM instance_text_revisions
WHERE key = $1 AND locale = $2 ORDER BY id DESC LIMIT $3;
