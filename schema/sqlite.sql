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
    'manage_settings', 'manage_roles', 'manage_invites', 'view_audit', 'manage_jobs'
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
  fingerprint BLOB NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX account_keys_key_id_fingerprint ON account_keys (key_id, fingerprint);
CREATE INDEX account_keys_key_id_id ON account_keys (key_id, id DESC);
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

-- content: statuses (data-model §4.4) --------------------------------------------------------

-- Resource: a status. Visibility, author, reply target, and publication time never change.
CREATE TABLE statuses (
  id TEXT NOT NULL PRIMARY KEY,
  uri TEXT NOT NULL,
  url TEXT,
  author_account_id TEXT NOT NULL,
  is_local INTEGER NOT NULL CHECK (is_local IN (0, 1)),
  visibility TEXT NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  in_reply_to_uri TEXT,
  conversation_uri TEXT,
  published_at INTEGER NOT NULL,
  -- local statuses are never federated, so only this instance has them (ADR-0017).
  CHECK (visibility <> 'local' OR is_local = 1)
) STRICT;
CREATE UNIQUE INDEX statuses_uri ON statuses (uri);
CREATE INDEX statuses_author_account_id_id ON statuses (author_account_id, id);
CREATE INDEX statuses_in_reply_to_uri ON statuses (in_reply_to_uri);
CREATE INDEX statuses_conversation_uri_id ON statuses (conversation_uri, id);

-- Resource (subtype): a status posted on this instance, with the client API's Idempotency-Key.
CREATE TABLE local_statuses (
  status_id TEXT NOT NULL PRIMARY KEY,
  author_account_id TEXT NOT NULL,
  client_id TEXT,
  idempotency_key TEXT
) STRICT;
CREATE UNIQUE INDEX local_statuses_author_account_id_idempotency_key ON local_statuses (author_account_id, idempotency_key);

-- Event (revision): one version of a status's content (FR-109).
CREATE TABLE status_revisions (
  id TEXT NOT NULL PRIMARY KEY,
  status_id TEXT NOT NULL,
  seq INTEGER NOT NULL CHECK (seq >= 1),
  spoiler_text TEXT NOT NULL,
  sensitive INTEGER NOT NULL CHECK (sensitive IN (0, 1)),
  language TEXT NOT NULL,
  revised_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX status_revisions_status_id_seq ON status_revisions (status_id, seq);

-- Event (child of a revision): the content per language, normalized from contentMap (NFR-I18N-5).
CREATE TABLE status_revision_bodies (
  revision_id TEXT NOT NULL,
  language TEXT NOT NULL,
  content_html TEXT NOT NULL,
  PRIMARY KEY (revision_id, language)
) STRICT;

-- Event (subtype of a revision): the source text a local author typed.
CREATE TABLE status_revision_sources (
  revision_id TEXT NOT NULL PRIMARY KEY,
  source_text TEXT NOT NULL,
  source_format TEXT NOT NULL CHECK (source_format IN ('text_plain'))
) STRICT;

-- Event (child of a revision): the attached media, with per-revision alt text (FR-105).
CREATE TABLE status_revision_attachments (
  revision_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 3),
  media_id TEXT NOT NULL,
  description TEXT,
  PRIMARY KEY (revision_id, position)
) STRICT;

-- Event (child of a revision): the mentioned accounts.
CREATE TABLE status_revision_mentions (
  revision_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  PRIMARY KEY (revision_id, account_id)
) STRICT;

-- Event (child of a revision): the hashtags.
CREATE TABLE status_revision_tags (
  revision_id TEXT NOT NULL,
  tag_id TEXT NOT NULL,
  PRIMARY KEY (revision_id, tag_id)
) STRICT;

-- Event (child of a revision): the custom emoji.
CREATE TABLE status_revision_emojis (
  revision_id TEXT NOT NULL,
  emoji_id TEXT NOT NULL,
  PRIMARY KEY (revision_id, emoji_id)
) STRICT;

-- Event: the deletion of a status. It outlives the purge as the tombstone of the URI.
CREATE TABLE status_deletions (
  status_id TEXT NOT NULL PRIMARY KEY,
  uri TEXT NOT NULL,
  author_account_id TEXT NOT NULL,
  origin TEXT NOT NULL CHECK (origin IN ('author', 'moderation', 'remote', 'account_deletion', 'retention')),
  moderation_action_id TEXT,
  deleted_at INTEGER NOT NULL
) STRICT;
CREATE INDEX status_deletions_uri ON status_deletions (uri);
-- A deletion other than by retention keeps its URI from coming back, so there is at most one.
-- A remote status deleted by retention may be fetched again and deleted again (ADR-0034).
CREATE UNIQUE INDEX status_deletions_tombstone_uri ON status_deletions (uri) WHERE origin <> 'retention';

