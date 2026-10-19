-- SPDX-License-Identifier: AGPL-3.0-only

-- The audit log (data-model 4.3, ADR-0037). It is append-only: there is no query that updates or
-- deletes an entry, except the retention cleanup (ADR-0038).

-- name: InsertAuditLogEntry :exec
INSERT INTO audit_log_entries (id, actor_kind, actor_user_id, action, target_kind, target_id, target_label,
  source_event_id, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- ListAuditLogEntries returns the entries newest first: those of an actor, an action, a kind of
-- target, and a target (each empty for any), before the entry before (empty for the first page).
-- name: ListAuditLogEntries :many
SELECT id, actor_kind, actor_user_id, action, target_kind, target_id, target_label, source_event_id, occurred_at
FROM audit_log_entries
WHERE (sqlc.arg(actor_user_id)::text = '' OR actor_user_id = sqlc.arg(actor_user_id)::text)
  AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text)
  AND (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (sqlc.arg(target_id)::text = '' OR target_id = sqlc.arg(target_id)::text)
  AND (sqlc.arg(before)::text = '' OR id < sqlc.arg(before)::text)
ORDER BY id DESC
LIMIT sqlc.arg(lim);

-- The targets of the entries that still exist, for the audit log to tell those that are gone
-- (erased past their retention, ADR-0038). Each returns, in ID order, the records among ids
-- that exist.

-- name: ListExistingReportIDs :many
SELECT id FROM reports WHERE id = ANY(sqlc.arg(ids)::text[]) ORDER BY id;

-- name: ListExistingAppealIDs :many
SELECT id FROM moderation_appeals WHERE id = ANY(sqlc.arg(ids)::text[]) ORDER BY id;

-- name: ListExistingSignupBlockIDs :many
SELECT id FROM signup_blocks WHERE id = ANY(sqlc.arg(ids)::text[]) ORDER BY id;
