-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of the database job queue and the outbox (internal/adapter/queue/db) for SQLite.
-- Timestamps are Unix milliseconds (see docs/architecture/data-model.md, section 1.2).

-- name: InsertJob :exec
INSERT INTO jobs (id, type, payload, idempotency_key, ordering_key, attempt, run_at, created_at)
VALUES (?, ?, ?, ?, ?, 0, ?, ?)
ON CONFLICT (idempotency_key) DO NOTHING;

-- ClaimJob leases the due, unleased job with the earliest run_at that its ordering key does
-- not hold back: no other job of the key may be leased or enqueued before it, by (created_at,
-- id). BEGIN IMMEDIATE serializes claims, so two workers never claim the same job or two jobs
-- of one key.
-- name: ClaimJob :one
UPDATE jobs
SET locked_by = @worker, locked_until = @locked_until, attempt = attempt + 1
WHERE id = (
  SELECT j.id FROM jobs AS j
  WHERE j.run_at <= @now AND (j.locked_until IS NULL OR j.locked_until <= @now)
    AND NOT EXISTS (
      SELECT 1 FROM jobs AS k
      WHERE k.ordering_key = j.ordering_key AND k.id <> j.id
        AND (k.locked_until > @now OR k.created_at < j.created_at OR (k.created_at = j.created_at AND k.id < j.id))
    )
  ORDER BY j.run_at, j.id
  LIMIT 1
)
RETURNING id, type, payload, idempotency_key, ordering_key, attempt, run_at, last_error, created_at;

-- name: DeleteClaimedJob :execrows
DELETE FROM jobs WHERE id = ? AND locked_by = ? AND attempt = ?;

-- name: ReleaseJob :execrows
UPDATE jobs
SET locked_by = NULL, locked_until = NULL,
  attempt = @new_attempt, run_at = @run_at, last_error = @last_error
WHERE id = @id AND locked_by = @worker AND attempt = @attempt;

-- name: InsertDeadJobFromClaim :execrows
INSERT INTO dead_jobs (id, type, payload, idempotency_key, ordering_key, attempt, last_error, created_at, failed_at)
SELECT j.id, j.type, j.payload, j.idempotency_key, j.ordering_key, j.attempt, @last_error, j.created_at, @failed_at
FROM jobs AS j
WHERE j.id = @id AND j.locked_by = @worker AND j.attempt = @attempt;

-- ListDeadJobs returns the dead jobs most recently failed first, after the position
-- (@after_failed_at, @after_id); an empty @after_id starts at the first.
-- name: ListDeadJobs :many
SELECT id, type, payload, idempotency_key, ordering_key, attempt, last_error, created_at, failed_at
FROM dead_jobs
WHERE CAST(sqlc.arg(after_id) AS TEXT) = ''
  OR failed_at < CAST(sqlc.arg(after_failed_at) AS INTEGER)
  OR (failed_at = CAST(sqlc.arg(after_failed_at) AS INTEGER) AND id < CAST(sqlc.arg(after_id) AS TEXT))
ORDER BY failed_at DESC, id DESC
LIMIT sqlc.arg(batch_limit);

-- name: RequeueDeadJob :execrows
INSERT INTO jobs (id, type, payload, idempotency_key, ordering_key, attempt, run_at, created_at)
SELECT d.id, d.type, d.payload, d.idempotency_key, d.ordering_key, 0, @run_at, d.created_at
FROM dead_jobs AS d
WHERE d.id = @id
ON CONFLICT (idempotency_key) DO NOTHING;

-- name: DeleteDeadJob :execrows
DELETE FROM dead_jobs WHERE id = ?;

-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (id, job_type, payload, idempotency_key, ordering_key, run_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListOutboxEvents :many
SELECT id, job_type, payload, idempotency_key, ordering_key, run_at, created_at
FROM outbox_events
ORDER BY id
LIMIT ?;

-- name: DeleteOutboxEvent :exec
DELETE FROM outbox_events WHERE id = ?;
