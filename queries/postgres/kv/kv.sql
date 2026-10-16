-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of the database key-value store (internal/adapter/kv/db) for PostgreSQL.
-- An entry is live while expires_at is NULL or later than now. Counters are decimal text in value.

-- name: GetEntry :one
SELECT value FROM kv_entries
WHERE namespace = @namespace AND key = @key AND (expires_at IS NULL OR expires_at > @now::timestamptz);

-- name: SetEntry :exec
INSERT INTO kv_entries (namespace, key, value, expires_at) VALUES (@namespace, @key, @value, @expires_at)
ON CONFLICT (namespace, key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at;

-- name: DeleteEntry :exec
DELETE FROM kv_entries WHERE namespace = @namespace AND key = @key;

-- IncrEntry adds the counter in @initial (decimal text) to a live counter, keeping its expiry, or
-- starts a missing or expired one at @initial with expiry @expires_at, in one statement.
-- name: IncrEntry :one
INSERT INTO kv_entries (namespace, key, value, expires_at) VALUES (@namespace, @key, @initial, @expires_at)
ON CONFLICT (namespace, key) DO UPDATE SET
  value = CASE WHEN kv_entries.expires_at <= @now::timestamptz THEN excluded.value
    ELSE convert_to((convert_from(kv_entries.value, 'UTF8')::bigint + convert_from(excluded.value, 'UTF8')::bigint)::text, 'UTF8') END,
  expires_at = CASE WHEN kv_entries.expires_at <= @now::timestamptz THEN excluded.expires_at
    ELSE kv_entries.expires_at END
RETURNING value;

-- name: DeleteExpiredEntries :execrows
DELETE FROM kv_entries WHERE expires_at <= @now::timestamptz;
