-- SPDX-License-Identifier: AGPL-3.0-only

-- The audit log (data-model 4.3, ADR-0037). It is append-only: there is no query that updates or
-- deletes an entry, except the retention cleanup (ADR-0038).

-- name: InsertAuditLogEntry :exec
INSERT INTO audit_log_entries (id, actor_kind, actor_user_id, action, target_kind, target_id, target_label,
  source_event_id, occurred_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- ListAuditLogEntries returns the entries newest first: those of an actor, an action, a kind of
-- target, and a target (each empty for any), before the entry before (empty for the first page).
-- name: ListAuditLogEntries :many
SELECT id, actor_kind, actor_user_id, action, target_kind, target_id, target_label, source_event_id, occurred_at
FROM audit_log_entries
WHERE (CAST(sqlc.arg(actor_user_id) AS TEXT) = '' OR actor_user_id = sqlc.arg(actor_user_id))
  AND (CAST(sqlc.arg(action) AS TEXT) = '' OR action = sqlc.arg(action))
  AND (CAST(sqlc.arg(target_kind) AS TEXT) = '' OR target_kind = sqlc.arg(target_kind))
  AND (CAST(sqlc.arg(target_id) AS TEXT) = '' OR target_id = sqlc.arg(target_id))
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR id < sqlc.arg(before))
ORDER BY id DESC
LIMIT sqlc.arg(lim);
