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

-- name: InsertReportRuleCitation :exec
INSERT INTO report_rule_citations (report_id, rule_id) VALUES (?, ?)
ON CONFLICT (report_id, rule_id) DO NOTHING;

-- name: UpsertReportState :exec
INSERT INTO report_states (report_id, state, outcome, assignee_user_id, since) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (report_id) DO UPDATE SET
  state = excluded.state, outcome = excluded.outcome, assignee_user_id = excluded.assignee_user_id,
  since = excluded.since;

-- name: GetReport :one
SELECT id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at
FROM reports WHERE id = ?;

-- name: GetReportByURI :one
SELECT id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at
FROM reports WHERE uri = ?;

-- name: ListReportStatusIDs :many
SELECT status_id FROM report_statuses WHERE report_id = ? ORDER BY status_id;

-- name: ListReportRuleIDs :many
SELECT rule_id FROM report_rule_citations WHERE report_id = ? ORDER BY rule_id;

-- name: GetReportState :one
SELECT report_id, state, outcome, assignee_user_id, since FROM report_states WHERE report_id = ?;

-- ListReportsOfTarget returns the reports of a target account, oldest first.
-- name: ListReportsOfTarget :many
SELECT id, uri, reporter_account_id, target_account_id, category, comment, forwarded, filed_at
FROM reports WHERE target_account_id = ? ORDER BY id;

-- ListReportsByReporter returns the reports that an account filed with their state, newest
-- first, those before the report before (empty for the first page).
-- name: ListReportsByReporter :many
SELECT r.id, r.uri, r.reporter_account_id, r.target_account_id, r.category, r.comment, r.forwarded,
  r.filed_at, s.state, s.outcome, s.assignee_user_id, s.since
FROM reports r JOIN report_states s ON s.report_id = r.id
WHERE r.reporter_account_id = sqlc.arg(reporter_account_id)
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR r.id < sqlc.arg(before))
ORDER BY r.id DESC
LIMIT sqlc.arg(lim);

-- Moderators' handling of reports and domain actions -----------------------------------------

-- name: InsertReportAssignment :exec
INSERT INTO report_assignments (id, report_id, assignee_user_id, assigned_by_user_id, assigned_at)
VALUES (?, ?, ?, ?, ?);

-- name: InsertReportResolution :exec
INSERT INTO report_resolutions (id, report_id, outcome, resolved_by_user_id, resolved_at)
VALUES (?, ?, ?, ?, ?);

-- GetReportStateForUpdate reads the state of a report inside the transaction that changes it.
-- name: GetReportStateForUpdate :one
SELECT report_id, state, outcome, assignee_user_id, since FROM report_states WHERE report_id = ?;

-- ListReports returns the reports in a state (empty for any) with their state, newest first,
-- those before the report before (empty for the first page).
-- name: ListReports :many
SELECT r.id, r.uri, r.reporter_account_id, r.target_account_id, r.category, r.comment, r.forwarded,
  r.filed_at, s.state, s.outcome, s.assignee_user_id, s.since
FROM reports r JOIN report_states s ON s.report_id = r.id
WHERE (CAST(sqlc.arg(state) AS TEXT) = '' OR s.state = sqlc.arg(state))
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR r.id < sqlc.arg(before))
ORDER BY r.id DESC
LIMIT sqlc.arg(lim);

-- ListDomainModerationActions returns the domain actions with their lifts, newest first: those
-- of a domain (empty for every domain), of the kinds (a JSON array of strings; empty for every
-- kind), only those in effect at in_effect_at unless it is 0, and those before the action before
-- (empty for the first page).
-- name: ListDomainModerationActions :many
SELECT a.id, a.domain, a.kind, a.public_comment, a.private_note, a.obfuscate, a.issued_by_user_id,
  a.expires_at, a.issued_at, l.action_id AS lift_action_id, l.lifted_by_user_id, l.lifted_at
FROM domain_moderation_actions a
LEFT JOIN domain_moderation_action_lifts l ON l.action_id = a.id
WHERE (CAST(sqlc.arg(domain) AS TEXT) = '' OR a.domain = sqlc.arg(domain))
  AND (CAST(sqlc.arg(kinds) AS TEXT) = ''
    OR a.kind IN (SELECT value FROM json_each(CAST(sqlc.arg(kinds) AS TEXT))))
  AND (CAST(sqlc.arg(in_effect_at) AS INTEGER) = 0
    OR (l.action_id IS NULL AND (a.expires_at IS NULL OR a.expires_at > sqlc.arg(in_effect_at))))
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR a.id < sqlc.arg(before))
ORDER BY a.id DESC
LIMIT sqlc.arg(lim);

-- name: GetDomainModerationActionRecord :one
SELECT a.id, a.domain, a.kind, a.public_comment, a.private_note, a.obfuscate, a.issued_by_user_id,
  a.expires_at, a.issued_at, l.action_id AS lift_action_id, l.lifted_by_user_id, l.lifted_at
FROM domain_moderation_actions a
LEFT JOIN domain_moderation_action_lifts l ON l.action_id = a.id
WHERE a.id = ?;

-- ListModerationActions returns the actions on accounts with their lifts, newest first: those
-- on an account (empty for every account), only those in effect at in_effect_at unless it is 0,
-- and those before the action before (empty for the first page).
-- name: ListModerationActions :many
SELECT a.id, a.target_account_id, a.kind, a.reason_category, a.public_reason, a.private_note,
  a.report_id, a.issuer, a.issued_by_user_id, a.notify_target, a.expires_at, a.issued_at,
  l.action_id AS lift_action_id, l.lifted_by_user_id, l.reason AS lift_reason, l.lifted_at
FROM moderation_actions a
LEFT JOIN moderation_action_lifts l ON l.action_id = a.id
WHERE (CAST(sqlc.arg(account_id) AS TEXT) = '' OR a.target_account_id = sqlc.arg(account_id))
  AND (CAST(sqlc.arg(in_effect_at) AS INTEGER) = 0
    OR (l.action_id IS NULL AND (a.expires_at IS NULL OR a.expires_at > sqlc.arg(in_effect_at))))
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR a.id < sqlc.arg(before))
ORDER BY a.id DESC
LIMIT sqlc.arg(lim);

-- name: GetModerationActionRecord :one
SELECT a.id, a.target_account_id, a.kind, a.reason_category, a.public_reason, a.private_note,
  a.report_id, a.issuer, a.issued_by_user_id, a.notify_target, a.expires_at, a.issued_at,
  l.action_id AS lift_action_id, l.lifted_by_user_id, l.reason AS lift_reason, l.lifted_at
FROM moderation_actions a
LEFT JOIN moderation_action_lifts l ON l.action_id = a.id
WHERE a.id = ?;
