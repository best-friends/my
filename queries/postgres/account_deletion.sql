-- SPDX-License-Identifier: AGPL-3.0-only

-- Scheduled deletions of accounts (data-model sections 4.2 and 8) ---------------------------------------

-- ListDueDeletions returns up to lim accounts whose deletion is scheduled at or before now,
-- earliest first.
-- name: ListDueDeletions :many
SELECT account_id, state, since, deletion_due_at FROM account_lifecycle_states
WHERE state = 'deletion_scheduled' AND deletion_due_at <= sqlc.arg(now)::timestamptz
ORDER BY deletion_due_at, account_id
LIMIT sqlc.arg(lim);
