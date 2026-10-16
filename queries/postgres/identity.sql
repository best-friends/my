-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertUser :exec
INSERT INTO users (id, created_at) VALUES ($1, $2);

-- name: GetUser :one
SELECT id, created_at FROM users WHERE id = $1;

-- name: InsertPasswordCredential :exec
INSERT INTO password_credentials (id, user_id, password_hash, set_at) VALUES ($1, $2, $3, $4);

-- name: GetCurrentPasswordCredential :one
SELECT id, user_id, password_hash, set_at FROM password_credentials
WHERE user_id = $1 ORDER BY id DESC LIMIT 1;

-- name: InsertRoleIfAbsent :exec
INSERT INTO roles (id, key, position, created_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (key) DO NOTHING;

-- name: GetRoleByKey :one
SELECT id, key, position, created_at FROM roles WHERE key = $1;

-- name: InsertRolePermissionIfAbsent :exec
INSERT INTO role_permissions (role_id, permission) VALUES ($1, $2)
ON CONFLICT (role_id, permission) DO NOTHING;

-- name: ListRolePermissions :many
SELECT permission FROM role_permissions WHERE role_id = $1 ORDER BY permission;
