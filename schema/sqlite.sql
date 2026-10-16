-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Desired-state schema for SQLite (ADR-0023). Atlas Community Edition diffs this file
-- against the target database and applies the difference: `make db-apply DB_DIALECT=sqlite`.
--
-- Edit this file and schema/postgres.sql together; never write migration SQL by hand.
-- Tables follow docs/architecture/data-model.md, mapping logical types per its §1.2:
-- id, string, text, iri, lang, and enum are TEXT; timestamp is INTEGER Unix milliseconds;
-- bool is INTEGER 0/1; bytes is BLOB. Every table is STRICT.

-- identity (data-model §4.1) -----------------------------------------------------------------

-- Resource: a login principal.
CREATE TABLE users (
  id TEXT NOT NULL PRIMARY KEY,
  created_at INTEGER NOT NULL
) STRICT;

-- Event (revision): the newest row of a user is the current password.
CREATE TABLE password_credentials (
  id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  set_at INTEGER NOT NULL
) STRICT;
CREATE INDEX password_credentials_user_id_id ON password_credentials (user_id, id DESC);

-- Resource: a role, such as the built-in admin and moderator.
CREATE TABLE roles (
  id TEXT NOT NULL PRIMARY KEY,
  key TEXT NOT NULL,
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX roles_key ON roles (key);

-- Current value: the permissions a role grants.
CREATE TABLE role_permissions (
  role_id TEXT NOT NULL,
  permission TEXT NOT NULL CHECK (permission IN (
    'manage_reports', 'manage_accounts', 'manage_domains', 'manage_rules',
    'manage_settings', 'manage_roles', 'manage_invites', 'view_audit'
  )),
  PRIMARY KEY (role_id, permission)
) STRICT;

-- Event: a role granted to a user. granted_by_user_id is NULL for a grant from the CLI.
CREATE TABLE user_role_grants (
  id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  role_id TEXT NOT NULL,
  granted_by_user_id TEXT,
  granted_at INTEGER NOT NULL
) STRICT;

-- Event: a grant revoked. revoked_by_user_id is NULL for a revocation from the CLI.
CREATE TABLE user_role_revocations (
  grant_id TEXT NOT NULL PRIMARY KEY,
  revoked_by_user_id TEXT,
  revoked_at INTEGER NOT NULL
) STRICT;

-- Projection: the roles a user holds now (grants without a revocation).
CREATE TABLE user_roles (
  user_id TEXT NOT NULL,
  role_id TEXT NOT NULL,
  grant_id TEXT NOT NULL,
  PRIMARY KEY (user_id, role_id)
) STRICT;

-- account (data-model §4.2) ------------------------------------------------------------------

-- Resource: a local or remote actor. domain is '' for local actors.
CREATE TABLE accounts (
  id TEXT NOT NULL PRIMARY KEY,
  uri TEXT NOT NULL,
  domain TEXT NOT NULL,
  actor_type TEXT NOT NULL CHECK (actor_type IN ('person', 'service', 'application', 'group', 'organization')),
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX accounts_uri ON accounts (uri);
CREATE INDEX accounts_domain_id ON accounts (domain, id);
-- The only local Application actor is the instance actor.
CREATE UNIQUE INDEX accounts_instance_actor ON accounts (domain, actor_type) WHERE domain = '' AND actor_type = 'application';

-- Resource (subtype): a local account, owned by one user.
CREATE TABLE local_accounts (
  account_id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  username TEXT NOT NULL,
  username_normalized TEXT NOT NULL
) STRICT;
CREATE UNIQUE INDEX local_accounts_user_id ON local_accounts (user_id);
CREATE UNIQUE INDEX local_accounts_username_normalized ON local_accounts (username_normalized);

-- Projection: @username@domain resolution (WebFinger, FR-305, FR-401).
CREATE TABLE account_handles (
  account_id TEXT NOT NULL PRIMARY KEY,
  username TEXT NOT NULL,
  username_normalized TEXT NOT NULL,
  domain TEXT NOT NULL
) STRICT;
CREATE UNIQUE INDEX account_handles_username_normalized_domain ON account_handles (username_normalized, domain);

-- Event (revision): one version of a profile.
CREATE TABLE account_profile_revisions (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  display_name TEXT NOT NULL,
  note_html TEXT NOT NULL,
  locked INTEGER NOT NULL CHECK (locked IN (0, 1)),
  bot INTEGER NOT NULL CHECK (bot IN (0, 1)),
  discoverable INTEGER NOT NULL CHECK (discoverable IN (0, 1)),
  indexable INTEGER NOT NULL CHECK (indexable IN (0, 1)),
  avatar_media_id TEXT,
  header_media_id TEXT,
  revised_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX account_profile_revisions_account_id_seq ON account_profile_revisions (account_id, seq);

-- Event (child of a revision): the profile's metadata fields (FR-006).
CREATE TABLE account_profile_fields (
  revision_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  name TEXT NOT NULL,
  value_html TEXT NOT NULL,
  PRIMARY KEY (revision_id, position)
) STRICT;

-- Projection: the current profile revision of each account.
CREATE TABLE account_profile_heads (
  account_id TEXT NOT NULL PRIMARY KEY,
  revision_id TEXT NOT NULL
) STRICT;

-- Resource: a public signing key of an actor.
CREATE TABLE account_keys (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  key_id TEXT NOT NULL,
  algorithm TEXT NOT NULL CHECK (algorithm IN ('rsa_sha256', 'ed25519')),
  public_key_pem TEXT NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX account_keys_key_id ON account_keys (key_id);
CREATE INDEX account_keys_account_id ON account_keys (account_id);

-- Resource (subtype): the sealed private key of a local actor's key (NFR-SEC-7).
CREATE TABLE local_account_private_keys (
  account_key_id TEXT NOT NULL PRIMARY KEY,
  private_key_enc BLOB NOT NULL
) STRICT;

-- Event: a lifecycle transition of an account (self and system axis, data-model §5.1).
CREATE TABLE account_lifecycle_events (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN (
    'deactivated', 'reactivated', 'deletion_scheduled', 'deletion_cancelled', 'deleted', 'remote_deleted'
  )),
  initiated_by TEXT NOT NULL CHECK (initiated_by IN ('self', 'moderator', 'system', 'remote')),
  moderation_action_id TEXT,
  deletion_due_at INTEGER,
  occurred_at INTEGER NOT NULL,
  CHECK ((kind = 'deletion_scheduled') = (deletion_due_at IS NOT NULL))
) STRICT;
CREATE INDEX account_lifecycle_events_account_id_id ON account_lifecycle_events (account_id, id DESC);

-- Projection: the current lifecycle state. An account without a row is active.
CREATE TABLE account_lifecycle_states (
  account_id TEXT NOT NULL PRIMARY KEY,
  state TEXT NOT NULL CHECK (state IN ('active', 'deactivated', 'deletion_scheduled', 'deleted')),
  since INTEGER NOT NULL,
  deletion_due_at INTEGER,
  CHECK ((state = 'deletion_scheduled') = (deletion_due_at IS NOT NULL))
) STRICT;
CREATE INDEX account_lifecycle_states_state_deletion_due_at ON account_lifecycle_states (state, deletion_due_at);
