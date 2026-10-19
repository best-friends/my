-- SPDX-License-Identifier: AGPL-3.0-only

-- Granting and revoking roles (FR-720).

-- ListRoles returns every role, highest position first.
-- name: ListRoles :many
SELECT id, key, position, created_at FROM roles ORDER BY position DESC, key;

-- ListAllRolePermissions returns the permissions of every role.
-- name: ListAllRolePermissions :many
SELECT role_id, permission FROM role_permissions ORDER BY role_id, permission;

-- GetUserRoleGrantID returns the grant through which a user holds a role now.
-- name: GetUserRoleGrantID :one
SELECT grant_id FROM user_roles WHERE user_id = ? AND role_id = ?;

-- CountRoleHolders counts the users who hold a role now.
-- name: CountRoleHolders :one
SELECT COUNT(*) FROM user_roles WHERE role_id = ?;

-- ListRoleHolderIDs returns the users who hold a role now, by ID.
-- name: ListRoleHolderIDs :many
SELECT user_id FROM user_roles WHERE role_id = ? ORDER BY user_id;

-- name: InsertUserRoleRevocation :exec
INSERT INTO user_role_revocations (grant_id, revoked_by_user_id, revoked_at) VALUES (?, ?, ?);

-- name: DeleteUserRoleByGrant :exec
DELETE FROM user_roles WHERE grant_id = ?;
