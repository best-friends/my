-- SPDX-License-Identifier: AGPL-3.0-only

-- The account search of the moderators (FR-714, #344). Each query reads the tables of one module
-- and returns IDs newest first in keyset pages; internal/app/identity.AccountAdmin combines them.

-- account --------------------------------------------------------------------------------------

-- SearchAccountIDs returns the accounts newest first, without the instance actor: those on a
-- domain (empty for any), local or remote (origin empty for both), whose handle's normalized
-- username starts with username_prefix (empty for any), before the account before (empty for
-- the first page).
-- name: SearchAccountIDs :many
SELECT a.id FROM accounts a
WHERE NOT (a.domain = '' AND a.actor_type = 'application')
  AND (sqlc.arg(domain)::text = '' OR a.domain = sqlc.arg(domain)::text)
  AND (sqlc.arg(origin)::text = ''
    OR (sqlc.arg(origin)::text = 'local' AND a.domain = '')
    OR (sqlc.arg(origin)::text = 'remote' AND a.domain <> ''))
  AND (sqlc.arg(username_prefix)::text = '' OR EXISTS (
    SELECT 1 FROM account_handles h
    WHERE h.account_id = a.id AND left(h.username_normalized, length(sqlc.arg(username_prefix)::text)) = sqlc.arg(username_prefix)::text))
  AND (sqlc.arg(before)::text = '' OR a.id < sqlc.arg(before)::text)
ORDER BY a.id DESC
LIMIT sqlc.arg(lim);

-- ListAccountIDsInLifecycleState returns the accounts in a lifecycle state newest first, before
-- the account before (empty for the first page).
-- name: ListAccountIDsInLifecycleState :many
SELECT account_id FROM account_lifecycle_states
WHERE state = sqlc.arg(state)::text AND (sqlc.arg(before)::text = '' OR account_id < sqlc.arg(before)::text)
ORDER BY account_id DESC
LIMIT sqlc.arg(lim);

-- identity -------------------------------------------------------------------------------------

-- ListUserIDsByEmailDomain returns the users whose primary address ends with a suffix ('@' and
-- the domain, lowercase) newest first, before the user before (empty for the first page).
-- name: ListUserIDsByEmailDomain :many
SELECT user_id FROM user_primary_emails
WHERE right(address_normalized, length(sqlc.arg(suffix)::text)) = sqlc.arg(suffix)::text
  AND (sqlc.arg(before)::text = '' OR user_id < sqlc.arg(before)::text)
ORDER BY user_id DESC
LIMIT sqlc.arg(lim);

-- ListUserIDsByIP returns the users who registered or signed in from an IP address newest
-- first, before the user before (empty for the first page).
-- name: ListUserIDsByIP :many
SELECT u.user_id FROM (
  SELECT o.user_id FROM session_origins o WHERE o.ip = sqlc.arg(ip)::text
  UNION
  SELECT r.user_id FROM registration_origins r WHERE r.ip = sqlc.arg(ip)::text
) AS u
WHERE sqlc.arg(before)::text = '' OR u.user_id < sqlc.arg(before)::text
ORDER BY u.user_id DESC
LIMIT sqlc.arg(lim);

-- ListUserIDsInRegistrationState returns the users whose registration is in a state newest
-- first, before the user before (empty for the first page).
-- name: ListUserIDsInRegistrationState :many
SELECT user_id FROM user_registration_states
WHERE state = sqlc.arg(state)::text AND (sqlc.arg(before)::text = '' OR user_id < sqlc.arg(before)::text)
ORDER BY user_id DESC
LIMIT sqlc.arg(lim);

-- ListSessionOriginsOfUser returns the newest origins of a user's sessions.
-- name: ListSessionOriginsOfUser :many
SELECT session_id, user_id, ip, user_agent, created_at FROM session_origins
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC, session_id DESC
LIMIT sqlc.arg(lim);

-- moderation -----------------------------------------------------------------------------------

-- ListRestrictedAccountIDs returns the accounts on which their own actions impose a restriction
-- in effect at now, newest first, before the account before (empty for the first page).
-- name: ListRestrictedAccountIDs :many
SELECT DISTINCT account_id FROM account_restrictions
WHERE restriction = sqlc.arg(restriction)::text
  AND (effective_until IS NULL OR effective_until > sqlc.arg(now)::timestamptz)
  AND (sqlc.arg(before)::text = '' OR account_id < sqlc.arg(before)::text)
ORDER BY account_id DESC
LIMIT sqlc.arg(lim);
