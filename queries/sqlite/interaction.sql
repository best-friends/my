-- SPDX-License-Identifier: AGPL-3.0-only

-- Boosts (resource) and their retractions (event).

-- name: InsertBoost :exec
INSERT INTO boosts (id, uri, booster_account_id, status_id, visibility, is_local, boosted_at) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: InsertBoostRetraction :exec
INSERT INTO boost_retractions (boost_id, origin, retracted_at) VALUES (?, ?, ?);

-- name: GetBoost :one
SELECT id, uri, booster_account_id, status_id, visibility, is_local, boosted_at FROM boosts WHERE id = ?;

-- name: GetBoostByURI :one
SELECT id, uri, booster_account_id, status_id, visibility, is_local, boosted_at FROM boosts WHERE uri = ?;

-- name: GetBoostRetraction :one
SELECT boost_id, origin, retracted_at FROM boost_retractions WHERE boost_id = ?;

-- Projection boost_edges.

-- name: GetBoostEdge :one
SELECT booster_account_id, status_id, boost_id FROM boost_edges WHERE booster_account_id = ? AND status_id = ?;

-- InsertBoostEdge inserts nothing when the pair already has a live boost.
-- name: InsertBoostEdge :execrows
INSERT INTO boost_edges (booster_account_id, status_id, boost_id) VALUES (?, ?, ?)
ON CONFLICT (booster_account_id, status_id) DO NOTHING;

-- name: DeleteBoostEdge :execrows
DELETE FROM boost_edges WHERE booster_account_id = ? AND status_id = ? AND boost_id = ?;

-- name: DeleteStatusBoostEdges :exec
DELETE FROM boost_edges WHERE status_id = ?;

-- name: DeleteAllBoostEdges :exec
DELETE FROM boost_edges;

-- RebuildBoostEdges states the projector's rule: boosts that are not retracted, of statuses
-- that are not deleted.
-- name: RebuildBoostEdges :exec
INSERT INTO boost_edges (booster_account_id, status_id, boost_id)
SELECT b.booster_account_id, b.status_id, b.id
FROM boosts b
WHERE NOT EXISTS (SELECT 1 FROM boost_retractions r WHERE r.boost_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM status_deletions d WHERE d.status_id = b.status_id);

-- name: ListBoostEdges :many
SELECT booster_account_id, status_id, boost_id FROM boost_edges ORDER BY booster_account_id, status_id;

-- The status_counters columns that the interactions change.

-- name: AddStatusBoosts :exec
UPDATE status_counters SET boosts_count = boosts_count + sqlc.arg(delta) WHERE status_id = sqlc.arg(status_id);

-- name: AddStatusFavourites :exec
UPDATE status_counters SET favourites_count = favourites_count + sqlc.arg(delta) WHERE status_id = sqlc.arg(status_id);

-- name: CountStatusMention :one
SELECT COUNT(*) FROM status_mention_index WHERE status_id = ? AND account_id = ?;

-- Favourites, bookmarks, and pins (current values). Each insert does nothing when the account
-- already has the row for the status.

-- name: InsertFavourite :execrows
INSERT INTO favourites (account_id, status_id, uri, created_at, id) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (account_id, status_id) DO NOTHING;

-- name: DeleteFavourite :many
DELETE FROM favourites WHERE account_id = ? AND status_id = ?
RETURNING account_id, status_id, uri, created_at, id;

-- name: GetFavourite :one
SELECT account_id, status_id, uri, created_at, id FROM favourites WHERE account_id = ? AND status_id = ?;

-- name: ListFavourites :many
SELECT account_id, status_id, uri, created_at, id FROM favourites
WHERE account_id = sqlc.arg(account_id) AND (CAST(sqlc.arg(before) AS TEXT) = '' OR id < CAST(sqlc.arg(before) AS TEXT))
ORDER BY id DESC LIMIT sqlc.arg(page_limit);

-- name: InsertBookmark :execrows
INSERT INTO bookmarks (account_id, status_id, created_at, id) VALUES (?, ?, ?, ?)
ON CONFLICT (account_id, status_id) DO NOTHING;

-- name: DeleteBookmark :execrows
DELETE FROM bookmarks WHERE account_id = ? AND status_id = ?;

-- name: GetBookmark :one
SELECT account_id, status_id, created_at, id FROM bookmarks WHERE account_id = ? AND status_id = ?;

-- name: ListBookmarks :many
SELECT account_id, status_id, created_at, id FROM bookmarks
WHERE account_id = sqlc.arg(account_id) AND (CAST(sqlc.arg(before) AS TEXT) = '' OR id < CAST(sqlc.arg(before) AS TEXT))
ORDER BY id DESC LIMIT sqlc.arg(page_limit);

-- name: InsertStatusPin :execrows
INSERT INTO status_pins (account_id, status_id, created_at, id) VALUES (?, ?, ?, ?)
ON CONFLICT (account_id, status_id) DO NOTHING;

-- name: DeleteStatusPin :execrows
DELETE FROM status_pins WHERE account_id = ? AND status_id = ?;

-- name: CountStatusPins :one
SELECT COUNT(*) FROM status_pins WHERE account_id = ?;

-- name: GetStatusPin :one
SELECT account_id, status_id, created_at, id FROM status_pins WHERE account_id = ? AND status_id = ?;

-- name: ListStatusPins :many
SELECT account_id, status_id, created_at, id FROM status_pins WHERE account_id = ? ORDER BY id DESC;
