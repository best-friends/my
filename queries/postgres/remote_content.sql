-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of remote statuses and media (data-model section 4.4), owned by internal/app/content.

-- name: InsertRemoteStatusRevision :exec
INSERT INTO remote_status_revisions (revision_id, activity_uri, html_policy_version)
VALUES (@revision_id, @activity_uri, @html_policy_version);

-- name: GetRemoteStatusRevision :one
SELECT revision_id, activity_uri, html_policy_version FROM remote_status_revisions
WHERE revision_id = @revision_id;

-- name: InsertRemoteMedia :execrows
INSERT INTO remote_media (
  media_id, owner_account_id, remote_url, declared_content_type, declared_media_type, width, height,
  blurhash, focus_x, focus_y, description
) VALUES (
  @media_id, @owner_account_id, @remote_url, @declared_content_type, @declared_media_type, @width, @height,
  @blurhash, @focus_x, @focus_y, @description
)
ON CONFLICT (owner_account_id, remote_url) DO NOTHING;

-- name: GetRemoteMediaByURL :one
SELECT media_id, owner_account_id, remote_url, declared_content_type, declared_media_type, width, height,
  blurhash, focus_x, focus_y, description
FROM remote_media WHERE owner_account_id = @owner_account_id AND remote_url = @remote_url;

-- name: UpsertRemoteMediaCache :exec
INSERT INTO remote_media_cache (url, blob_key, content_type, file_size, fetched_at, expires_at, last_accessed_at)
VALUES (@url, @blob_key, @content_type, @file_size, @fetched_at, @expires_at, @last_accessed_at)
ON CONFLICT (url) DO UPDATE SET
  blob_key = excluded.blob_key,
  content_type = excluded.content_type,
  file_size = excluded.file_size,
  fetched_at = excluded.fetched_at,
  expires_at = excluded.expires_at,
  last_accessed_at = excluded.last_accessed_at;

-- name: GetRemoteMediaCache :one
SELECT url, blob_key, content_type, file_size, fetched_at, expires_at, last_accessed_at
FROM remote_media_cache WHERE url = @url;

-- name: TouchRemoteMediaCache :exec
UPDATE remote_media_cache SET last_accessed_at = @last_accessed_at WHERE url = @url;

-- ListExpiredRemoteMediaCache returns at most @batch_limit entries past their expiry, the
-- earliest expired first, for the cleanup to delete with their blobs.
-- name: ListExpiredRemoteMediaCache :many
SELECT url, blob_key FROM remote_media_cache WHERE expires_at <= @now
ORDER BY expires_at LIMIT @batch_limit;

-- ListLeastRecentlyAccessedRemoteMediaCache returns the least recently accessed entries, for the
-- cleanup to bring the cache under its size limit.
-- name: ListLeastRecentlyAccessedRemoteMediaCache :many
SELECT url, blob_key, file_size FROM remote_media_cache ORDER BY last_accessed_at LIMIT @batch_limit;

-- name: SumRemoteMediaCacheSize :one
SELECT CAST(COALESCE(SUM(file_size), 0) AS BIGINT) FROM remote_media_cache;

-- name: DeleteRemoteMediaCache :exec
DELETE FROM remote_media_cache WHERE url = @url;
