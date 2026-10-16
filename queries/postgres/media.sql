-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertMediaFile :exec
INSERT INTO media_files (id, owner_account_id, origin, created_at) VALUES ($1, $2, $3, $4);

-- name: InsertUploadedMedia :exec
INSERT INTO uploaded_media (media_id, blob_key, content_type, file_size) VALUES ($1, $2, $3, $4);

-- name: InsertMediaProcessingResult :exec
INSERT INTO media_processing_results (
  id, media_id, outcome, media_type, width, height, duration_ms, blurhash, processed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: InsertMediaVariant :exec
INSERT INTO media_variants (result_id, kind, blob_key, content_type, file_size, width, height)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: HasMediaProcessingResult :one
SELECT EXISTS (SELECT 1 FROM media_processing_results WHERE media_id = $1);

-- name: UpsertMediaFocus :exec
INSERT INTO media_focus (media_id, focus_x, focus_y) VALUES ($1, $2, $3)
ON CONFLICT (media_id) DO UPDATE SET focus_x = excluded.focus_x, focus_y = excluded.focus_y;

-- name: DeleteUnattachedMediaFile :execrows
DELETE FROM media_files
WHERE id = sqlc.arg(id)
  AND NOT EXISTS (SELECT 1 FROM status_revision_attachments a WHERE a.media_id = sqlc.arg(id));

-- name: DeleteUploadedMedia :exec
DELETE FROM uploaded_media WHERE media_id = $1;

-- name: DeleteMediaVariantsOf :exec
DELETE FROM media_variants WHERE result_id IN (SELECT id FROM media_processing_results WHERE media_id = $1);

-- name: DeleteMediaProcessingResults :exec
DELETE FROM media_processing_results WHERE media_id = $1;

-- name: DeleteMediaFocus :exec
DELETE FROM media_focus WHERE media_id = $1;

-- name: GetMediaFile :one
SELECT id, owner_account_id, origin, created_at FROM media_files WHERE id = $1;

-- name: GetUploadedMedia :one
SELECT media_id, blob_key, content_type, file_size FROM uploaded_media WHERE media_id = $1;

-- name: ListMediaProcessingResults :many
SELECT id, media_id, outcome, media_type, width, height, duration_ms, blurhash, processed_at
FROM media_processing_results WHERE media_id = $1 ORDER BY id;

-- name: ListMediaVariantsOf :many
SELECT v.result_id, v.kind, v.blob_key, v.content_type, v.file_size, v.width, v.height
FROM media_variants v JOIN media_processing_results r ON r.id = v.result_id
WHERE r.media_id = $1 ORDER BY v.result_id, v.kind;

-- name: GetMediaFocus :one
SELECT media_id, focus_x, focus_y FROM media_focus WHERE media_id = $1;

-- Uploads by ID, which is a UUIDv7 and so also orders and bounds them by creation time.
-- name: ListUnattachedUploads :many
SELECT m.id FROM media_files m
WHERE m.origin = 'upload' AND m.id > sqlc.arg(after_id) AND m.id < sqlc.arg(before_id)
  AND NOT EXISTS (SELECT 1 FROM status_revision_attachments a WHERE a.media_id = m.id)
ORDER BY m.id
LIMIT sqlc.arg(row_limit);

-- name: UpsertMediaDescription :exec
INSERT INTO media_descriptions (media_id, description) VALUES ($1, $2)
ON CONFLICT (media_id) DO UPDATE SET description = excluded.description;

-- name: DeleteMediaDescription :exec
DELETE FROM media_descriptions WHERE media_id = $1;

-- name: GetMediaDescription :one
SELECT media_id, description FROM media_descriptions WHERE media_id = $1;

-- name: IsMediaAttached :one
SELECT EXISTS (SELECT 1 FROM status_revision_attachments WHERE media_id = $1);
