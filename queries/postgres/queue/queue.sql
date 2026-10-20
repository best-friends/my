-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of the database job queue and the outbox (internal/adapter/queue/db) for PostgreSQL.

-- name: InsertJob :exec
INSERT INTO jobs (id, type, payload, idempotency_key, ordering_key, attempt, run_at, created_at)
VALUES ($1, $2, $3, $4, $5, 0, $6, $7)
ON CONFLICT (idempotency_key) DO NOTHING;

-- A claim takes three statements in one transaction, because under READ COMMITTED a claim
-- may not see the lease of a job of the same ordering key that another worker is claiming at
-- the same time:
--
--  1. PickJob finds a candidate: the due, unleased job with the earliest run_at that its
--     ordering key does not hold back (no other job of the key leased or enqueued before it, by
--     (created_at, id)). FOR UPDATE SKIP LOCKED lets concurrent workers pass over the rows that
--     others are claiming instead of waiting for them.
--  2. LockOrderingKey, for a candidate with an ordering key, serializes the claims of that key
--     until the transaction ends.
--  3. ClaimJob checks the candidate again in a snapshot taken after the lock, so it sees the
--     leases committed by earlier claims of the key, and leases it.

-- name: PickJob :one
SELECT j.id, j.ordering_key FROM jobs AS j
WHERE j.run_at <= sqlc.arg(now) AND (j.locked_until IS NULL OR j.locked_until <= sqlc.arg(now))
  AND NOT EXISTS (
    SELECT 1 FROM jobs AS k
    WHERE k.ordering_key = j.ordering_key AND k.id <> j.id
      AND (k.locked_until > sqlc.arg(now) OR k.created_at < j.created_at OR (k.created_at = j.created_at AND k.id < j.id))
  )
ORDER BY j.run_at, j.id
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: LockOrderingKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text, 0));

-- name: ClaimJob :one
UPDATE jobs AS j
SET locked_by = sqlc.arg(worker), locked_until = sqlc.arg(locked_until), attempt = j.attempt + 1
WHERE j.id = sqlc.arg(id)
  AND j.run_at <= sqlc.arg(now) AND (j.locked_until IS NULL OR j.locked_until <= sqlc.arg(now))
  AND NOT EXISTS (
    SELECT 1 FROM jobs AS k
    WHERE k.ordering_key = j.ordering_key AND k.id <> j.id
      AND (k.locked_until > sqlc.arg(now) OR k.created_at < j.created_at OR (k.created_at = j.created_at AND k.id < j.id))
  )
RETURNING j.id, j.type, j.payload, j.idempotency_key, j.ordering_key, j.attempt, j.run_at, j.last_error, j.created_at;

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

-- ListDeadJobs returns the dead jobs most recently failed first, after the position
-- (@after_failed_at, @after_id); an empty @after_id starts at the first.
-- name: ListDeadJobs :many
SELECT id, type, payload, idempotency_key, ordering_key, attempt, last_error, created_at, failed_at
FROM dead_jobs
WHERE sqlc.arg(after_id)::text = ''
  OR failed_at < sqlc.arg(after_failed_at)::timestamptz
  OR (failed_at = sqlc.arg(after_failed_at)::timestamptz AND id < sqlc.arg(after_id)::text)
ORDER BY failed_at DESC, id DESC
LIMIT sqlc.arg(batch_limit);

-- name: RequeueDeadJob :execrows
INSERT INTO jobs (id, type, payload, idempotency_key, ordering_key, attempt, run_at, created_at)
SELECT d.id, d.type, d.payload, d.idempotency_key, d.ordering_key, 0, sqlc.arg(run_at), d.created_at
FROM dead_jobs AS d
WHERE d.id = sqlc.arg(id)
ON CONFLICT (idempotency_key) DO NOTHING;

-- name: DeleteDeadJob :execrows
DELETE FROM dead_jobs WHERE id = $1;

-- DeleteDeadJobsBefore removes at most @batch_limit dead jobs that failed before @failed_before,
-- the oldest first.
-- name: DeleteDeadJobsBefore :execrows
DELETE FROM dead_jobs
WHERE id IN (
  SELECT d.id FROM dead_jobs AS d WHERE d.failed_at < sqlc.arg(failed_before)::timestamptz
  ORDER BY d.failed_at, d.id LIMIT sqlc.arg(batch_limit)
);

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

-- CountReadyJobs counts the due, unleased jobs of each type, with the earliest run_at among
-- them, for the queue gauges of the metrics snapshot (ADR-0055 section 4). It counts the jobs
-- that their ordering key holds back too.
-- name: CountReadyJobs :many
SELECT type, COUNT(*) AS ready, CAST(MIN(run_at) AS timestamptz) AS oldest_run_at
FROM jobs
WHERE run_at <= sqlc.arg(now) AND (locked_until IS NULL OR locked_until <= sqlc.arg(now))
GROUP BY type
ORDER BY type;

-- name: CountDeadJobs :one
SELECT COUNT(*) FROM dead_jobs;
