-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Desired-state schema for PostgreSQL (ADR-0023). Atlas Community Edition diffs this file
-- against the `public` schema of the target database and applies the difference:
-- `make db-apply DB_DIALECT=postgres`.
--
-- Edit this file and schema/sqlite.sql together; never write migration SQL by hand.
-- Tables follow docs/architecture/data-model.md, mapping logical types per its §1.2:
-- id is text COLLATE "C" (byte order, so that UUIDv7 strings sort by time under any database
-- collation); string, text, iri, lang, and enum are text; timestamp is timestamptz;
-- bool is boolean; bytes is bytea.
--
-- Atlas Community Edition does not manage extensions, sequences, views, or triggers;
-- keep them out of this file. Extensions go in schema/postgres.bootstrap.sql.

-- identity (data-model §4.1) -----------------------------------------------------------------

-- Resource: a login principal.
CREATE TABLE users (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  created_at timestamptz NOT NULL
);

-- Event (revision): the newest row of a user is the current password.
CREATE TABLE password_credentials (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  password_hash text NOT NULL,
  set_at timestamptz NOT NULL
);
CREATE INDEX password_credentials_user_id_id ON password_credentials (user_id, id DESC);

-- Resource: a role, such as the built-in admin and moderator.
CREATE TABLE roles (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  key text NOT NULL,
  position integer NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX roles_key ON roles (key);

-- Current value: the permissions a role grants.
CREATE TABLE role_permissions (
  role_id text COLLATE "C" NOT NULL,
  permission text NOT NULL CHECK (permission IN (
    'manage_reports', 'manage_accounts', 'manage_domains', 'manage_rules',
    'manage_settings', 'manage_roles', 'manage_invites', 'view_audit'
  )),
  PRIMARY KEY (role_id, permission)
);

-- Event: a role granted to a user. granted_by_user_id is NULL for a grant from the CLI.
CREATE TABLE user_role_grants (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  role_id text COLLATE "C" NOT NULL,
  granted_by_user_id text COLLATE "C",
  granted_at timestamptz NOT NULL
);

-- Event: a grant revoked. revoked_by_user_id is NULL for a revocation from the CLI.
CREATE TABLE user_role_revocations (
  grant_id text COLLATE "C" NOT NULL PRIMARY KEY,
  revoked_by_user_id text COLLATE "C",
  revoked_at timestamptz NOT NULL
);

-- Projection: the roles a user holds now (grants without a revocation).
CREATE TABLE user_roles (
  user_id text COLLATE "C" NOT NULL,
  role_id text COLLATE "C" NOT NULL,
  grant_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (user_id, role_id)
);

-- account (data-model §4.2) ------------------------------------------------------------------

-- Resource: a local or remote actor. domain is '' for local actors.
CREATE TABLE accounts (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text NOT NULL,
  domain text NOT NULL,
  actor_type text NOT NULL CHECK (actor_type IN ('person', 'service', 'application', 'group', 'organization')),
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX accounts_uri ON accounts (uri);
CREATE INDEX accounts_domain_id ON accounts (domain, id);
-- The only local Application actor is the instance actor.
CREATE UNIQUE INDEX accounts_instance_actor ON accounts (domain, actor_type) WHERE domain = '' AND actor_type = 'application';

-- Resource (subtype): a local account, owned by one user.
CREATE TABLE local_accounts (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  username text NOT NULL,
  username_normalized text NOT NULL
);
CREATE UNIQUE INDEX local_accounts_user_id ON local_accounts (user_id);
CREATE UNIQUE INDEX local_accounts_username_normalized ON local_accounts (username_normalized);

-- Projection: @username@domain resolution (WebFinger, FR-305, FR-401).
CREATE TABLE account_handles (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  username text NOT NULL,
  username_normalized text NOT NULL,
  domain text NOT NULL
);
CREATE UNIQUE INDEX account_handles_username_normalized_domain ON account_handles (username_normalized, domain);

-- Event (revision): one version of a profile.
CREATE TABLE account_profile_revisions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  account_id text COLLATE "C" NOT NULL,
  seq integer NOT NULL,
  display_name text NOT NULL,
  note_html text NOT NULL,
  locked boolean NOT NULL,
  bot boolean NOT NULL,
  discoverable boolean NOT NULL,
  indexable boolean NOT NULL,
  avatar_media_id text COLLATE "C",
  header_media_id text COLLATE "C",
  revised_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX account_profile_revisions_account_id_seq ON account_profile_revisions (account_id, seq);

-- Event (child of a revision): the profile's metadata fields (FR-006).
CREATE TABLE account_profile_fields (
  revision_id text COLLATE "C" NOT NULL,
  position integer NOT NULL,
  name text NOT NULL,
  value_html text NOT NULL,
  PRIMARY KEY (revision_id, position)
);

-- Projection: the current profile revision of each account.
CREATE TABLE account_profile_heads (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id text COLLATE "C" NOT NULL
);

-- Resource: a public signing key of an actor.
CREATE TABLE account_keys (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  account_id text COLLATE "C" NOT NULL,
  key_id text NOT NULL,
  algorithm text NOT NULL CHECK (algorithm IN ('rsa_sha256', 'ed25519')),
  public_key_pem text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX account_keys_key_id ON account_keys (key_id);
CREATE INDEX account_keys_account_id ON account_keys (account_id);

-- Resource (subtype): the sealed private key of a local actor's key (NFR-SEC-7).
CREATE TABLE local_account_private_keys (
  account_key_id text COLLATE "C" NOT NULL PRIMARY KEY,
  private_key_enc bytea NOT NULL
);

-- Event: a lifecycle transition of an account (self and system axis, data-model §5.1).
CREATE TABLE account_lifecycle_events (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  account_id text COLLATE "C" NOT NULL,
  kind text NOT NULL CHECK (kind IN (
    'deactivated', 'reactivated', 'deletion_scheduled', 'deletion_cancelled', 'deleted', 'remote_deleted'
  )),
  initiated_by text NOT NULL CHECK (initiated_by IN ('self', 'moderator', 'system', 'remote')),
  moderation_action_id text COLLATE "C",
  deletion_due_at timestamptz,
  occurred_at timestamptz NOT NULL,
  CHECK ((kind = 'deletion_scheduled') = (deletion_due_at IS NOT NULL))
);
CREATE INDEX account_lifecycle_events_account_id_id ON account_lifecycle_events (account_id, id DESC);

-- Projection: the current lifecycle state. An account without a row is active.
CREATE TABLE account_lifecycle_states (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  state text NOT NULL CHECK (state IN ('active', 'deactivated', 'deletion_scheduled', 'deleted')),
  since timestamptz NOT NULL,
  deletion_due_at timestamptz,
  CHECK ((state = 'deletion_scheduled') = (deletion_due_at IS NOT NULL))
);
CREATE INDEX account_lifecycle_states_state_deletion_due_at ON account_lifecycle_states (state, deletion_due_at);
