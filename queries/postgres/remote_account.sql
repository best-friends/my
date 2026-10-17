-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of the remote actors and their keys (data-model section 4.2), owned by
-- internal/app/account.

-- name: InsertRemoteAccount :execrows
INSERT INTO remote_accounts (account_id, first_seen_at) VALUES (@account_id, @first_seen_at)
ON CONFLICT (account_id) DO NOTHING;

-- name: InsertRemoteAccountSnapshot :exec
INSERT INTO remote_account_snapshots (
  id, account_id, preferred_username, profile_url, inbox_url, shared_inbox_url, outbox_url,
  followers_url, following_url, featured_url, followers_total, following_total, statuses_total,
  document_digest, fetched_at
) VALUES (
  @id, @account_id, @preferred_username, @profile_url, @inbox_url, @shared_inbox_url, @outbox_url,
  @followers_url, @following_url, @featured_url, @followers_total, @following_total, @statuses_total,
  @document_digest, @fetched_at
);

-- GetLatestRemoteAccountSnapshot returns the newest snapshot of an actor.
-- name: GetLatestRemoteAccountSnapshot :one
SELECT id, account_id, preferred_username, profile_url, inbox_url, shared_inbox_url, outbox_url,
  followers_url, following_url, featured_url, followers_total, following_total, statuses_total,
  document_digest, fetched_at
FROM remote_account_snapshots WHERE account_id = @account_id ORDER BY id DESC LIMIT 1;

-- name: UpsertRemoteAccountHead :exec
INSERT INTO remote_account_heads (
  account_id, snapshot_id, inbox_url, shared_inbox_url, last_fetched_at, consecutive_fetch_failures
) VALUES (@account_id, @snapshot_id, @inbox_url, @shared_inbox_url, @last_fetched_at, @consecutive_fetch_failures)
ON CONFLICT (account_id) DO UPDATE SET
  snapshot_id = excluded.snapshot_id,
  inbox_url = excluded.inbox_url,
  shared_inbox_url = excluded.shared_inbox_url,
  last_fetched_at = excluded.last_fetched_at,
  consecutive_fetch_failures = excluded.consecutive_fetch_failures;

-- name: GetRemoteAccountHead :one
SELECT account_id, snapshot_id, inbox_url, shared_inbox_url, last_fetched_at, consecutive_fetch_failures
FROM remote_account_heads WHERE account_id = @account_id;

-- ListStaleRemoteAccountHeads returns the actors fetched before @fetched_before, the least
-- recently fetched first, for the periodic refresh.
-- name: ListStaleRemoteAccountHeads :many
SELECT account_id, snapshot_id, inbox_url, shared_inbox_url, last_fetched_at, consecutive_fetch_failures
FROM remote_account_heads WHERE last_fetched_at < @fetched_before
ORDER BY last_fetched_at LIMIT @batch_limit;

-- name: InsertRemoteAccountHandleVerification :exec
INSERT INTO remote_account_handle_verifications (id, account_id, username, domain, verified_at)
VALUES (@id, @account_id, @username, @domain, @verified_at);

-- GetLatestRemoteAccountHandleVerification returns the newest verified handle of an actor.
-- name: GetLatestRemoteAccountHandleVerification :one
SELECT id, account_id, username, domain, verified_at FROM remote_account_handle_verifications
WHERE account_id = @account_id ORDER BY id DESC LIMIT 1;

-- name: InsertRemoteProfileRevision :exec
INSERT INTO remote_profile_revisions (revision_id, snapshot_id, html_policy_version)
VALUES (@revision_id, @snapshot_id, @html_policy_version);

-- GetAccountKeyByKeyIDAndFingerprint finds a key fetched before, to tell a rotation apart.
-- name: GetAccountKeyByKeyIDAndFingerprint :one
SELECT id, account_id, key_id, algorithm, public_key_pem, fingerprint, created_at FROM account_keys
WHERE key_id = @key_id AND fingerprint = @fingerprint;

-- GetCurrentAccountKey returns the newest key of a key IRI without a revocation, the one that
-- verifies signatures.
-- name: GetCurrentAccountKey :one
SELECT k.id, k.account_id, k.key_id, k.algorithm, k.public_key_pem, k.fingerprint, k.created_at
FROM account_keys k
WHERE k.key_id = @key_id
  AND NOT EXISTS (SELECT 1 FROM account_key_revocations r WHERE r.account_key_id = k.id)
ORDER BY k.id DESC LIMIT 1;

-- name: InsertAccountKeyRevocation :execrows
INSERT INTO account_key_revocations (account_key_id, revoked_at) VALUES (@account_key_id, @revoked_at)
ON CONFLICT (account_key_id) DO NOTHING;
