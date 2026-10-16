-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of the database job queue and the outbox (internal/adapter/queue/db) for PostgreSQL.

-- name: InsertJob :exec
INSERT INTO jobs (id, type, payload, idempotency_key, ordering_key, attempt, run_at, created_at)
VALUES ($1, $2, $3, $4, $5, 0, $6, $7)
ON CONFLICT (idempotency_key) DO NOTHING;

-- ClaimJob leases the due, unleased job with the earliest run_at. FOR UPDATE SKIP LOCKED lets
-- concurrent workers pass over the rows that others are claiming instead of waiting for them.
-- name: ClaimJob :one
UPDATE jobs
SET locked_by = sqlc.arg(worker), locked_until = sqlc.arg(locked_until), attempt = attempt + 1
WHERE id = (
  SELECT j.id FROM jobs AS j
  WHERE j.run_at <= sqlc.arg(now) AND (j.locked_until IS NULL OR j.locked_until <= sqlc.arg(now))
  ORDER BY j.run_at, j.id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING id, type, payload, idempotency_key, ordering_key, attempt, run_at, last_error, created_at;

-- name: DeleteClaimedJob :execrows
DELETE FROM jobs WHERE id = $1 AND locked_by = $2 AND attempt = $3;

-- name: ReleaseJob :execrows
UPDATE jobs
SET locked_by = NULL, locked_until = NULL,
  attempt = sqlc.arg(new_attempt), run_at = sqlc.arg(run_at), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id) AND locked_by = sqlc.arg(worker) AND attempt = sqlc.arg(attempt);

-- name: InsertDeadJobFromClaim :execrows
INSERT INTO dead_jobs (id, type, payload, idempotency_key, ordering_key, attempt, last_error, created_at, failed_at)
SELECT j.id, j.type, j.payload, j.idempotency_key, j.ordering_key, j.attempt, sqlc.arg(last_error), j.created_at, sqlc.arg(failed_at)
FROM jobs AS j
WHERE j.id = sqlc.arg(id) AND j.locked_by = sqlc.arg(worker) AND j.attempt = sqlc.arg(attempt);

-- name: ListDeadJobs :many
SELECT id, type, payload, idempotency_key, ordering_key, attempt, last_error, created_at, failed_at
FROM dead_jobs
ORDER BY failed_at DESC, id DESC
LIMIT $1;

-- name: RequeueDeadJob :execrows
INSERT INTO jobs (id, type, payload, idempotency_key, ordering_key, attempt, run_at, created_at)
SELECT d.id, d.type, d.payload, d.idempotency_key, d.ordering_key, 0, sqlc.arg(run_at), d.created_at
FROM dead_jobs AS d
WHERE d.id = sqlc.arg(id)
ON CONFLICT (idempotency_key) DO NOTHING;

-- name: DeleteDeadJob :execrows
DELETE FROM dead_jobs WHERE id = $1;

-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (id, job_type, payload, idempotency_key, ordering_key, run_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListOutboxEvents :many
SELECT id, job_type, payload, idempotency_key, ordering_key, run_at, created_at
FROM outbox_events
ORDER BY id
LIMIT $1;

-- name: DeleteOutboxEvent :exec
DELETE FROM outbox_events WHERE id = $1;
