-- SPDX-License-Identifier: AGPL-3.0-only

-- LockAccountPair takes a transaction-scoped advisory lock on a pair of accounts, so that
-- follows and blocks of one pair serialize under READ COMMITTED.
-- name: LockAccountPair :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(pair_key)::text, 0));

-- name: InsertBlock :exec
INSERT INTO blocks (id, uri, blocker_account_id, blocked_account_id, created_at) VALUES ($1, $2, $3, $4, $5);

-- name: InsertBlockRetraction :exec
INSERT INTO block_retractions (block_id, retracted_at) VALUES ($1, $2);

-- name: GetBlock :one
SELECT id, uri, blocker_account_id, blocked_account_id, created_at FROM blocks WHERE id = $1;

-- name: GetBlockEdge :one
SELECT blocker_account_id, blocked_account_id, block_id
FROM block_edges WHERE blocker_account_id = $1 AND blocked_account_id = $2;

-- InsertBlockEdge inserts nothing when the pair already has an edge.
-- name: InsertBlockEdge :execrows
INSERT INTO block_edges (blocker_account_id, blocked_account_id, block_id) VALUES ($1, $2, $3)
ON CONFLICT (blocker_account_id, blocked_account_id) DO NOTHING;

-- name: DeleteBlockEdge :exec
DELETE FROM block_edges WHERE blocker_account_id = $1 AND blocked_account_id = $2 AND block_id = $3;

-- name: DeleteAllBlockEdges :exec
DELETE FROM block_edges;

-- name: ListBlockHistories :many
SELECT b.id, b.uri, b.blocker_account_id, b.blocked_account_id, b.created_at, r.retracted_at
FROM blocks b
LEFT JOIN block_retractions r ON r.block_id = b.id
ORDER BY b.id;

-- name: ListBlockEdges :many
SELECT blocker_account_id, blocked_account_id, block_id
FROM block_edges ORDER BY blocker_account_id, blocked_account_id;

-- ListBlockingPage pages the edges out of a blocker, newest block first; an empty before starts
-- at the newest.
-- name: ListBlockingPage :many
SELECT blocker_account_id, blocked_account_id, block_id
FROM block_edges
WHERE blocker_account_id = sqlc.arg(account_id)
  AND (sqlc.arg(before)::text = '' OR block_id < sqlc.arg(before)::text)
ORDER BY block_id DESC
LIMIT sqlc.arg(page_limit);

-- name: UpsertMute :exec
INSERT INTO mutes (account_id, target_account_id, hide_notifications, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id, target_account_id) DO UPDATE SET
  hide_notifications = excluded.hide_notifications,
  expires_at = excluded.expires_at,
  created_at = excluded.created_at;

-- DeleteMute deletes the mute and reports, by the rows it deleted, whether there was one.
-- name: DeleteMute :execrows
DELETE FROM mutes WHERE account_id = $1 AND target_account_id = $2;

-- GetActiveMute returns the mute unless it has expired at now.
-- name: GetActiveMute :one
SELECT account_id, target_account_id, hide_notifications, expires_at, created_at
FROM mutes
WHERE account_id = sqlc.arg(account_id) AND target_account_id = sqlc.arg(target_account_id)
  AND (expires_at IS NULL OR expires_at > sqlc.arg(now)::timestamptz);

-- ListActiveMutesPage pages the mutes of an account that have not expired at now, newest first
-- by (created_at, target_account_id); an empty before_target_account_id starts at the newest.
-- name: ListActiveMutesPage :many
SELECT account_id, target_account_id, hide_notifications, expires_at, created_at
FROM mutes
WHERE account_id = sqlc.arg(account_id)
  AND (expires_at IS NULL OR expires_at > sqlc.arg(now)::timestamptz)
  AND (sqlc.arg(before_target_account_id)::text = ''
    OR created_at < sqlc.arg(before_created_at)::timestamptz
    OR (created_at = sqlc.arg(before_created_at)::timestamptz AND target_account_id < sqlc.arg(before_target_account_id)::text))
ORDER BY created_at DESC, target_account_id DESC
LIMIT sqlc.arg(page_limit);

-- ListAllMutes returns every mute, expired ones included, ordered by account and target, as the
-- source of other modules' projections.
-- name: ListAllMutes :many
SELECT account_id, target_account_id, hide_notifications, expires_at, created_at
FROM mutes ORDER BY account_id, target_account_id;
