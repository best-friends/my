-- SPDX-License-Identifier: AGPL-3.0-only

-- Granting and revoking roles (FR-720).

-- ListRoles returns every role, highest position first.
-- name: ListRoles :many
SELECT id, key, position, created_at FROM roles ORDER BY position DESC, key;

-- ListAllRolePermissions returns the permissions of every role.
-- name: ListAllRolePermissions :many
SELECT role_id, permission FROM role_permissions ORDER BY role_id, permission;

-- GetRoleByKeyForUpdate reads and locks a role inside the transaction that grants or revokes
-- it, so that changes to one role run one after another.
-- name: GetRoleByKeyForUpdate :one
SELECT id, key, position, created_at FROM roles WHERE key = $1
FOR UPDATE;

-- GetUserRoleGrantID returns the grant through which a user holds a role now.
-- name: GetUserRoleGrantID :one
SELECT grant_id FROM user_roles WHERE user_id = $1 AND role_id = $2;

-- CountRoleHolders counts the users who hold a role now.
-- name: CountRoleHolders :one
SELECT COUNT(*) FROM user_roles WHERE role_id = $1;

-- ListRoleHolderIDs returns the users who hold a role now, by ID.
-- name: ListRoleHolderIDs :many
SELECT user_id FROM user_roles WHERE role_id = $1 ORDER BY user_id;

-- name: InsertUserRoleRevocation :exec
INSERT INTO user_role_revocations (grant_id, revoked_by_user_id, revoked_at) VALUES ($1, $2, $3);

-- name: DeleteUserRoleByGrant :exec
DELETE FROM user_roles WHERE grant_id = $1;