-- Resource: a hashtag, unique by its normalized name.
CREATE TABLE tags (
  id TEXT NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  name_normalized TEXT NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX tags_name_normalized ON tags (name_normalized);

-- Projection: the current revision of each status that is not deleted.
CREATE TABLE status_heads (
  status_id TEXT NOT NULL PRIMARY KEY,
  revision_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  last_revised_at INTEGER NOT NULL
) STRICT;

-- Projection: the known parent of each reply, neither of them deleted (FR-106).
CREATE TABLE status_reply_links (
  status_id TEXT NOT NULL PRIMARY KEY,
  parent_status_id TEXT NOT NULL,
  parent_author_account_id TEXT NOT NULL
) STRICT;
CREATE INDEX status_reply_links_parent_status_id_status_id ON status_reply_links (parent_status_id, status_id);

-- Projection: the accounts that the current revision of each status mentions.
CREATE TABLE status_mention_index (
  account_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  PRIMARY KEY (account_id, status_id)
) STRICT;
CREATE INDEX status_mention_index_status_id ON status_mention_index (status_id);

-- Projection: the counters of each status that is not deleted (data-model §2.3).
CREATE TABLE status_counters (
  status_id TEXT NOT NULL PRIMARY KEY,
  replies_count INTEGER NOT NULL CHECK (replies_count >= 0),
  boosts_count INTEGER NOT NULL CHECK (boosts_count >= 0),
  favourites_count INTEGER NOT NULL CHECK (favourites_count >= 0)
) STRICT;

-- Projection: the statuses of each account that are not deleted. No row means none.
CREATE TABLE account_post_counters (
  account_id TEXT NOT NULL PRIMARY KEY,
  statuses_count INTEGER NOT NULL CHECK (statuses_count > 0),
  last_status_at INTEGER NOT NULL
) STRICT;

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

-- graph: blocks and mutes (data-model §4.5) -------------------------------------------------

-- Resource: one block. Blocking again after a retraction inserts a new row. uri is NULL only for
-- a received Block without an id.
CREATE TABLE blocks (
  id TEXT NOT NULL PRIMARY KEY,
  uri TEXT,
  blocker_account_id TEXT NOT NULL,
  blocked_account_id TEXT NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX blocks_uri ON blocks (uri);

-- Event: a block was lifted.
CREATE TABLE block_retractions (
  block_id TEXT NOT NULL PRIMARY KEY,
  retracted_at INTEGER NOT NULL
) STRICT;

-- Projection: the block in force of each pair (without a retraction).
CREATE TABLE block_edges (
  blocker_account_id TEXT NOT NULL,
  blocked_account_id TEXT NOT NULL,
  block_id TEXT NOT NULL,
  PRIMARY KEY (blocker_account_id, blocked_account_id)
) STRICT;
CREATE INDEX block_edges_blocked_account_id ON block_edges (blocked_account_id);
CREATE INDEX block_edges_blocker_block_id ON block_edges (blocker_account_id, block_id);

-- Current value: an account's mute of another. A mute whose expires_at has passed no longer
-- applies; readers filter it out, and the hourly cleanup deletes it.
CREATE TABLE mutes (
  account_id TEXT NOT NULL,
  target_account_id TEXT NOT NULL,
  hide_notifications INTEGER NOT NULL CHECK (hide_notifications IN (0, 1)),
  expires_at INTEGER,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (account_id, target_account_id),
  CHECK (expires_at IS NULL OR expires_at > created_at)
) STRICT;
CREATE INDEX mutes_account_created_at_target ON mutes (account_id, created_at, target_account_id);
-- The periodic cleanup deletes the expired mutes (FR-803).
CREATE INDEX mutes_expires_at ON mutes (expires_at);

-- ---------------------------------------------------------------------------------------------
-- 基盤: job queue and Transactional Outbox (data-model.md §4.10, ADR-0004).
-- Owned by internal/adapter/queue/db.
-- ---------------------------------------------------------------------------------------------

-- Jobs written in a domain transaction, waiting for the relay to enqueue them.
CREATE TABLE outbox_events (
  id TEXT NOT NULL PRIMARY KEY,
  job_type TEXT NOT NULL,
  payload BLOB NOT NULL,
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
  payload BLOB NOT NULL,
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
  payload BLOB NOT NULL,
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
  -- Whether the moderator notifications (admin_report, admin_sign_up) also come by email.
  email_moderator_notifications INTEGER NOT NULL CHECK (email_moderator_notifications IN (0, 1)),
  updated_at INTEGER NOT NULL
) STRICT;

-- Current value: the languages a user's timelines show; no rows show every language.
CREATE TABLE user_timeline_languages (
  user_id TEXT NOT NULL,
  language TEXT NOT NULL,
  PRIMARY KEY (user_id, language)
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- instance: settings and texts (data-model §4.9). Owned by internal/app/instance.
-- Keys are defined in internal/domain/instance; add a key there and to its CHECK here together.
-- ---------------------------------------------------------------------------------------------

-- Event (revision): one change of a setting. value is the canonical string form of the key's
-- scalar type. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_setting_revisions (
  id TEXT NOT NULL PRIMARY KEY,
  key TEXT NOT NULL CHECK (key IN (
    'registration_mode', 'max_status_length', 'default_locale', 'local_timeline_enabled', 'federated_timeline_enabled',
    'preview_image', 'remote_media_policy', 'remote_media_cache_ttl', 'authorized_fetch',
    'domain_actions_visibility', 'domain_actions_rationale_visibility', 'appeal_window',
    'retention_session_origins', 'retention_registration_origins', 'retention_reports',
    'retention_moderation_actions', 'retention_audit_log'
  )),
  value TEXT NOT NULL,
  revised_by_user_id TEXT,
  revised_at INTEGER NOT NULL
) STRICT;
CREATE INDEX instance_setting_revisions_key_id ON instance_setting_revisions (key, id DESC);

-- Projection: the current value of each setting that has been set. Unset keys use the default
-- defined in code.
CREATE TABLE instance_settings (
  key TEXT NOT NULL PRIMARY KEY,
  revision_id TEXT NOT NULL,
  value TEXT NOT NULL
) STRICT;

-- Event (revision): one version of a localized text. An empty body clears the text in that
-- locale. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_text_revisions (
  id TEXT NOT NULL PRIMARY KEY,
  key TEXT NOT NULL CHECK (key IN (
    'name', 'short_description', 'description', 'terms', 'privacy_policy', 'preview_image_alt'
  )),
  locale TEXT NOT NULL,
  body TEXT NOT NULL,
  revised_by_user_id TEXT,
  revised_at INTEGER NOT NULL
) STRICT;
CREATE INDEX instance_text_revisions_key_locale_id ON instance_text_revisions (key, locale, id DESC);

-- Projection: the current revision of each text in each locale.
CREATE TABLE instance_texts (
  key TEXT NOT NULL,
  locale TEXT NOT NULL,
  revision_id TEXT NOT NULL,
  PRIMARY KEY (key, locale)
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- instance: rules (data-model §4.9, FR-705). Owned by internal/app/instance.
-- A rule keeps its ID across revisions, so reports cite it by rule_id; retired rules stay.
-- ---------------------------------------------------------------------------------------------

-- Resource: a rule of the instance.
CREATE TABLE instance_rules (
  id TEXT NOT NULL PRIMARY KEY,
  created_at INTEGER NOT NULL
) STRICT;

-- Event (revision): one version of a rule, with its position and texts. revised_by_user_id is
-- NULL for a change from the CLI.
CREATE TABLE instance_rule_revisions (
  id TEXT NOT NULL PRIMARY KEY,
  rule_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  revised_by_user_id TEXT,
  revised_at INTEGER NOT NULL
) STRICT;
CREATE INDEX instance_rule_revisions_rule_id_id ON instance_rule_revisions (rule_id, id DESC);

-- Event (child of a revision): the text of a rule revision in one locale. An empty hint means none.
CREATE TABLE instance_rule_texts (
  rule_revision_id TEXT NOT NULL,
  locale TEXT NOT NULL,
  text TEXT NOT NULL,
  hint TEXT NOT NULL,
  PRIMARY KEY (rule_revision_id, locale)
) STRICT;

-- Event: the retirement of a rule. retired_by_user_id is NULL for the CLI.
CREATE TABLE instance_rule_retirements (
  rule_id TEXT NOT NULL PRIMARY KEY,
  retired_by_user_id TEXT,
  retired_at INTEGER NOT NULL
) STRICT;

-- Projection: the current revision and position of each rule in effect (not retired).
CREATE TABLE instance_rule_heads (
  rule_id TEXT NOT NULL PRIMARY KEY,
  revision_id TEXT NOT NULL,
  position INTEGER NOT NULL
) STRICT;
CREATE INDEX instance_rule_heads_position ON instance_rule_heads (position);

-- ---------------------------------------------------------------------------------------------
-- content: media files (data-model §4.4, ADR-0005, ADR-0019 §8). Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Resource: a media file, uploaded by a local account or fetched from a remote server.
CREATE TABLE media_files (
  id TEXT NOT NULL PRIMARY KEY,
  owner_account_id TEXT NOT NULL,
  origin TEXT NOT NULL CHECK (origin IN ('upload', 'remote')),
  created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX media_files_owner_account_id_id ON media_files (owner_account_id, id);

-- Resource (subtype): an upload as received. content_type is detected from the content. The
-- blob under blob_key is private and is deleted once processing has a result.
CREATE TABLE uploaded_media (
  media_id TEXT NOT NULL PRIMARY KEY,
  blob_key TEXT NOT NULL,
  content_type TEXT NOT NULL,
  file_size INTEGER NOT NULL CHECK (file_size >= 0)
) STRICT;

-- Event: one processing of a media file. The newest row of a file is its current state.
CREATE TABLE media_processing_results (
  id TEXT NOT NULL PRIMARY KEY,
  media_id TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('ready', 'failed')),
  media_type TEXT NOT NULL CHECK (media_type IN ('image', 'gifv', 'video', 'audio', 'unknown')),
  width INTEGER,
  height INTEGER,
  duration_ms INTEGER,
  blurhash TEXT,
  processed_at INTEGER NOT NULL
) STRICT;
CREATE INDEX media_processing_results_media_id_id ON media_processing_results (media_id, id DESC);

-- Event (child of a result): a processed rendition in blob storage, under the public prefix.
CREATE TABLE media_variants (
  result_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('full', 'thumbnail')),
  blob_key TEXT NOT NULL,
  content_type TEXT NOT NULL,
  file_size INTEGER NOT NULL CHECK (file_size >= 0),
  width INTEGER NOT NULL,
  height INTEGER NOT NULL,
  PRIMARY KEY (result_id, kind)
) STRICT;

-- Current value: the focal point of a media file. No row means the center.
CREATE TABLE media_focus (
  media_id TEXT NOT NULL PRIMARY KEY,
  focus_x REAL NOT NULL CHECK (focus_x BETWEEN -1.0 AND 1.0),
  focus_y REAL NOT NULL CHECK (focus_y BETWEEN -1.0 AND 1.0)
) STRICT;

-- Current value: the description that the uploader set on an upload, which a post copies into
-- its attachment (client API #112). No row means none.
CREATE TABLE media_descriptions (
  media_id TEXT NOT NULL PRIMARY KEY,
  description TEXT NOT NULL CHECK (description <> '')
) STRICT;

-- The cleanup of unattached uploads (FR-803) looks up the references to a media file.
CREATE INDEX status_revision_attachments_media_id ON status_revision_attachments (media_id);
CREATE INDEX account_profile_revisions_avatar_media_id ON account_profile_revisions (avatar_media_id);
CREATE INDEX account_profile_revisions_header_media_id ON account_profile_revisions (header_media_id);

-- ---------------------------------------------------------------------------------------------
-- identity: browser sessions of the server-rendered screens (data-model.md §4.1, ADR-0019 §4).
-- ---------------------------------------------------------------------------------------------

-- Resource: a sign-in. token_hash is the SHA-256 digest of the session cookie's value.
CREATE TABLE sessions (
  id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  token_hash BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX sessions_token_hash ON sessions (token_hash);
CREATE INDEX sessions_user_id ON sessions (user_id);

-- Event: a session ended before it expired.
CREATE TABLE session_revocations (
  session_id TEXT NOT NULL PRIMARY KEY,
  reason TEXT NOT NULL CHECK (reason IN ('logout', 'replaced', 'password_changed', 'moderation', 'expired_cleanup', 'account_deleted')),
  revoked_at INTEGER NOT NULL
) STRICT;

-- Current value: when a session was last used, updated at most once per interval.
CREATE TABLE session_activity (
  session_id TEXT NOT NULL PRIMARY KEY,
  last_seen_at INTEGER NOT NULL
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- identity: OAuth 2.0 authorization server (data-model.md §4.1, ADR-0019 §4, ADR-0030).
-- Every client_id column except oauth_clients.client_id refers to oauth_clients.id.
-- ---------------------------------------------------------------------------------------------

-- Resource: a registered client. client_secret_hash is NULL for public clients.
CREATE TABLE oauth_clients (
  id TEXT NOT NULL PRIMARY KEY,
  client_id TEXT NOT NULL,
  client_secret_hash BLOB,
  name TEXT NOT NULL,
  website TEXT,
  scope TEXT NOT NULL,
  is_first_party INTEGER NOT NULL CHECK (is_first_party IN (0, 1)),
  owner_user_id TEXT,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX oauth_clients_client_id ON oauth_clients (client_id);

-- Current value: the redirect URIs a client may use, matched exactly.
CREATE TABLE oauth_client_redirect_uris (
  client_id TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  PRIMARY KEY (client_id, redirect_uri)
) STRICT;

-- Resource: an authorization code, keyed by its SHA-256 digest. session_id is the browser session
-- that authorized it (NULL when it was issued outside one).
CREATE TABLE oauth_authorization_codes (
  code_hash BLOB NOT NULL PRIMARY KEY,
  client_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  scope TEXT NOT NULL,
  code_challenge TEXT NOT NULL,
  session_id TEXT,
  issued_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
) STRICT;

-- Event: a code was exchanged; the primary key makes it one-time. token_id is the first token
-- of the family it started.
CREATE TABLE oauth_authorization_code_redemptions (
  code_hash BLOB NOT NULL PRIMARY KEY,
  token_id TEXT NOT NULL,
  redeemed_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX oauth_authorization_code_redemptions_token_id ON oauth_authorization_code_redemptions (token_id);

-- Resource: an access token with its refresh token. A rotation issues a new row in the same
-- family (family_id is the id of the family's first row).
CREATE TABLE oauth_access_tokens (
  id TEXT NOT NULL PRIMARY KEY,
  family_id TEXT NOT NULL,
  client_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  access_token_hash BLOB NOT NULL,
  refresh_token_hash BLOB,
  scope TEXT NOT NULL,
  rotated_from_token_id TEXT,
  issued_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  refresh_expires_at INTEGER,
  CONSTRAINT oauth_access_tokens_refresh_pair CHECK ((refresh_token_hash IS NULL) = (refresh_expires_at IS NULL))
) STRICT;
CREATE UNIQUE INDEX oauth_access_tokens_access_token_hash ON oauth_access_tokens (access_token_hash);
CREATE UNIQUE INDEX oauth_access_tokens_refresh_token_hash ON oauth_access_tokens (refresh_token_hash);
CREATE INDEX oauth_access_tokens_family_id ON oauth_access_tokens (family_id);
CREATE INDEX oauth_access_tokens_user_id ON oauth_access_tokens (user_id);

-- Event: a token row (its access and refresh token) stopped working.
CREATE TABLE oauth_token_revocations (
  token_id TEXT NOT NULL PRIMARY KEY,
  reason TEXT NOT NULL CHECK (reason IN ('user', 'rotation', 'refresh_reuse', 'code_reuse', 'password_changed', 'moderation', 'account_deleted')),
  revoked_at INTEGER NOT NULL
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- 基盤: key-value store (data-model.md §4.10). Owned by internal/adapter/kv/db.
-- ---------------------------------------------------------------------------------------------

-- Entries of the KeyValueStore port. expires_at is NULL for entries that never expire.
CREATE TABLE kv_entries (
  namespace TEXT NOT NULL,
  key TEXT NOT NULL,
  value BLOB NOT NULL,
  expires_at INTEGER,
  PRIMARY KEY (namespace, key)
) STRICT;
CREATE INDEX kv_entries_expires_at ON kv_entries (expires_at);

-- ---------------------------------------------------------------------------------------------
-- identity: registration, invites, email verification, and one-time tokens
-- (data-model.md §4.1, FR-001, FR-002).
-- ---------------------------------------------------------------------------------------------

-- Event: an email address was verified, at most once per address.
CREATE TABLE user_email_verifications (
  email_address_id TEXT NOT NULL PRIMARY KEY,
  verified_at INTEGER NOT NULL
) STRICT;

-- Resource: a sign-up from the registration page, one per user. reason is the applicant's
-- answer in approval mode; invite_id is the invite it redeemed in invite mode.
CREATE TABLE registration_applications (
  id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  reason TEXT,
  invite_id TEXT,
  submitted_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX registration_applications_user_id ON registration_applications (user_id);

-- Event: an application was approved or rejected. decided_by_user_id is NULL for the system
-- and for a decision from the CLI.
CREATE TABLE registration_decisions (
  id TEXT NOT NULL PRIMARY KEY,
  application_id TEXT NOT NULL,
  decision TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
  decided_by TEXT NOT NULL CHECK (decided_by IN ('moderator', 'system')),
  decided_by_user_id TEXT,
  note TEXT,
  decided_at INTEGER NOT NULL
) STRICT;
CREATE INDEX registration_decisions_application_id_id ON registration_decisions (application_id, id DESC);

-- Projection: the registration state of each user who signed up; users without a row (created
-- with the CLI) are approved.
CREATE TABLE user_registration_states (
  user_id TEXT NOT NULL PRIMARY KEY,
  state TEXT NOT NULL CHECK (state IN ('pending', 'approved', 'rejected')),
  since INTEGER NOT NULL
) STRICT;
CREATE INDEX user_registration_states_state_since ON user_registration_states (state, since);

-- Resource: an invite code. created_by_user_id is NULL for an invite from the CLI; max_uses
-- and expires_at are NULL for no limit.
CREATE TABLE invites (
  id TEXT NOT NULL PRIMARY KEY,
  code TEXT NOT NULL,
  created_by_user_id TEXT,
  max_uses INTEGER,
  expires_at INTEGER,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX invites_code ON invites (code);

-- Event: one use of an invite. seq is 1 + the uses before it, so that the primary key keeps
-- concurrent sign-ups from exceeding max_uses.
CREATE TABLE invite_redemptions (
  invite_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  redeemed_at INTEGER NOT NULL,
  PRIMARY KEY (invite_id, seq)
) STRICT;

-- Event: an invite was revoked. revoked_by_user_id is NULL for a revocation from the CLI.
CREATE TABLE invite_revocations (
  invite_id TEXT NOT NULL PRIMARY KEY,
  revoked_by_user_id TEXT,
  revoked_at INTEGER NOT NULL
) STRICT;

-- Resource: a one-time token sent to a user. token_hash is the SHA-256 digest of the token.
CREATE TABLE user_tokens (
  id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  purpose TEXT NOT NULL CHECK (purpose IN ('email_verification', 'password_reset')),
  email_address_id TEXT,
  token_hash BLOB NOT NULL,
  expires_at INTEGER NOT NULL,
  issued_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX user_tokens_token_hash ON user_tokens (token_hash);

-- Event: a token was used; the primary key makes every token single-use.
CREATE TABLE user_token_redemptions (
  token_id TEXT NOT NULL PRIMARY KEY,
  redeemed_at INTEGER NOT NULL
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- content: boosts, favourites, bookmarks, and pins (data-model §4.4, FR-107, FR-113).
-- Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Resource: a boost (Announce) of a status. visibility copies the status's and never changes;
-- a boost of a local status stays on this instance (ADR-0017).
CREATE TABLE boosts (
  id TEXT NOT NULL PRIMARY KEY,
  uri TEXT NOT NULL,
  booster_account_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  visibility TEXT NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  is_local INTEGER NOT NULL CHECK (is_local IN (0, 1)),
  boosted_at INTEGER NOT NULL,
  -- only local accounts boost local statuses, and such boosts never federate (ADR-0017).
  CHECK (visibility <> 'local' OR is_local = 1)
) STRICT;
CREATE UNIQUE INDEX boosts_uri ON boosts (uri);
CREATE INDEX boosts_status_id ON boosts (status_id);

-- Event: a boost was retracted (Undo Announce). A boost is retracted at most once.
CREATE TABLE boost_retractions (
  boost_id TEXT NOT NULL PRIMARY KEY,
  origin TEXT NOT NULL CHECK (origin IN ('booster', 'status_deleted', 'moderation', 'remote')),
  retracted_at INTEGER NOT NULL
) STRICT;

-- Projection: the live boost of each pair, not retracted, of a status that is not deleted.
CREATE TABLE boost_edges (
  booster_account_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  boost_id TEXT NOT NULL,
  PRIMARY KEY (booster_account_id, status_id)
) STRICT;
CREATE INDEX boost_edges_status_id ON boost_edges (status_id);

-- Current value: an account's favourite (Like) of a status.
CREATE TABLE favourites (
  account_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  uri TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  id TEXT NOT NULL,
  PRIMARY KEY (account_id, status_id)
) STRICT;
CREATE UNIQUE INDEX favourites_uri ON favourites (uri);
CREATE INDEX favourites_account_id_id ON favourites (account_id, id);
CREATE INDEX favourites_status_id_id ON favourites (status_id, id);

-- Current value: an account's private bookmark of a status.
CREATE TABLE bookmarks (
  account_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  id TEXT NOT NULL,
  PRIMARY KEY (account_id, status_id)
) STRICT;
CREATE INDEX bookmarks_account_id_id ON bookmarks (account_id, id);
-- The retention purge keeps a bookmarked remote status (ADR-0034).
CREATE INDEX bookmarks_status_id ON bookmarks (status_id);

-- Current value: a status its author pinned to the profile (the Featured collection). Each pin
-- holds one of the account's slots 1..5 (content.MaxPins), so the unique slot index caps the pins
-- of an account even under concurrent inserts.
CREATE TABLE status_pins (
  account_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  slot INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 5),
  created_at INTEGER NOT NULL,
  id TEXT NOT NULL,
  PRIMARY KEY (account_id, status_id)
) STRICT;
CREATE UNIQUE INDEX status_pins_account_id_slot ON status_pins (account_id, slot);
CREATE INDEX status_pins_account_id_id ON status_pins (account_id, id);

-- ---------------------------------------------------------------------------------------------
-- timeline: feeds (data-model §4.6, §6, ADR-0013). Owned by internal/app/timeline, whose
-- synchronous projectors derive every table from the events of content and graph (and of
-- moderation for the restrictions); the timeline queries read only these tables.
-- ---------------------------------------------------------------------------------------------

-- Projection: one entry per status (kind post, id = status ID) and per live boost (kind boost,
-- id = boost ID) of a status that is not deleted. author_* is the poster or the booster,
-- original_author_* the author of the status; a domain is '' for a local account.
CREATE TABLE feed_entries (
  id TEXT NOT NULL PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('post', 'boost')),
  status_id TEXT NOT NULL,
  author_account_id TEXT NOT NULL,
  author_domain TEXT NOT NULL,
  original_author_account_id TEXT NOT NULL,
  original_author_domain TEXT NOT NULL,
  visibility TEXT NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  is_local INTEGER NOT NULL CHECK (is_local IN (0, 1)),
  language TEXT NOT NULL,
  revision_seq INTEGER NOT NULL CHECK (revision_seq >= 1),
  is_reply INTEGER NOT NULL CHECK (is_reply IN (0, 1))
) STRICT;
CREATE INDEX feed_entries_author_account_id_id ON feed_entries (author_account_id, id);
CREATE INDEX feed_entries_status_id ON feed_entries (status_id);
CREATE INDEX feed_entries_local ON feed_entries (id) WHERE is_local = 1 AND kind = 'post' AND visibility IN ('public', 'local');
CREATE INDEX feed_entries_public ON feed_entries (id) WHERE kind = 'post' AND visibility = 'public';

-- Projection: the recipients (current mentions) of a direct post.
CREATE TABLE feed_direct_recipients (
  entry_id TEXT NOT NULL,
  recipient_account_id TEXT NOT NULL,
  PRIMARY KEY (recipient_account_id, entry_id)
) STRICT;
CREATE INDEX feed_direct_recipients_entry_id ON feed_direct_recipients (entry_id);

-- Projection: the current hashtags of a post, with its immutable author and visibility.
CREATE TABLE feed_tag_entries (
  tag_id TEXT NOT NULL,
  entry_id TEXT NOT NULL,
  author_account_id TEXT NOT NULL,
  visibility TEXT NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  is_local INTEGER NOT NULL CHECK (is_local IN (0, 1)),
  PRIMARY KEY (tag_id, entry_id)
) STRICT;
CREATE INDEX feed_tag_entries_entry_id ON feed_tag_entries (entry_id);

-- Projection: the active follows, with the follower's show_boosts preference.
CREATE TABLE feed_follow_edges (
  follower_account_id TEXT NOT NULL,
  followee_account_id TEXT NOT NULL,
  show_boosts INTEGER NOT NULL CHECK (show_boosts IN (0, 1)),
  PRIMARY KEY (follower_account_id, followee_account_id)
) STRICT;

-- Projection: the accounts whose entries a viewer does not see: those the viewer blocks, those
-- that block the viewer, and those the viewer mutes (until expires_at; NULL never expires).
CREATE TABLE feed_exclusions (
  viewer_account_id TEXT NOT NULL,
  target_account_id TEXT NOT NULL,
  reason TEXT NOT NULL CHECK (reason IN ('block', 'blocked_by', 'mute')),
  expires_at INTEGER,
  PRIMARY KEY (viewer_account_id, target_account_id, reason)
) STRICT;

-- Projection: the moderation restrictions of accounts that bear on timelines, until
-- effective_until (NULL: until lifted).
CREATE TABLE feed_author_restrictions (
  account_id TEXT NOT NULL,
  restriction TEXT NOT NULL CHECK (restriction IN ('profile_view', 'visibility_ceiling_private', 'public_discovery')),
  effective_until INTEGER,
  PRIMARY KEY (account_id, restriction)
) STRICT;

-- Projection: the moderation restrictions of domains (and their subdomains) that bear on
-- timelines.
CREATE TABLE feed_domain_restrictions (
  domain TEXT NOT NULL,
  restriction TEXT NOT NULL CHECK (restriction IN ('profile_view', 'visibility_ceiling_private', 'public_discovery')),
  effective_until INTEGER,
  PRIMARY KEY (domain, restriction)
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- notification: notifications, their dismissals, read markers, and the exclusions and deleted
-- statuses that hide them (data-model §4.7, FR-205).
-- Owned by internal/app/notification.
-- ---------------------------------------------------------------------------------------------

-- Event: something happened to a local account. dedupe_key makes generating it again from the
-- same domain event, or from an action the source repeats, insert nothing.
CREATE TABLE notifications (
  id TEXT NOT NULL PRIMARY KEY,
  recipient_account_id TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN (
    'follow', 'follow_request', 'follow_accepted', 'mention', 'boost', 'favourite', 'poll_closed', 'moderation_notice',
    'report_resolved', 'admin_report', 'admin_sign_up', 'relationships_severed', 'appeal_decided',
    'admin_appeal'
  )),
  source_account_id TEXT,
  status_id TEXT,
  moderation_action_id TEXT,
  -- admin_report, report_resolved: the report.
  report_id TEXT,
  -- admin_sign_up: the user whose registration awaits approval.
  subject_user_id TEXT,
  -- relationships_severed: the relationship_severances row.
  severance_id TEXT,
  -- appeal_decided, admin_appeal: the appeal.
  appeal_id TEXT,
  dedupe_key TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  -- nobody is notified of their own actions.
  CHECK (source_account_id IS NULL OR source_account_id <> recipient_account_id)
) STRICT;
CREATE UNIQUE INDEX notifications_recipient_account_id_dedupe_key ON notifications (recipient_account_id, dedupe_key);
CREATE INDEX notifications_recipient_account_id_id ON notifications (recipient_account_id, id);
CREATE INDEX notifications_recipient_account_id_type_id ON notifications (recipient_account_id, type, id);
-- The retention purge keeps a remote status that a notification is about (ADR-0034).
CREATE INDEX notifications_status_id ON notifications (status_id);

-- Event: the recipient dismissed a notification. A notification is dismissed at most once.
CREATE TABLE notification_dismissals (
  notification_id TEXT NOT NULL PRIMARY KEY,
  dismissed_at INTEGER NOT NULL
) STRICT;

-- Current value: the newest notification an account has read; greater IDs are unread.
CREATE TABLE notification_read_markers (
  account_id TEXT NOT NULL PRIMARY KEY,
  last_read_id TEXT NOT NULL
) STRICT;

-- Projection of the graph events: while in force (expires_at NULL or later than now), the
-- notifications from target_account_id are hidden from viewer_account_id at read time.
CREATE TABLE notification_exclusions (
  viewer_account_id TEXT NOT NULL,
  target_account_id TEXT NOT NULL,
  reason TEXT NOT NULL CHECK (reason IN ('block', 'blocked_by', 'mute')),
  expires_at INTEGER,
  PRIMARY KEY (viewer_account_id, target_account_id, reason)
) STRICT;

-- Projection of content.status_deleted: the notifications about these statuses are hidden at
-- read time; the erasure job deletes them and the row.
CREATE TABLE notification_deleted_statuses (
  status_id TEXT NOT NULL PRIMARY KEY
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- account: what the owner of a local profile typed (data-model §4.2). Owned by
-- internal/app/account; the client API returns it for editing the profile.
-- ---------------------------------------------------------------------------------------------

-- Event (subtype of account_profile_revisions): the profile text of a local revision as typed,
-- from which note_html is rendered.
CREATE TABLE account_profile_revision_sources (
  revision_id TEXT NOT NULL PRIMARY KEY,
  note_source TEXT NOT NULL
) STRICT;

-- Event (subtype of account_profile_fields): the value of a local field as typed, from which
-- value_html is rendered.
CREATE TABLE account_profile_field_sources (
  revision_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value_source TEXT NOT NULL,
  PRIMARY KEY (revision_id, position)
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- account: remote actors and key rotation (data-model §4.2, docs/architecture/federation.md).
-- Owned by internal/app/account.
-- ---------------------------------------------------------------------------------------------

-- Resource (subtype): an actor of another server.
CREATE TABLE remote_accounts (
  account_id TEXT NOT NULL PRIMARY KEY,
  first_seen_at INTEGER NOT NULL
) STRICT;

-- Event: one fetch of a remote actor's document whose content differed from the previous one.
-- document_digest is the SHA-256 of the document.
CREATE TABLE remote_account_snapshots (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  preferred_username TEXT NOT NULL,
  profile_url TEXT,
  inbox_url TEXT NOT NULL,
  shared_inbox_url TEXT,
  outbox_url TEXT,
  followers_url TEXT,
  following_url TEXT,
  featured_url TEXT,
  followers_total INTEGER,
  following_total INTEGER,
  statuses_total INTEGER,
  document_digest BLOB NOT NULL,
  fetched_at INTEGER NOT NULL
) STRICT;
CREATE INDEX remote_account_snapshots_account_id_id ON remote_account_snapshots (account_id, id DESC);

-- Projection: the newest snapshot of each remote actor, with the endpoints delivery reads and
-- the fetch state of the periodic refresh.
CREATE TABLE remote_account_heads (
  account_id TEXT NOT NULL PRIMARY KEY,
  snapshot_id TEXT NOT NULL,
  inbox_url TEXT NOT NULL,
  shared_inbox_url TEXT,
  last_fetched_at INTEGER NOT NULL,
  consecutive_fetch_failures INTEGER NOT NULL CHECK (consecutive_fetch_failures >= 0)
) STRICT;
CREATE INDEX remote_account_heads_last_fetched_at ON remote_account_heads (last_fetched_at);
CREATE INDEX remote_account_heads_shared_inbox_url ON remote_account_heads (shared_inbox_url);

-- Event: a WebFinger check of acct:{username}@{domain}. verified means that its self link points
-- at the actor's uri; mismatch means that it answered otherwise, and the account has no handle
-- until a later check verifies one.
CREATE TABLE remote_account_handle_verifications (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  username TEXT NOT NULL,
  domain TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('verified', 'mismatch')),
  checked_at INTEGER NOT NULL
) STRICT;
CREATE INDEX remote_account_handle_verifications_account_id_id ON remote_account_handle_verifications (account_id, id DESC);

-- Event (subtype of account_profile_revisions): the snapshot a remote revision came from and the
-- version of the HTML policy that sanitized its note_html and value_html (ADR-0019 §2).
CREATE TABLE remote_profile_revisions (
  revision_id TEXT NOT NULL PRIMARY KEY,
  snapshot_id TEXT NOT NULL,
  html_policy_version INTEGER NOT NULL
) STRICT;

-- Event: a key was replaced or withdrawn. Verification uses the newest key of a key_id without
-- a revocation.
CREATE TABLE account_key_revocations (
  account_key_id TEXT NOT NULL PRIMARY KEY,
  revoked_at INTEGER NOT NULL
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- content: remote statuses and media (data-model §4.4, docs/architecture/federation.md §9).
-- Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Event (subtype of status_revisions): the activity that carried a remote revision (NULL for a
-- revision made from a fetch) and the version of the HTML policy that sanitized its bodies.
CREATE TABLE remote_status_revisions (
  revision_id TEXT NOT NULL PRIMARY KEY,
  activity_uri TEXT,
  html_policy_version INTEGER NOT NULL
) STRICT;

-- Resource (subtype of media_files): a remote file as the received document declared it. The
-- file itself is not fetched. owner_account_id copies media_files.owner_account_id.
CREATE TABLE remote_media (
  media_id TEXT NOT NULL PRIMARY KEY,
  owner_account_id TEXT NOT NULL,
  remote_url TEXT NOT NULL,
  declared_content_type TEXT,
  declared_media_type TEXT NOT NULL CHECK (declared_media_type IN ('image', 'gifv', 'video', 'audio', 'unknown')),
  width INTEGER,
  height INTEGER,
  blurhash TEXT,
  focus_x REAL CHECK (focus_x BETWEEN -1.0 AND 1.0),
  focus_y REAL CHECK (focus_y BETWEEN -1.0 AND 1.0),
  description TEXT
) STRICT;
CREATE UNIQUE INDEX remote_media_owner_account_id_remote_url ON remote_media (owner_account_id, remote_url);

-- Current value (cache): a remote file copied into blob storage under remote/ (FR-409,
-- NFR-COST-4). The cleanup removes rows past expires_at and the least recently accessed ones.
CREATE TABLE remote_media_cache (
  url TEXT NOT NULL PRIMARY KEY,
  blob_key TEXT NOT NULL,
  content_type TEXT NOT NULL,
  file_size INTEGER NOT NULL,
  fetched_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  last_accessed_at INTEGER NOT NULL
) STRICT;
CREATE INDEX remote_media_cache_expires_at ON remote_media_cache (expires_at);
CREATE INDEX remote_media_cache_last_accessed_at ON remote_media_cache (last_accessed_at);

-- ---------------------------------------------------------------------------------------------
-- federation: inbox, delivery, and remote instances (data-model §4.8,
-- docs/architecture/federation.md). Owned by internal/app/federation.
-- ---------------------------------------------------------------------------------------------

-- Event: an activity accepted at an inbox, once per activity_uri (synthesized from the body's
-- digest when the activity has no id). trust is refetch when the signer is not the actor.
CREATE TABLE received_activities (
  activity_uri TEXT NOT NULL PRIMARY KEY,
  actor_uri TEXT NOT NULL,
  type TEXT NOT NULL,
  signature_scheme TEXT NOT NULL CHECK (signature_scheme IN ('rfc9421', 'cavage')),
  trust TEXT NOT NULL CHECK (trust IN ('signed', 'refetch')),
  received_at INTEGER NOT NULL
) STRICT;
CREATE INDEX received_activities_received_at ON received_activities (received_at);

-- Event: the received document with the bytes it arrived with, read by the asynchronous
-- processing and kept briefly for investigation and fixtures.
CREATE TABLE received_activity_documents (
  activity_uri TEXT NOT NULL PRIMARY KEY,
  document BLOB NOT NULL
) STRICT;

-- Event: a Delete of an unknown object or an Undo of an unknown target. A Create, Update, or
-- Follow of the uri that arrives later is dropped.
CREATE TABLE remote_object_tombstones (
  uri TEXT NOT NULL PRIMARY KEY,
  actor_uri TEXT NOT NULL,
  activity_uri TEXT NOT NULL,
  received_at INTEGER NOT NULL
) STRICT;
CREATE INDEX remote_object_tombstones_received_at ON remote_object_tombstones (received_at);

-- Event: an activity a local actor sends about one of its objects.
CREATE TABLE outbound_activities (
  id TEXT NOT NULL PRIMARY KEY,
  activity_uri TEXT NOT NULL,
  type TEXT NOT NULL,
  actor_account_id TEXT NOT NULL,
  object_type TEXT NOT NULL CHECK (object_type IN ('status', 'boost', 'favourite', 'follow', 'block', 'account', 'report')),
  object_id TEXT NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX outbound_activities_activity_uri ON outbound_activities (activity_uri);
CREATE INDEX outbound_activities_object_type_object_id_id ON outbound_activities (object_type, object_id, id);

-- Event: the body of an outbound activity, built once by the fan-out so that every delivery
-- and retry sends the same bytes.
CREATE TABLE outbound_activity_documents (
  outbound_activity_id TEXT NOT NULL PRIMARY KEY,
  document BLOB NOT NULL,
  built_at INTEGER NOT NULL
) STRICT;

-- Event: an inbox (shared inboxes merged) an outbound activity is delivered to. It is also the
-- record of where a status went, for its later Update and Delete.
CREATE TABLE deliveries (
  id TEXT NOT NULL PRIMARY KEY,
  outbound_activity_id TEXT NOT NULL,
  inbox_url TEXT NOT NULL,
  domain TEXT NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX deliveries_outbound_activity_id_inbox_url ON deliveries (outbound_activity_id, inbox_url);

-- Event: one attempt of a delivery. signature_scheme is NULL when no response came back.
CREATE TABLE delivery_attempts (
  id TEXT NOT NULL PRIMARY KEY,
  delivery_id TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('succeeded', 'retryable_failure', 'permanent_failure', 'skipped')),
  signature_scheme TEXT CHECK (signature_scheme IN ('rfc9421', 'cavage')),
  http_status INTEGER,
  error TEXT,
  attempted_at INTEGER NOT NULL
) STRICT;
CREATE INDEX delivery_attempts_delivery_id_id ON delivery_attempts (delivery_id, id);

-- Resource: a server this instance has seen.
CREATE TABLE remote_instances (
  domain TEXT NOT NULL PRIMARY KEY,
  first_seen_at INTEGER NOT NULL
) STRICT;

-- Event: the software a server reported in its NodeInfo.
CREATE TABLE instance_nodeinfo_observations (
  id TEXT NOT NULL PRIMARY KEY,
  domain TEXT NOT NULL,
  software_name TEXT NOT NULL,
  software_version TEXT NOT NULL,
  observed_at INTEGER NOT NULL
) STRICT;
CREATE INDEX instance_nodeinfo_observations_domain_id ON instance_nodeinfo_observations (domain, id DESC);

-- Event: a server was marked unavailable or available as a delivery target, with the reason.
CREATE TABLE instance_availability_events (
  id TEXT NOT NULL PRIMARY KEY,
  domain TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('marked_unavailable', 'marked_available')),
  reason TEXT NOT NULL CHECK (reason IN (
    'failure_days', 'delivery_succeeded', 'inbound_request', 'probe_succeeded', 'operator'
  )),
  occurred_at INTEGER NOT NULL
) STRICT;
CREATE INDEX instance_availability_events_domain_id ON instance_availability_events (domain, id DESC);

-- Projection: whether a server is a delivery target, from delivery_attempts and
-- instance_availability_events. failure_days counts the distinct UTC days of the latest run of
-- failures; last_failure_day is YYYYMMDD, or '' without a failure.
CREATE TABLE instance_health (
  domain TEXT NOT NULL PRIMARY KEY,
  state TEXT NOT NULL CHECK (state IN ('available', 'unavailable')),
  since INTEGER NOT NULL,
  failure_days INTEGER NOT NULL CHECK (failure_days >= 0),
  last_failure_day TEXT NOT NULL CHECK (length(last_failure_day) IN (0, 8))
) STRICT;
CREATE INDEX instance_health_state ON instance_health (state);

-- ---------------------------------------------------------------------------------------------
-- content: custom emoji and polls of statuses (data-model §4.4). Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Resource: a custom emoji, unique by its shortcode on a domain ('' for this instance). uri is
-- the id of a remote emoji's Emoji object.
CREATE TABLE custom_emojis (
  id TEXT NOT NULL PRIMARY KEY,
  shortcode TEXT NOT NULL,
  domain TEXT NOT NULL,
  uri TEXT,
  created_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX custom_emojis_shortcode_domain ON custom_emojis (shortcode, domain);
CREATE UNIQUE INDEX custom_emojis_uri ON custom_emojis (uri);

-- Event (revision): the image of a custom emoji. The newest row is the current image.
CREATE TABLE custom_emoji_images (
  id TEXT NOT NULL PRIMARY KEY,
  emoji_id TEXT NOT NULL,
  media_id TEXT NOT NULL,
  revised_at INTEGER NOT NULL
) STRICT;
CREATE INDEX custom_emoji_images_emoji_id_id ON custom_emoji_images (emoji_id, id DESC);
-- The remote media cache looks up whether a media file is a custom emoji image (avatars policy).
CREATE INDEX custom_emoji_images_media_id ON custom_emoji_images (media_id);

-- Event (child of a revision): the poll of a status revision (FR-112). closes_at is NULL for a
-- poll without an end.
CREATE TABLE polls (
  id TEXT NOT NULL PRIMARY KEY,
  revision_id TEXT NOT NULL,
  multiple INTEGER NOT NULL CHECK (multiple IN (0, 1)),
  closes_at INTEGER
) STRICT;
CREATE UNIQUE INDEX polls_revision_id ON polls (revision_id);

-- Event (child of a poll): one choice of a poll.
CREATE TABLE poll_options (
  poll_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position >= 0),
  title TEXT NOT NULL,
  PRIMARY KEY (poll_id, position)
) STRICT;

-- Event: the counts of a remote poll as its server reported them (in a Create or an Update).
CREATE TABLE poll_tally_observations (
  id TEXT NOT NULL PRIMARY KEY,
  poll_id TEXT NOT NULL,
  voters_count INTEGER CHECK (voters_count >= 0),
  observed_at INTEGER NOT NULL
) STRICT;
CREATE INDEX poll_tally_observations_poll_id_id ON poll_tally_observations (poll_id, id DESC);

-- Event (child of an observation): the votes of one choice.
CREATE TABLE poll_tally_observation_counts (
  observation_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position >= 0),
  votes INTEGER NOT NULL CHECK (votes >= 0),
  PRIMARY KEY (observation_id, position)
) STRICT;

-- Projection: the votes of each choice of a poll, from its newest observation (remote polls).
-- voters_count is NULL when the server did not report it.
CREATE TABLE poll_tallies (
  poll_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position >= 0),
  votes INTEGER NOT NULL CHECK (votes >= 0),
  voters_count INTEGER CHECK (voters_count >= 0),
  PRIMARY KEY (poll_id, position)
) STRICT;

-- graph and federation: re-sent Follows and early Undo Follows (data-model §4.5, §4.8) ----------

-- Event: a Follow received again, under another URI, for a follow that is already active. The
-- follow keeps its row; its Accept is sent again for the new URI.
CREATE TABLE follow_resends (
  uri TEXT NOT NULL PRIMARY KEY,
  follow_id TEXT NOT NULL,
  received_at INTEGER NOT NULL
) STRICT;

-- Event: an Undo of an unknown Follow, by the pair of its actor and the object that the embedded
-- Follow names. The first Follow of the pair that arrives later is dropped, and its URI recorded
-- in remote_object_tombstones with this activity_uri.
CREATE TABLE remote_follow_tombstones (
  activity_uri TEXT NOT NULL PRIMARY KEY,
  actor_uri TEXT NOT NULL,
  object_uri TEXT NOT NULL,
  received_at INTEGER NOT NULL
) STRICT;
CREATE INDEX remote_follow_tombstones_actor_uri_object_uri ON remote_follow_tombstones (actor_uri, object_uri, received_at);
CREATE INDEX remote_follow_tombstones_received_at ON remote_follow_tombstones (received_at);
CREATE INDEX remote_object_tombstones_activity_uri ON remote_object_tombstones (activity_uri);

-- ---------------------------------------------------------------------------------------------
-- moderation: actions on accounts and domains, the restrictions they impose, and reports
-- (data-model §4.3, §5.2, docs/architecture/federation.md §11). Owned by internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Event: a moderation action on an account. A ban never expires.
CREATE TABLE moderation_actions (
  id TEXT NOT NULL PRIMARY KEY,
  target_account_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN (
    'warn', 'mark_statuses_sensitive', 'delete_statuses', 'force_sensitive', 'limit',
    'restrict_posting', 'restrict_interaction', 'disable_login', 'suspend', 'ban'
  )),
  reason_category TEXT NOT NULL CHECK (reason_category IN ('spam', 'legal', 'violation', 'other')),
  public_reason TEXT,
  private_note TEXT,
  report_id TEXT,
  issuer TEXT NOT NULL CHECK (issuer IN ('moderator', 'system', 'remote')),
  issued_by_user_id TEXT,
  notify_target INTEGER NOT NULL CHECK (notify_target IN (0, 1)),
  expires_at INTEGER,
  issued_at INTEGER NOT NULL,
  CHECK (kind <> 'ban' OR expires_at IS NULL),
  CHECK (expires_at IS NULL OR expires_at > issued_at)
) STRICT;
CREATE INDEX moderation_actions_target_account_id_id ON moderation_actions (target_account_id, id DESC);
CREATE INDEX moderation_actions_report_id ON moderation_actions (report_id);

-- Event: a moderation action on an account was lifted. Expiry is not a lift.
-- lifted_by_user_id is NULL for the system.
CREATE TABLE moderation_action_lifts (
  action_id TEXT NOT NULL PRIMARY KEY,
  lifted_by_user_id TEXT,
  reason TEXT,
  lifted_at INTEGER NOT NULL
) STRICT;

-- Projection: the restrictions that the actions on an account impose, one row per restriction
-- and action, until effective_until (NULL: until lifted).
CREATE TABLE account_restrictions (
  account_id TEXT NOT NULL,
  restriction TEXT NOT NULL CHECK (restriction IN (
    'login', 'post', 'interact', 'visibility_ceiling_private', 'public_discovery',
    'unsolicited_reach', 'media_sensitive', 'profile_view', 'federation', 'media_fetch',
    'report_intake'
  )),
  source_action_id TEXT NOT NULL,
  effective_until INTEGER,
  PRIMARY KEY (account_id, restriction, source_action_id)
) STRICT;
CREATE INDEX account_restrictions_source_action_id ON account_restrictions (source_action_id);
-- The removal of expired rows, which tells the end of expired suspensions (federation.md §11.1).
CREATE INDEX account_restrictions_effective_until ON account_restrictions (effective_until);

-- Event: a moderation action on a domain and its subdomains. issued_by_user_id is NULL for the
-- CLI.
CREATE TABLE domain_moderation_actions (
  id TEXT NOT NULL PRIMARY KEY,
  domain TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('limit', 'suspend', 'reject_media', 'reject_reports', 'force_sensitive')),
  public_comment TEXT,
  private_note TEXT,
  obfuscate INTEGER NOT NULL CHECK (obfuscate IN (0, 1)),
  issued_by_user_id TEXT,
  expires_at INTEGER,
  issued_at INTEGER NOT NULL,
  CHECK (expires_at IS NULL OR expires_at > issued_at)
) STRICT;
CREATE INDEX domain_moderation_actions_domain_id ON domain_moderation_actions (domain, id DESC);

-- Event: a domain action was lifted. lifted_by_user_id is NULL for the CLI.
CREATE TABLE domain_moderation_action_lifts (
  action_id TEXT NOT NULL PRIMARY KEY,
  lifted_by_user_id TEXT,
  lifted_at INTEGER NOT NULL
) STRICT;

-- Projection: the restrictions that the actions on a domain impose on it and its subdomains,
-- until effective_until (NULL: until lifted).
CREATE TABLE domain_restrictions (
  domain TEXT NOT NULL,
  restriction TEXT NOT NULL CHECK (restriction IN (
    'public_discovery', 'unsolicited_reach', 'profile_view', 'federation', 'media_fetch',
    'report_intake', 'media_sensitive'
  )),
  source_action_id TEXT NOT NULL,
  effective_until INTEGER,
  PRIMARY KEY (domain, restriction, source_action_id)
) STRICT;
CREATE INDEX domain_restrictions_source_action_id ON domain_restrictions (source_action_id);

-- Resource: a report of an account. uri is the id of a received Flag (or the URI synthesized
-- for a Flag without one), NULL for a local report.
CREATE TABLE reports (
  id TEXT NOT NULL PRIMARY KEY,
  uri TEXT,
  reporter_account_id TEXT NOT NULL,
  target_account_id TEXT NOT NULL,
  category TEXT NOT NULL CHECK (category IN ('spam', 'legal', 'violation', 'other')),
  comment TEXT NOT NULL,
  forwarded INTEGER NOT NULL CHECK (forwarded IN (0, 1)),
  filed_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX reports_uri ON reports (uri);
CREATE INDEX reports_target_account_id_id ON reports (target_account_id, id);
-- A reporter lists their own reports.
CREATE INDEX reports_reporter_account_id_id ON reports (reporter_account_id, id);

-- Resource (child of a report): the statuses a report points at.
CREATE TABLE report_statuses (
  report_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  PRIMARY KEY (report_id, status_id)
) STRICT;

-- Resource (child of a report): the instance rules a local report of the violation category
-- cites. A rule keeps its ID across revisions (instance_rules).
CREATE TABLE report_rule_citations (
  report_id TEXT NOT NULL,
  rule_id TEXT NOT NULL,
  PRIMARY KEY (report_id, rule_id)
) STRICT;

-- Projection: whether a report is open, how it was resolved, and who handles it. outcome is
-- that of the latest resolution, set exactly when the report is resolved.
CREATE TABLE report_states (
  report_id TEXT NOT NULL PRIMARY KEY,
  state TEXT NOT NULL CHECK (state IN ('open', 'resolved')),
  outcome TEXT CHECK (outcome IN ('actioned', 'dismissed')),
  assignee_user_id TEXT,
  since INTEGER NOT NULL,
  CHECK ((state = 'resolved') = (outcome IS NOT NULL))
) STRICT;
CREATE INDEX report_states_state_report_id ON report_states (state, report_id);

-- Event: a moderator took a report, or gave it up (assignee_user_id NULL).
-- assigned_by_user_id is NULL for the CLI.
CREATE TABLE report_assignments (
  id TEXT NOT NULL PRIMARY KEY,
  report_id TEXT NOT NULL,
  assignee_user_id TEXT,
  assigned_by_user_id TEXT,
  assigned_at INTEGER NOT NULL
) STRICT;
CREATE INDEX report_assignments_report_id_id ON report_assignments (report_id, id DESC);

-- Event: a moderator resolved a report. resolved_by_user_id is NULL for the CLI.
CREATE TABLE report_resolutions (
  id TEXT NOT NULL PRIMARY KEY,
  report_id TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('actioned', 'dismissed')),
  resolved_by_user_id TEXT,
  resolved_at INTEGER NOT NULL
) STRICT;
CREATE INDEX report_resolutions_report_id_id ON report_resolutions (report_id, id DESC);

-- moderation and content: actions on statuses (#364, data-model §4.3, §4.4) -------------------

-- Event child: a status that an action on statuses (delete_statuses, mark_statuses_sensitive)
-- acts on. The status is the target account's.
CREATE TABLE moderation_action_statuses (
  action_id TEXT NOT NULL,
  status_id TEXT NOT NULL,
  PRIMARY KEY (action_id, status_id)
) STRICT;

-- Projection (content): a status that a mark_statuses_sensitive action shows as sensitive, until
-- the action is lifted.
CREATE TABLE status_sensitivity_overrides (
  status_id TEXT NOT NULL,
  source_action_id TEXT NOT NULL,
  PRIMARY KEY (status_id, source_action_id)
) STRICT;
CREATE INDEX status_sensitivity_overrides_source_action_id ON status_sensitivity_overrides (source_action_id);

-- moderation: notes on reports and their reopenings (#333, data-model §4.3) --------------------

-- Event: a moderator reopened a resolved report. reopened_by_user_id is NULL for the CLI.
CREATE TABLE report_reopenings (
  id TEXT NOT NULL PRIMARY KEY,
  report_id TEXT NOT NULL,
  reopened_by_user_id TEXT,
  reopened_at INTEGER NOT NULL
) STRICT;
CREATE INDEX report_reopenings_report_id_id ON report_reopenings (report_id, id DESC);

-- Event: a moderator's internal note on a report, shown to the moderators only.
-- author_user_id is NULL for the CLI.
CREATE TABLE report_notes (
  id TEXT NOT NULL PRIMARY KEY,
  report_id TEXT NOT NULL,
  author_user_id TEXT,
  body TEXT NOT NULL,
  written_at INTEGER NOT NULL
) STRICT;
CREATE INDEX report_notes_report_id_id ON report_notes (report_id, id);

-- ---------------------------------------------------------------------------------------------
-- identity: where sign-ins and registrations came from (data-model.md §4.1, ADR-0038). Kept
-- apart from sessions and users so that the purge erases them after their retention.
-- ---------------------------------------------------------------------------------------------

-- Resource (child of a session): the client that started a session. ip and user_agent are NULL
-- when unknown; a session with neither has no row.
CREATE TABLE session_origins (
  session_id TEXT NOT NULL PRIMARY KEY,
  user_id TEXT NOT NULL,
  ip TEXT,
  user_agent TEXT,
  created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX session_origins_ip ON session_origins (ip);
CREATE INDEX session_origins_created_at ON session_origins (created_at);
CREATE INDEX session_origins_user_id_created_at ON session_origins (user_id, created_at DESC);

-- Resource: the IP address of a registration from the registration page. A registration from
-- an unknown address has no row.
CREATE TABLE registration_origins (
  user_id TEXT NOT NULL PRIMARY KEY,
  ip TEXT NOT NULL,
  recorded_at INTEGER NOT NULL
) STRICT;
CREATE INDEX registration_origins_ip ON registration_origins (ip);
CREATE INDEX registration_origins_recorded_at ON registration_origins (recorded_at);

-- moderation: the audit log (data-model §4.3, ADR-0037). Owned by internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Event: one administrative operation, appended in the transaction of the operation by the
-- moderation module's projector. Never updated; only the retention cleanup deletes rows
-- (ADR-0038). target_label copies the target's display name, so that the entry stays readable
-- after the target is erased. source_event_id is the row of the source event, which holds the
-- reasons and notes. actor_user_id is set exactly for a user actor (the CLI acts as no user).
CREATE TABLE audit_log_entries (
  id TEXT NOT NULL PRIMARY KEY,
  actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'cli', 'system', 'remote')),
  actor_user_id TEXT,
  action TEXT NOT NULL CHECK (action IN (
    'account_action_issued', 'account_action_lifted', 'domain_action_issued',
    'domain_action_lifted', 'report_assigned', 'report_unassigned', 'report_resolved',
    'report_reopened', 'report_note_added',
    'registration_approved', 'registration_rejected', 'setting_changed', 'text_changed',
    'rule_added', 'rule_revised', 'rule_moved', 'rule_retired', 'role_granted', 'role_revoked', 'appeal_upheld',
    'appeal_overturned', 'account_deletion_cancelled',
    'revealed_private_data', 'signup_block_added', 'signup_block_lifted'
  )),
  target_kind TEXT NOT NULL CHECK (target_kind IN (
    'account', 'status', 'domain', 'report', 'appeal', 'user', 'role', 'setting', 'rule',
    'signup_block'
  )),
  target_id TEXT NOT NULL,
  target_label TEXT NOT NULL,
  source_event_id TEXT,
  occurred_at INTEGER NOT NULL,
  CHECK ((actor_kind = 'user') = (actor_user_id IS NOT NULL))
) STRICT;
CREATE INDEX audit_log_entries_target_kind_target_id_id ON audit_log_entries (target_kind, target_id, id DESC);
CREATE INDEX audit_log_entries_actor_user_id_id ON audit_log_entries (actor_user_id, id DESC);

-- ---------------------------------------------------------------------------------------------
-- moderation: the follows that suspensions cut (data-model §4.3, FR-717, ADR-0036). Owned by
-- internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Event: the active follows that one local account lost when a domain (domain_action_id) or a
-- remote account (action_id) was suspended; exactly one of the two is set. One row per account
-- and action, so that processing the same action again records and notifies nothing more.
CREATE TABLE relationship_severances (
  id TEXT NOT NULL PRIMARY KEY,
  domain_action_id TEXT,
  action_id TEXT,
  account_id TEXT NOT NULL,
  followers_lost INTEGER NOT NULL CHECK (followers_lost >= 0),
  following_lost INTEGER NOT NULL CHECK (following_lost >= 0),
  occurred_at INTEGER NOT NULL,
  CHECK ((domain_action_id IS NULL) <> (action_id IS NULL))
) STRICT;
CREATE INDEX relationship_severances_account_id_id ON relationship_severances (account_id, id DESC);
CREATE UNIQUE INDEX relationship_severances_domain_action_id_account_id ON relationship_severances (domain_action_id, account_id);
CREATE UNIQUE INDEX relationship_severances_action_id_account_id ON relationship_severances (action_id, account_id);

-- ---------------------------------------------------------------------------------------------
-- moderation: appeals of actions on local accounts (#337, data-model §4.3, FR-707, ADR-0038).
-- Owned by internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Resource: a local account's appeal of an action on it, at most one per action, within the
-- appeal window (instance setting appeal_window) from the action.
CREATE TABLE moderation_appeals (
  id TEXT NOT NULL PRIMARY KEY,
  action_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  body TEXT NOT NULL,
  submitted_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX moderation_appeals_action_id ON moderation_appeals (action_id);

-- Event: a moderator's decision on an appeal, at most one. An overturned appeal records the
-- lift of its action in the same transaction. decided_by_user_id is NULL for the CLI.
CREATE TABLE moderation_appeal_decisions (
  appeal_id TEXT NOT NULL PRIMARY KEY,
  decision TEXT NOT NULL CHECK (decision IN ('upheld', 'overturned')),
  public_reason TEXT,
  decided_by_user_id TEXT,
  decided_at INTEGER NOT NULL
) STRICT;

-- Projection: where each appeal stands, for the queue of pending appeals.
CREATE TABLE appeal_states (
  appeal_id TEXT NOT NULL PRIMARY KEY,
  state TEXT NOT NULL CHECK (state IN ('pending', 'upheld', 'overturned')),
  since INTEGER NOT NULL
) STRICT;
CREATE INDEX appeal_states_state_appeal_id ON appeal_states (state, appeal_id);

-- moderation: the sign-up block list (data-model §4.3, FR-719). Owned by internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Event: a block on sign-ups from the registration page. value is an IDNA A-label domain
-- (email_domain, which covers its subdomains), the hex SHA-256 of a canonical email address
-- (email_hash), or a masked CIDR prefix (ip_range). moderation_action_id is the ban the block
-- was made from; created_by_user_id is NULL for the CLI. A block is in effect until it expires
-- or is lifted.
CREATE TABLE signup_blocks (
  id TEXT NOT NULL PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('email_domain', 'email_hash', 'ip_range')),
  value TEXT NOT NULL,
  severity TEXT NOT NULL CHECK (severity IN ('reject', 'require_approval')),
  comment TEXT,
  moderation_action_id TEXT,
  created_by_user_id TEXT,
  expires_at INTEGER,
  created_at INTEGER NOT NULL,
  CHECK (expires_at IS NULL OR expires_at > created_at)
) STRICT;
CREATE INDEX signup_blocks_kind_value ON signup_blocks (kind, value);

-- Event: a sign-up block was lifted. Expiry is not a lift. lifted_by_user_id is NULL for the
-- CLI.
CREATE TABLE signup_block_lifts (
  block_id TEXT NOT NULL PRIMARY KEY,
  lifted_by_user_id TEXT,
  lifted_at INTEGER NOT NULL
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- moderation: retention of the moderation records and the holds that stop it (#349,
-- data-model §4.3, §8, ADR-0038). Owned by internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Event: an operator's hold on the erasure of an account's records (a court order to preserve
-- them, a disclosure request in progress). created_by_user_id is NULL for the CLI.
CREATE TABLE retention_holds (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_by_user_id TEXT,
  created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX retention_holds_account_id ON retention_holds (account_id);

-- Event: a hold was released, at most once. released_by_user_id is NULL for the CLI.
CREATE TABLE retention_hold_releases (
  hold_id TEXT NOT NULL PRIMARY KEY,
  released_by_user_id TEXT,
  released_at INTEGER NOT NULL
) STRICT;

-- The purge of remote statuses keeps the statuses that a report or an action points at.
CREATE INDEX report_statuses_status_id ON report_statuses (status_id);
CREATE INDEX moderation_action_statuses_status_id ON moderation_action_statuses (status_id);

-- ---------------------------------------------------------------------------------------------
-- account: the erasure of deleted local accounts' data (#397, data-model §4.2, §8). Owned by
-- internal/app/account; the indexes serve the erasure's statements in the other modules.
-- ---------------------------------------------------------------------------------------------

-- Event: the erasure job removed the data of a deleted local account (data-model §8). An
-- account is erased at most once; one whose statuses the moderation records still keep has no
-- row until they are erased too.
CREATE TABLE account_erasures (
  account_id TEXT NOT NULL PRIMARY KEY,
  erased_at INTEGER NOT NULL
) STRICT;

CREATE INDEX follows_follower_account_id ON follows (follower_account_id);
CREATE INDEX follows_followee_account_id ON follows (followee_account_id);
CREATE INDEX blocks_blocker_account_id ON blocks (blocker_account_id);
CREATE INDEX blocks_blocked_account_id ON blocks (blocked_account_id);
CREATE INDEX boosts_booster_account_id ON boosts (booster_account_id);
CREATE INDEX mutes_target_account_id ON mutes (target_account_id);
CREATE INDEX notifications_source_account_id ON notifications (source_account_id);
CREATE INDEX notifications_subject_user_id ON notifications (subject_user_id);
CREATE INDEX feed_entries_original_author_account_id ON feed_entries (original_author_account_id);
CREATE INDEX feed_follow_edges_followee_account_id ON feed_follow_edges (followee_account_id);
CREATE INDEX feed_exclusions_target_account_id ON feed_exclusions (target_account_id);
CREATE INDEX notification_exclusions_target_account_id ON notification_exclusions (target_account_id);
