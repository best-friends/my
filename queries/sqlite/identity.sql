-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertUser :exec
INSERT INTO users (id, created_at) VALUES (?, ?);

-- name: GetUser :one
SELECT id, created_at FROM users WHERE id = ?;

-- name: InsertPasswordCredential :exec
INSERT INTO password_credentials (id, user_id, password_hash, set_at) VALUES (?, ?, ?, ?);

-- name: GetCurrentPasswordCredential :one
SELECT id, user_id, password_hash, set_at FROM password_credentials
WHERE user_id = ? ORDER BY id DESC LIMIT 1;

-- name: InsertRoleIfAbsent :exec
INSERT INTO roles (id, key, position, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT (key) DO NOTHING;

-- name: GetRoleByKey :one
SELECT id, key, position, created_at FROM roles WHERE key = ?;

-- name: InsertRolePermissionIfAbsent :exec
INSERT INTO role_permissions (role_id, permission) VALUES (?, ?)
ON CONFLICT (role_id, permission) DO NOTHING;

-- name: ListRolePermissions :many
SELECT permission FROM role_permissions WHERE role_id = ? ORDER BY permission;
