-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertAccount :exec
INSERT INTO accounts (id, uri, domain, actor_type, created_at) VALUES ($1, $2, $3, $4, $5);

-- name: InsertLocalAccount :exec
INSERT INTO local_accounts (account_id, user_id, username, username_normalized) VALUES ($1, $2, $3, $4);

-- name: InsertProfileRevision :exec
INSERT INTO account_profile_revisions (
  id, account_id, seq, display_name, note_html, locked, bot, discoverable, indexable,
  avatar_media_id, header_media_id, revised_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: InsertProfileField :exec
INSERT INTO account_profile_fields (revision_id, position, name, value_html) VALUES ($1, $2, $3, $4);

-- name: InsertAccountKey :exec
INSERT INTO account_keys (id, account_id, key_id, algorithm, public_key_pem, created_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertLocalAccountPrivateKey :exec
INSERT INTO local_account_private_keys (account_key_id, private_key_enc) VALUES ($1, $2);

-- name: InsertLifecycleEvent :exec
INSERT INTO account_lifecycle_events (
  id, account_id, kind, initiated_by, moderation_action_id, deletion_due_at, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpsertAccountHandle :exec
INSERT INTO account_handles (account_id, username, username_normalized, domain) VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id) DO UPDATE SET
  username = excluded.username,
  username_normalized = excluded.username_normalized,
  domain = excluded.domain;

-- name: DeleteAllAccountHandles :exec
DELETE FROM account_handles;

-- name: GetProfileHead :one
SELECT h.account_id, h.revision_id, r.seq
FROM account_profile_heads h JOIN account_profile_revisions r ON r.id = h.revision_id
WHERE h.account_id = $1;

-- name: UpsertProfileHead :exec
INSERT INTO account_profile_heads (account_id, revision_id) VALUES ($1, $2)
ON CONFLICT (account_id) DO UPDATE SET revision_id = excluded.revision_id;

-- name: DeleteAllProfileHeads :exec
DELETE FROM account_profile_heads;

-- name: GetLifecycleState :one
SELECT account_id, state, since, deletion_due_at FROM account_lifecycle_states WHERE account_id = $1;

-- name: UpsertLifecycleState :exec
INSERT INTO account_lifecycle_states (account_id, state, since, deletion_due_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id) DO UPDATE SET
  state = excluded.state,
  since = excluded.since,
  deletion_due_at = excluded.deletion_due_at;

-- name: DeleteAllLifecycleStates :exec
DELETE FROM account_lifecycle_states;

-- name: ListLocalAccounts :many
SELECT account_id, user_id, username, username_normalized FROM local_accounts ORDER BY account_id;

-- name: ListProfileRevisionSeqs :many
SELECT account_id, id, seq FROM account_profile_revisions ORDER BY account_id, seq;

-- name: ListLifecycleEvents :many
SELECT id, account_id, kind, initiated_by, moderation_action_id, deletion_due_at, occurred_at
FROM account_lifecycle_events ORDER BY account_id, id;

-- name: ListAccountHandles :many
SELECT account_id, username, username_normalized, domain FROM account_handles ORDER BY account_id;

-- name: ListProfileHeads :many
SELECT h.account_id, h.revision_id, r.seq
FROM account_profile_heads h JOIN account_profile_revisions r ON r.id = h.revision_id
ORDER BY h.account_id;

-- name: ListLifecycleStates :many
SELECT account_id, state, since, deletion_due_at FROM account_lifecycle_states ORDER BY account_id;

-- name: GetAccount :one
SELECT id, uri, domain, actor_type, created_at FROM accounts WHERE id = $1;

-- name: GetInstanceActor :one
SELECT id, uri, domain, actor_type, created_at FROM accounts
WHERE domain = '' AND actor_type = 'application';

-- name: GetLocalAccount :one
SELECT account_id, user_id, username, username_normalized FROM local_accounts WHERE account_id = $1;

-- name: GetAccountHandleByName :one
SELECT account_id, username, username_normalized, domain FROM account_handles
WHERE username_normalized = $1 AND domain = $2;

-- name: GetAccountHandle :one
SELECT account_id, username, username_normalized, domain FROM account_handles WHERE account_id = $1;

-- name: GetProfileRevision :one
SELECT id, account_id, seq, display_name, note_html, locked, bot, discoverable, indexable,
  avatar_media_id, header_media_id, revised_at
FROM account_profile_revisions WHERE id = $1;

-- name: ListProfileFields :many
SELECT revision_id, position, name, value_html FROM account_profile_fields
WHERE revision_id = $1 ORDER BY position;

-- name: ListAccountKeys :many
SELECT id, account_id, key_id, algorithm, public_key_pem, created_at FROM account_keys
WHERE account_id = $1 ORDER BY id;

-- name: GetLocalAccountPrivateKey :one
SELECT private_key_enc FROM local_account_private_keys WHERE account_key_id = $1;

-- name: CountLocalAccounts :one
SELECT COUNT(*) FROM local_accounts la
LEFT JOIN account_lifecycle_states s ON s.account_id = la.account_id
WHERE s.state IS NULL OR s.state <> 'deleted';
