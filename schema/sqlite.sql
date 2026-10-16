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

-- Resource: an email address a user added.
CREATE TABLE user_email_addresses (
  id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  address TEXT NOT NULL,
  address_normalized TEXT NOT NULL,
  added_at INTEGER NOT NULL
) STRICT;
CREATE INDEX user_email_addresses_user_id_id ON user_email_addresses (user_id, id);

-- Projection: the primary email address of each user, for login and against duplicates.
CREATE TABLE user_primary_emails (
  user_id TEXT NOT NULL PRIMARY KEY,
  email_address_id TEXT NOT NULL,
  address_normalized TEXT NOT NULL
) STRICT;
CREATE UNIQUE INDEX user_primary_emails_address_normalized ON user_primary_emails (address_normalized);

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

-- graph: follows (data-model §4.5) ------------------------------------------------------------

-- Resource: one follow relationship. Following again after a rejection or termination inserts a
-- new row, so earlier relationships stay on record.
CREATE TABLE follows (
  id TEXT NOT NULL PRIMARY KEY,
  uri TEXT NOT NULL,
  follower_account_id TEXT NOT NULL,
  followee_account_id TEXT NOT NULL,
  requested_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX follows_uri ON follows (uri);

-- Event: a follow became active.
CREATE TABLE follow_acceptances (
  follow_id TEXT NOT NULL PRIMARY KEY,
  accepted_by TEXT NOT NULL CHECK (accepted_by IN ('auto', 'followee', 'remote')),
  activity_uri TEXT,
  accepted_at INTEGER NOT NULL
) STRICT;

-- Event: a pending follow was declined.
CREATE TABLE follow_rejections (
  follow_id TEXT NOT NULL PRIMARY KEY,
  rejected_by TEXT NOT NULL CHECK (rejected_by IN ('followee', 'remote', 'system')),
  activity_uri TEXT,
  rejected_at INTEGER NOT NULL
) STRICT;

-- Event: a pending or active follow ended.
CREATE TABLE follow_terminations (
  follow_id TEXT NOT NULL PRIMARY KEY,
  reason TEXT NOT NULL CHECK (reason IN ('unfollowed', 'removed_by_followee', 'blocked', 'account_deleted', 'moderation')),
  terminated_at INTEGER NOT NULL
) STRICT;

-- Projection: the live follow of each pair (neither rejected nor terminated).
CREATE TABLE follow_edges (
  follower_account_id TEXT NOT NULL,
  followee_account_id TEXT NOT NULL,
  follow_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('pending', 'active')),
  since INTEGER NOT NULL,
  PRIMARY KEY (follower_account_id, followee_account_id)
) STRICT;
CREATE INDEX follow_edges_followee_state_follow_id ON follow_edges (followee_account_id, state, follow_id);
CREATE INDEX follow_edges_follower_state_follow_id ON follow_edges (follower_account_id, state, follow_id);

-- Current value: the follower's settings of a follow.
CREATE TABLE follow_preferences (
  follow_id TEXT NOT NULL PRIMARY KEY,
  show_boosts INTEGER NOT NULL CHECK (show_boosts IN (0, 1)),
  notify INTEGER NOT NULL CHECK (notify IN (0, 1))
) STRICT;

-- Projection: the numbers of active follows into and out of an account. No row means zero.
CREATE TABLE follow_counters (
  account_id TEXT NOT NULL PRIMARY KEY,
  followers_count INTEGER NOT NULL CHECK (followers_count >= 0),
  following_count INTEGER NOT NULL CHECK (following_count >= 0)
) STRICT;

-- Tables follow docs/architecture/data-model.md, mapping logical types per its §1.2.

-- ---------------------------------------------------------------------------------------------
-- 基盤: job queue and Transactional Outbox (data-model.md §4.10, ADR-0004).
-- Owned by internal/adapter/queue/db.
-- ---------------------------------------------------------------------------------------------

-- Jobs written in a domain transaction, waiting for the relay to enqueue them.
CREATE TABLE outbox_events (
  id TEXT NOT NULL PRIMARY KEY,
  job_type TEXT NOT NULL,
  payload TEXT NOT NULL,
  idempotency_key TEXT,
  ordering_key TEXT,
  run_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;

-- Pending and running jobs of the database queue. A job is deleted when it finishes and moved
-- to dead_jobs when it fails for good. A claim sets locked_by and locked_until together.
CREATE TABLE jobs (
  id TEXT NOT NULL PRIMARY KEY,
  type TEXT NOT NULL,
  payload TEXT NOT NULL,
  idempotency_key TEXT,
  ordering_key TEXT,
  attempt INTEGER NOT NULL CHECK (attempt >= 0),
  run_at INTEGER NOT NULL,
  locked_by TEXT,
  locked_until INTEGER,
  last_error TEXT,
  created_at INTEGER NOT NULL,
  CHECK ((locked_by IS NULL) = (locked_until IS NULL))
) STRICT;
CREATE UNIQUE INDEX jobs_idempotency_key ON jobs (idempotency_key);
CREATE INDEX jobs_run_at ON jobs (run_at);
CREATE INDEX jobs_ordering_key_run_at ON jobs (ordering_key, run_at);

-- Jobs that failed for good, kept for the admin screen and the CLI to requeue.
CREATE TABLE dead_jobs (
  id TEXT NOT NULL PRIMARY KEY,
  type TEXT NOT NULL,
  payload TEXT NOT NULL,
  idempotency_key TEXT,
  ordering_key TEXT,
  attempt INTEGER NOT NULL,
  last_error TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  failed_at INTEGER NOT NULL
) STRICT;
CREATE INDEX dead_jobs_failed_at ON dead_jobs (failed_at);

-- ---------------------------------------------------------------------------------------------
-- identity: user preferences (data-model.md §4.1, FR-007, FR-208).
-- ---------------------------------------------------------------------------------------------

-- Current value: a user's settings. locale and default_post_language are canonical BCP 47
-- tags; time_zone is an IANA name; require_media_description is 0/1.
CREATE TABLE user_preferences (
  user_id TEXT NOT NULL PRIMARY KEY,
  locale TEXT NOT NULL,
  time_zone TEXT NOT NULL,
  default_visibility TEXT NOT NULL CHECK (default_visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  default_post_language TEXT NOT NULL,
  require_media_description INTEGER NOT NULL CHECK (require_media_description IN (0, 1)),
  updated_at INTEGER NOT NULL
) STRICT;

-- Current value: the languages a user's timelines show; no rows show every language.
CREATE TABLE user_timeline_languages (
  user_id TEXT NOT NULL,
  language TEXT NOT NULL,
  PRIMARY KEY (user_id, language)
) STRICT;
