-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertMediaFile :exec
INSERT INTO media_files (id, owner_account_id, origin, created_at) VALUES (?, ?, ?, ?);

-- name: InsertUploadedMedia :exec
INSERT INTO uploaded_media (media_id, blob_key, content_type, file_size) VALUES (?, ?, ?, ?);

-- name: InsertMediaProcessingResult :exec
INSERT INTO media_processing_results (
  id, media_id, purpose, outcome, media_type, width, height, duration_ms, blurhash, processed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertMediaVariant :exec
INSERT INTO media_variants (result_id, size, motion, blob_key, content_type, file_size, width, height)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- HasCurrentMediaProcessingResult reports whether the newest processing result of a media file
-- is for a purpose.
-- name: HasCurrentMediaProcessingResult :one
SELECT EXISTS (
  SELECT 1 FROM media_processing_results r
  WHERE r.purpose = sqlc.arg(purpose) AND r.id = (
    SELECT c.id FROM media_processing_results c WHERE c.media_id = sqlc.arg(media_id) ORDER BY c.id DESC LIMIT 1)
);

-- name: UpsertMediaFocus :exec
INSERT INTO media_focus (media_id, focus_x, focus_y) VALUES (?, ?, ?)
ON CONFLICT (media_id) DO UPDATE SET focus_x = excluded.focus_x, focus_y = excluded.focus_y;

-- name: DeleteUnattachedMediaFile :execrows
DELETE FROM media_files
WHERE id = sqlc.arg(id)
  AND NOT EXISTS (SELECT 1 FROM status_revision_attachments a WHERE a.media_id = sqlc.arg(id));

-- name: DeleteUploadedMedia :exec
DELETE FROM uploaded_media WHERE media_id = ?;

-- name: DeleteMediaVariantsOf :exec
DELETE FROM media_variants WHERE result_id IN (SELECT id FROM media_processing_results WHERE media_id = ?);

-- name: DeleteMediaProcessingResults :exec
DELETE FROM media_processing_results WHERE media_id = ?;

-- name: DeleteMediaFocus :exec
DELETE FROM media_focus WHERE media_id = ?;

-- name: GetMediaFile :one
SELECT id, owner_account_id, origin, created_at FROM media_files WHERE id = ?;

-- name: GetUploadedMedia :one
SELECT media_id, blob_key, content_type, file_size FROM uploaded_media WHERE media_id = ?;

-- name: ListMediaProcessingResults :many
SELECT id, media_id, purpose, outcome, media_type, width, height, duration_ms, blurhash, processed_at
FROM media_processing_results WHERE media_id = ? ORDER BY id;

-- name: ListMediaVariantsOf :many
SELECT v.result_id, v.size, v.motion, v.blob_key, v.content_type, v.file_size, v.width, v.height
FROM media_variants v JOIN media_processing_results r ON r.id = v.result_id
WHERE r.media_id = ? ORDER BY v.result_id, v.size, v.motion;

-- name: GetMediaFocus :one
SELECT media_id, focus_x, focus_y FROM media_focus WHERE media_id = ?;

-- Uploads by ID, which is a UUIDv7 and so also orders and bounds them by creation time.
-- name: ListUnattachedUploads :many
SELECT m.id FROM media_files m
WHERE m.origin = 'upload' AND m.id > sqlc.arg(after_id) AND m.id < sqlc.arg(before_id)
  AND NOT EXISTS (SELECT 1 FROM status_revision_attachments a WHERE a.media_id = m.id)
ORDER BY m.id
LIMIT sqlc.arg(row_limit);

-- The uploads of an owner by ID, with whether a status revision attaches each, for the erasure
-- of a deleted account (data-model section 8).
-- name: ListOwnerUploads :many
SELECT m.id, CAST(EXISTS (SELECT 1 FROM status_revision_attachments a WHERE a.media_id = m.id) AS BOOLEAN) AS attached
FROM media_files m
WHERE m.owner_account_id = sqlc.arg(owner_account_id) AND m.origin = 'upload' AND m.id > sqlc.arg(after_id)
ORDER BY m.id
LIMIT sqlc.arg(row_limit);

-- name: UpsertMediaDescription :exec
INSERT INTO media_descriptions (media_id, description) VALUES (?, ?)
ON CONFLICT (media_id) DO UPDATE SET description = excluded.description;

-- name: DeleteMediaDescription :exec
DELETE FROM media_descriptions WHERE media_id = ?;

-- name: GetMediaDescription :one
SELECT media_id, description FROM media_descriptions WHERE media_id = ?;

-- name: IsMediaAttached :one
SELECT EXISTS (SELECT 1 FROM status_revision_attachments WHERE media_id = ?);

-- ListReadyMediaVariants returns the variants of the current (newest) processing result of each
-- listed media file, with the file, for the files whose current result is ready. The list
-- is a JSON array of strings, read with json_each (see timeline.sql).
-- name: ListReadyMediaVariants :many
SELECT m.id, m.owner_account_id, m.origin, m.created_at,
  v.result_id, v.size, v.motion, v.blob_key, v.content_type, v.file_size, v.width, v.height
FROM media_files m
JOIN media_processing_results r ON r.id = (
  SELECT c.id FROM media_processing_results c WHERE c.media_id = m.id ORDER BY c.id DESC LIMIT 1)
JOIN media_variants v ON v.result_id = r.id
WHERE m.id IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT)))
  AND r.outcome = 'ready'
ORDER BY m.id, v.size, v.motion;
