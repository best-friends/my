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
  origin TEXT NOT NULL CHECK (origin IN ('author', 'moderation', 'remote', 'account_deletion')),
  moderation_action_id TEXT,
  deleted_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX status_deletions_uri ON status_deletions (uri);

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
-- applies; readers filter it out.
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
    'preview_image'
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
  user_agent TEXT,
  ip TEXT,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX sessions_token_hash ON sessions (token_hash);

-- Event: a session ended before it expired.
CREATE TABLE session_revocations (
  session_id TEXT NOT NULL PRIMARY KEY,
  reason TEXT NOT NULL CHECK (reason IN ('logout', 'replaced', 'password_changed', 'moderation', 'expired_cleanup')),
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

-- Resource: an authorization code, keyed by its SHA-256 digest.
CREATE TABLE oauth_authorization_codes (
  code_hash BLOB NOT NULL PRIMARY KEY,
  client_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  scope TEXT NOT NULL,
  code_challenge TEXT NOT NULL,
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

-- Event: a token row (its access and refresh token) stopped working.
CREATE TABLE oauth_token_revocations (
  token_id TEXT NOT NULL PRIMARY KEY,
  reason TEXT NOT NULL CHECK (reason IN ('user', 'rotation', 'refresh_reuse', 'code_reuse', 'password_changed', 'moderation')),
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
  revision_seq INTEGER NOT NULL CHECK (revision_seq >= 1)
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
    'follow', 'follow_request', 'follow_accepted', 'mention', 'boost', 'favourite', 'poll_closed', 'moderation_notice'
  )),
  source_account_id TEXT,
  status_id TEXT,
  moderation_action_id TEXT,
  dedupe_key TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  -- nobody is notified of their own actions.
  CHECK (source_account_id IS NULL OR source_account_id <> recipient_account_id)
) STRICT;
CREATE UNIQUE INDEX notifications_recipient_account_id_dedupe_key ON notifications (recipient_account_id, dedupe_key);
CREATE INDEX notifications_recipient_account_id_id ON notifications (recipient_account_id, id);
CREATE INDEX notifications_recipient_account_id_type_id ON notifications (recipient_account_id, type, id);

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
