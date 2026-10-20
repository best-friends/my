-- SPDX-License-Identifier: AGPL-3.0-only

-- The automatic deletion of local accounts' old posts (FR-118, data-model 4.4).

-- name: GetStatusAutoDeletePolicy :one
SELECT account_id, enabled, min_age_days, keep_pinned, keep_bookmarked, keep_favourited, keep_media,
  keep_polls, keep_min_boosts, keep_min_favourites, updated_at
FROM status_auto_delete_policies WHERE account_id = ?;

-- name: ListStatusAutoDeleteKeptVisibilities :many
SELECT visibility FROM status_auto_delete_kept_visibilities WHERE account_id = ? ORDER BY visibility;

-- name: UpsertStatusAutoDeletePolicy :exec
INSERT INTO status_auto_delete_policies (
  account_id, enabled, min_age_days, keep_pinned, keep_bookmarked, keep_favourited, keep_media,
  keep_polls, keep_min_boosts, keep_min_favourites, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (account_id) DO UPDATE SET
  enabled = excluded.enabled, min_age_days = excluded.min_age_days, keep_pinned = excluded.keep_pinned,
  keep_bookmarked = excluded.keep_bookmarked, keep_favourited = excluded.keep_favourited,
  keep_media = excluded.keep_media, keep_polls = excluded.keep_polls,
  keep_min_boosts = excluded.keep_min_boosts, keep_min_favourites = excluded.keep_min_favourites,
  updated_at = excluded.updated_at;

-- name: DeleteStatusAutoDeleteKeptVisibilities :exec
DELETE FROM status_auto_delete_kept_visibilities WHERE account_id = ?;

-- name: InsertStatusAutoDeleteKeptVisibility :exec
INSERT INTO status_auto_delete_kept_visibilities (account_id, visibility) VALUES (?, ?);

-- ListEnabledStatusAutoDeletePolicies returns up to @batch_limit enabled policies in account order
-- after the cursor @after.
-- name: ListEnabledStatusAutoDeletePolicies :many
SELECT account_id, enabled, min_age_days, keep_pinned, keep_bookmarked, keep_favourited, keep_media,
  keep_polls, keep_min_boosts, keep_min_favourites, updated_at
FROM status_auto_delete_policies
WHERE enabled = 1 AND account_id > CAST(sqlc.arg(after) AS TEXT)
ORDER BY account_id
LIMIT sqlc.arg(batch_limit);

-- ListStatusesToAutoDelete returns, oldest first, up to @batch_limit local statuses of
-- @account_id below @id_before that are not deleted, were published before @published_before,
-- and that none of the policy's exceptions keeps: a visibility in @kept_visibilities (a JSON
-- array), a pin, the author's bookmark or favourite, media or a poll in the current revision, or
-- at least @keep_min_boosts boosts or @keep_min_favourites favourites (0 for no threshold).
-- name: ListStatusesToAutoDelete :many
SELECT s.id, s.uri, s.url, s.author_account_id, s.is_local, s.visibility, s.in_reply_to_uri,
  s.conversation_uri, s.published_at
FROM statuses s
WHERE s.author_account_id = sqlc.arg(account_id) AND s.id < CAST(sqlc.arg(id_before) AS TEXT)
  AND s.is_local = 1 AND s.published_at < CAST(sqlc.arg(published_before) AS INTEGER)
  AND NOT EXISTS (SELECT 1 FROM status_deletions d WHERE d.status_id = s.id)
  AND s.visibility NOT IN (SELECT value FROM json_each(CAST(sqlc.arg(kept_visibilities) AS TEXT)))
  AND (CAST(sqlc.arg(keep_pinned) AS INTEGER) = 0
    OR NOT EXISTS (SELECT 1 FROM status_pins p WHERE p.status_id = s.id))
  AND (CAST(sqlc.arg(keep_bookmarked) AS INTEGER) = 0
    OR NOT EXISTS (SELECT 1 FROM bookmarks b WHERE b.account_id = s.author_account_id AND b.status_id = s.id))
  AND (CAST(sqlc.arg(keep_favourited) AS INTEGER) = 0
    OR NOT EXISTS (SELECT 1 FROM favourites f WHERE f.account_id = s.author_account_id AND f.status_id = s.id))
  AND (CAST(sqlc.arg(keep_media) AS INTEGER) = 0 OR NOT EXISTS (
    SELECT 1 FROM status_heads h JOIN status_revision_attachments a ON a.revision_id = h.revision_id
    WHERE h.status_id = s.id))
  AND (CAST(sqlc.arg(keep_polls) AS INTEGER) = 0 OR NOT EXISTS (
    SELECT 1 FROM status_heads h JOIN polls q ON q.revision_id = h.revision_id WHERE h.status_id = s.id))
  AND (CAST(sqlc.arg(keep_min_boosts) AS INTEGER) = 0 OR NOT EXISTS (
    SELECT 1 FROM status_counters c WHERE c.status_id = s.id
      AND c.boosts_count >= CAST(sqlc.arg(keep_min_boosts) AS INTEGER)))
  AND (CAST(sqlc.arg(keep_min_favourites) AS INTEGER) = 0 OR NOT EXISTS (
    SELECT 1 FROM status_counters c WHERE c.status_id = s.id
      AND c.favourites_count >= CAST(sqlc.arg(keep_min_favourites) AS INTEGER)))
ORDER BY s.id
LIMIT sqlc.arg(batch_limit);
