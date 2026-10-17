-- SPDX-License-Identifier: AGPL-3.0-only

-- Account actions ---------------------------------------------------------------------------

-- name: InsertModerationAction :exec
INSERT INTO moderation_actions (
  id, target_account_id, kind, reason_category, public_reason, private_note, report_id, issuer,
  issued_by_user_id, notify_target, expires_at, issued_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetModerationAction :one
SELECT id, target_account_id, kind, reason_category, public_reason, private_note, report_id, issuer,
  issued_by_user_id, notify_target, expires_at, issued_at
FROM moderation_actions WHERE id = ?;

-- InsertModerationActionLift inserts nothing when the action is already lifted.
-- name: InsertModerationActionLift :execrows
INSERT INTO moderation_action_lifts (action_id, lifted_by_user_id, reason, lifted_at) VALUES (?, ?, ?, ?)
ON CONFLICT (action_id) DO NOTHING;

-- name: InsertAccountRestriction :exec
INSERT INTO account_restrictions (account_id, restriction, source_action_id, effective_until) VALUES (?, ?, ?, ?);

-- name: DeleteAccountRestrictionsOfAction :exec
DELETE FROM account_restrictions WHERE source_action_id = ?;

-- name: ListAccountRestrictionRows :many
SELECT account_id, restriction, source_action_id, effective_until
FROM account_restrictions WHERE account_id = ?
ORDER BY restriction, source_action_id;

-- ListAccountRestrictionsInEffect returns the distinct restrictions of an account in effect at now.
-- name: ListAccountRestrictionsInEffect :many
SELECT DISTINCT restriction FROM account_restrictions
WHERE account_id = sqlc.arg(account_id)
  AND (effective_until IS NULL OR effective_until > CAST(sqlc.arg(now) AS INTEGER))
ORDER BY restriction;

-- Domain actions ----------------------------------------------------------------------------

-- name: InsertDomainModerationAction :exec
INSERT INTO domain_moderation_actions (
  id, domain, kind, public_comment, private_note, obfuscate, issued_by_user_id, expires_at, issued_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetDomainModerationAction :one
SELECT id, domain, kind, public_comment, private_note, obfuscate, issued_by_user_id, expires_at, issued_at
FROM domain_moderation_actions WHERE id = ?;

-- InsertDomainModerationActionLift inserts nothing when the action is already lifted.
-- name: InsertDomainModerationActionLift :execrows
INSERT INTO domain_moderation_action_lifts (action_id, lifted_by_user_id, lifted_at) VALUES (?, ?, ?)
ON CONFLICT (action_id) DO NOTHING;

-- name: InsertDomainRestriction :exec
INSERT INTO domain_restrictions (domain, restriction, source_action_id, effective_until) VALUES (?, ?, ?, ?);

-- name: DeleteDomainRestrictionsOfAction :exec
DELETE FROM domain_restrictions WHERE source_action_id = ?;

-- name: ListDomainRestrictionRows :many
SELECT domain, restriction, source_action_id, effective_until
FROM domain_restrictions WHERE domain = ?
ORDER BY restriction, source_action_id;

-- ListDomainRestrictionsInEffect returns the distinct restrictions in effect at now of any of the
-- domains (a JSON array of strings).
-- name: ListDomainRestrictionsInEffect :many
SELECT DISTINCT restriction FROM domain_restrictions
WHERE (effective_until IS NULL OR effective_until > CAST(sqlc.arg(now) AS INTEGER))
  AND domain IN (SELECT value FROM json_each(CAST(sqlc.arg(domains) AS TEXT)))
ORDER BY restriction;

-- Reports -----------------------------------------------------------------------------------

-- InsertReport inserts nothing when a report with the URI exists.
-- name: InsertReport :execrows
INSERT INTO reports (id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (uri) DO NOTHING;

-- name: InsertReportStatus :exec
INSERT INTO report_statuses (report_id, status_id) VALUES (?, ?)
ON CONFLICT (report_id, status_id) DO NOTHING;

-- name: UpsertReportState :exec
INSERT INTO report_states (report_id, state, assignee_user_id, since) VALUES (?, ?, ?, ?)
ON CONFLICT (report_id) DO UPDATE SET
  state = excluded.state, assignee_user_id = excluded.assignee_user_id, since = excluded.since;

-- name: GetReport :one
SELECT id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at
FROM reports WHERE id = ?;

-- name: GetReportByURI :one
SELECT id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at
FROM reports WHERE uri = ?;

-- name: ListReportStatusIDs :many
SELECT status_id FROM report_statuses WHERE report_id = ? ORDER BY status_id;

-- name: GetReportState :one
SELECT report_id, state, assignee_user_id, since FROM report_states WHERE report_id = ?;

-- ListReportsOfTarget returns the reports of a target account, oldest first.
-- name: ListReportsOfTarget :many
SELECT id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at
FROM reports WHERE target_account_id = ? ORDER BY id;
