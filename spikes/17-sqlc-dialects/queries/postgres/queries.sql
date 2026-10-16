-- SPDX-License-Identifier: AGPL-3.0-only

-- name: UpsertAccount :one
INSERT INTO accounts (id, username, domain, uri, display_name, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(username), sqlc.arg(domain), sqlc.arg(uri), sqlc.arg(display_name), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (uri) DO UPDATE
SET display_name = excluded.display_name,
    updated_at   = excluded.updated_at
RETURNING *;

-- name: GetAccount :one
SELECT * FROM accounts WHERE id = sqlc.arg(id);

-- name: AddFollowCounts :execrows
UPDATE accounts
SET followers_count = followers_count + sqlc.arg(followers_delta)::bigint,
    following_count = following_count + sqlc.arg(following_delta)::bigint
WHERE accounts.id = sqlc.arg(id);

-- name: RecountFollowCounts :execrows
UPDATE accounts
SET followers_count = (SELECT count(*) FROM follows f WHERE f.target_id = accounts.id),
    following_count = (SELECT count(*) FROM follows f WHERE f.follower_id = accounts.id)
WHERE accounts.id = sqlc.arg(id);

-- name: InsertFollow :execrows
INSERT INTO follows (id, follower_id, target_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(follower_id), sqlc.arg(target_id), sqlc.arg(created_at))
ON CONFLICT (follower_id, target_id) DO NOTHING;

-- name: DeleteFollow :execrows
DELETE FROM follows WHERE follower_id = sqlc.arg(follower_id) AND target_id = sqlc.arg(target_id);

-- name: GetFollow :one
SELECT * FROM follows WHERE follower_id = sqlc.arg(follower_id) AND target_id = sqlc.arg(target_id);

-- name: ListFollowers :many
SELECT * FROM follows
WHERE target_id = sqlc.arg(target_id) AND id < sqlc.arg(max_id)
ORDER BY id DESC
LIMIT sqlc.arg(lim);
