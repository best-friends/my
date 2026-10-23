-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertDataExport :exec
INSERT INTO data_exports (id, user_id, requested_at) VALUES (@id, @user_id, @requested_at);

-- InsertDataExportResult records the end of a build; the first result of an export stands.
-- name: InsertDataExportResult :exec
INSERT INTO data_export_results (export_id, outcome, blob_key, file_size, expires_at, completed_at)
VALUES (@export_id, @outcome, @blob_key, @file_size, @expires_at, @completed_at)
ON CONFLICT (export_id) DO NOTHING;

-- name: GetDataExport :one
SELECT e.id, e.user_id, e.requested_at, r.outcome, r.blob_key, r.file_size, r.expires_at, r.completed_at
FROM data_exports e LEFT JOIN data_export_results r ON r.export_id = e.id
WHERE e.id = sqlc.arg(id)::uuid;

-- ListDataExports returns the newest exports of a user with their results.
-- name: ListDataExports :many
SELECT e.id, e.user_id, e.requested_at, r.outcome, r.blob_key, r.file_size, r.expires_at, r.completed_at
FROM data_exports e LEFT JOIN data_export_results r ON r.export_id = e.id
WHERE e.user_id = sqlc.arg(user_id)::uuid
ORDER BY e.id DESC LIMIT @batch_limit;

-- ListExpiredDataExports returns the exports whose result expires at or before @expires_before.
-- name: ListExpiredDataExports :many
SELECT e.id, e.user_id, e.requested_at, r.outcome, r.blob_key, r.file_size, r.expires_at, r.completed_at
FROM data_export_results r JOIN data_exports e ON e.id = r.export_id
WHERE r.expires_at <= @expires_before
ORDER BY r.expires_at, r.export_id LIMIT @batch_limit;

-- name: DeleteDataExportResult :exec
DELETE FROM data_export_results WHERE export_id = sqlc.arg(export_id)::uuid;

-- name: DeleteDataExport :exec
DELETE FROM data_exports WHERE id = sqlc.arg(id)::uuid;
