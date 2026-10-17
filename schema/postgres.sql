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

-- Resource: an email address a user added.
CREATE TABLE user_email_addresses (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  address text NOT NULL,
  address_normalized text NOT NULL,
  added_at timestamptz NOT NULL
);
CREATE INDEX user_email_addresses_user_id_id ON user_email_addresses (user_id, id);

-- Projection: the primary email address of each user, for login and against duplicates.
CREATE TABLE user_primary_emails (
  user_id text COLLATE "C" NOT NULL PRIMARY KEY,
  email_address_id text COLLATE "C" NOT NULL,
  address_normalized text NOT NULL
);
CREATE UNIQUE INDEX user_primary_emails_address_normalized ON user_primary_emails (address_normalized);

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
    'manage_settings', 'manage_roles', 'manage_invites', 'view_audit', 'manage_jobs'
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
  fingerprint bytea NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX account_keys_key_id_fingerprint ON account_keys (key_id, fingerprint);
CREATE INDEX account_keys_key_id_id ON account_keys (key_id, id DESC);
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

-- content: statuses (data-model §4.4) --------------------------------------------------------

-- Resource: a status. Visibility, author, reply target, and publication time never change.
CREATE TABLE statuses (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text NOT NULL,
  url text,
  author_account_id text COLLATE "C" NOT NULL,
  is_local boolean NOT NULL,
  visibility text NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  in_reply_to_uri text,
  conversation_uri text,
  published_at timestamptz NOT NULL,
  -- local statuses are never federated, so only this instance has them (ADR-0017).
  CONSTRAINT statuses_local_check CHECK (visibility <> 'local' OR is_local)
);
CREATE UNIQUE INDEX statuses_uri ON statuses (uri);
CREATE INDEX statuses_author_account_id_id ON statuses (author_account_id, id);
CREATE INDEX statuses_in_reply_to_uri ON statuses (in_reply_to_uri);
CREATE INDEX statuses_conversation_uri_id ON statuses (conversation_uri, id);

-- Resource (subtype): a status posted on this instance, with the client API's Idempotency-Key.
CREATE TABLE local_statuses (
  status_id text COLLATE "C" NOT NULL PRIMARY KEY,
  author_account_id text COLLATE "C" NOT NULL,
  client_id text COLLATE "C",
  idempotency_key text
);
CREATE UNIQUE INDEX local_statuses_author_account_id_idempotency_key ON local_statuses (author_account_id, idempotency_key);

-- Event (revision): one version of a status's content (FR-109).
CREATE TABLE status_revisions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  status_id text COLLATE "C" NOT NULL,
  seq integer NOT NULL CHECK (seq >= 1),
  spoiler_text text NOT NULL,
  sensitive boolean NOT NULL,
  language text NOT NULL,
  revised_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX status_revisions_status_id_seq ON status_revisions (status_id, seq);

-- Event (child of a revision): the content per language, normalized from contentMap (NFR-I18N-5).
CREATE TABLE status_revision_bodies (
  revision_id text COLLATE "C" NOT NULL,
  language text NOT NULL,
  content_html text NOT NULL,
  PRIMARY KEY (revision_id, language)
);

-- Event (subtype of a revision): the source text a local author typed.
CREATE TABLE status_revision_sources (
  revision_id text COLLATE "C" NOT NULL PRIMARY KEY,
  source_text text NOT NULL,
  source_format text NOT NULL CHECK (source_format IN ('text_plain'))
);

-- Event (child of a revision): the attached media, with per-revision alt text (FR-105).
CREATE TABLE status_revision_attachments (
  revision_id text COLLATE "C" NOT NULL,
  position integer NOT NULL CHECK (position BETWEEN 0 AND 3),
  media_id text COLLATE "C" NOT NULL,
  description text,
  PRIMARY KEY (revision_id, position)
);

-- Event (child of a revision): the mentioned accounts.
CREATE TABLE status_revision_mentions (
  revision_id text COLLATE "C" NOT NULL,
  account_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (revision_id, account_id)
);

-- Event (child of a revision): the hashtags.
CREATE TABLE status_revision_tags (
  revision_id text COLLATE "C" NOT NULL,
  tag_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (revision_id, tag_id)
);

-- Event (child of a revision): the custom emoji.
CREATE TABLE status_revision_emojis (
  revision_id text COLLATE "C" NOT NULL,
  emoji_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (revision_id, emoji_id)
);

-- Event: the deletion of a status. It outlives the purge as the tombstone of the URI.
CREATE TABLE status_deletions (
  status_id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text NOT NULL,
  author_account_id text COLLATE "C" NOT NULL,
  origin text NOT NULL CHECK (origin IN ('author', 'moderation', 'remote', 'account_deletion', 'retention')),
  moderation_action_id text COLLATE "C",
  deleted_at timestamptz NOT NULL
);
CREATE INDEX status_deletions_uri ON status_deletions (uri);
-- A deletion other than by retention keeps its URI from coming back, so there is at most one.
-- A remote status deleted by retention may be fetched again and deleted again (ADR-0034).
CREATE UNIQUE INDEX status_deletions_tombstone_uri ON status_deletions (uri) WHERE origin <> 'retention';

-- Resource: a hashtag, unique by its normalized name.
CREATE TABLE tags (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  name text NOT NULL,
  name_normalized text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX tags_name_normalized ON tags (name_normalized);

-- Projection: the current revision of each status that is not deleted.
CREATE TABLE status_heads (
  status_id text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id text COLLATE "C" NOT NULL,
  seq integer NOT NULL,
  last_revised_at timestamptz NOT NULL
);

-- Projection: the known parent of each reply, neither of them deleted (FR-106).
CREATE TABLE status_reply_links (
  status_id text COLLATE "C" NOT NULL PRIMARY KEY,
  parent_status_id text COLLATE "C" NOT NULL,
  parent_author_account_id text COLLATE "C" NOT NULL
);
CREATE INDEX status_reply_links_parent_status_id_status_id ON status_reply_links (parent_status_id, status_id);

-- Projection: the accounts that the current revision of each status mentions.
CREATE TABLE status_mention_index (
  account_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (account_id, status_id)
);
CREATE INDEX status_mention_index_status_id ON status_mention_index (status_id);

-- Projection: the counters of each status that is not deleted (data-model §2.3).
CREATE TABLE status_counters (
  status_id text COLLATE "C" NOT NULL PRIMARY KEY,
  replies_count bigint NOT NULL CHECK (replies_count >= 0),
  boosts_count bigint NOT NULL CHECK (boosts_count >= 0),
  favourites_count bigint NOT NULL CHECK (favourites_count >= 0)
);

-- Projection: the statuses of each account that are not deleted. No row means none.
CREATE TABLE account_post_counters (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  statuses_count bigint NOT NULL CHECK (statuses_count > 0),
  last_status_at timestamptz NOT NULL
);

-- graph: follows (data-model §4.5) ------------------------------------------------------------

-- Resource: one follow relationship. Following again after a rejection or termination inserts a
-- new row, so earlier relationships stay on record.
CREATE TABLE follows (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text NOT NULL,
  follower_account_id text COLLATE "C" NOT NULL,
  followee_account_id text COLLATE "C" NOT NULL,
  requested_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX follows_uri ON follows (uri);

-- Event: a follow became active.
CREATE TABLE follow_acceptances (
  follow_id text COLLATE "C" NOT NULL PRIMARY KEY,
  accepted_by text NOT NULL CHECK (accepted_by IN ('auto', 'followee', 'remote')),
  activity_uri text,
  accepted_at timestamptz NOT NULL
);

-- Event: a pending follow was declined.
CREATE TABLE follow_rejections (
  follow_id text COLLATE "C" NOT NULL PRIMARY KEY,
  rejected_by text NOT NULL CHECK (rejected_by IN ('followee', 'remote', 'system')),
  activity_uri text,
  rejected_at timestamptz NOT NULL
);

-- Event: a pending or active follow ended.
CREATE TABLE follow_terminations (
  follow_id text COLLATE "C" NOT NULL PRIMARY KEY,
  reason text NOT NULL CHECK (reason IN ('unfollowed', 'removed_by_followee', 'blocked', 'account_deleted', 'moderation')),
  terminated_at timestamptz NOT NULL
);

-- Projection: the live follow of each pair (neither rejected nor terminated).
CREATE TABLE follow_edges (
  follower_account_id text COLLATE "C" NOT NULL,
  followee_account_id text COLLATE "C" NOT NULL,
  follow_id text COLLATE "C" NOT NULL,
  state text NOT NULL CHECK (state IN ('pending', 'active')),
  since timestamptz NOT NULL,
  PRIMARY KEY (follower_account_id, followee_account_id)
);
CREATE INDEX follow_edges_followee_state_follow_id ON follow_edges (followee_account_id, state, follow_id);
CREATE INDEX follow_edges_follower_state_follow_id ON follow_edges (follower_account_id, state, follow_id);

-- Current value: the follower's settings of a follow.
CREATE TABLE follow_preferences (
  follow_id text COLLATE "C" NOT NULL PRIMARY KEY,
  show_boosts boolean NOT NULL,
  notify boolean NOT NULL
);

-- Projection: the numbers of active follows into and out of an account. No row means zero.
CREATE TABLE follow_counters (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  followers_count bigint NOT NULL CHECK (followers_count >= 0),
  following_count bigint NOT NULL CHECK (following_count >= 0)
);

-- graph: blocks and mutes (data-model §4.5) -------------------------------------------------

-- Resource: one block. Blocking again after a retraction inserts a new row. uri is NULL only for
-- a received Block without an id.
CREATE TABLE blocks (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text,
  blocker_account_id text COLLATE "C" NOT NULL,
  blocked_account_id text COLLATE "C" NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX blocks_uri ON blocks (uri);

-- Event: a block was lifted.
CREATE TABLE block_retractions (
  block_id text COLLATE "C" NOT NULL PRIMARY KEY,
  retracted_at timestamptz NOT NULL
);

-- Projection: the block in force of each pair (without a retraction).
CREATE TABLE block_edges (
  blocker_account_id text COLLATE "C" NOT NULL,
  blocked_account_id text COLLATE "C" NOT NULL,
  block_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (blocker_account_id, blocked_account_id)
);
CREATE INDEX block_edges_blocked_account_id ON block_edges (blocked_account_id);
CREATE INDEX block_edges_blocker_block_id ON block_edges (blocker_account_id, block_id);

-- Current value: an account's mute of another. A mute whose expires_at has passed no longer
-- applies; readers filter it out, and the hourly cleanup deletes it.
CREATE TABLE mutes (
  account_id text COLLATE "C" NOT NULL,
  target_account_id text COLLATE "C" NOT NULL,
  hide_notifications boolean NOT NULL,
  expires_at timestamptz,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (account_id, target_account_id),
  CHECK (expires_at IS NULL OR expires_at > created_at)
);
CREATE INDEX mutes_account_created_at_target ON mutes (account_id, created_at, target_account_id);
-- The periodic cleanup deletes the expired mutes (FR-803).
CREATE INDEX mutes_expires_at ON mutes (expires_at);

-- ---------------------------------------------------------------------------------------------
-- 基盤: job queue and Transactional Outbox (data-model.md §4.10, ADR-0004).
-- Owned by internal/adapter/queue/db.
-- ---------------------------------------------------------------------------------------------

-- Jobs written in a domain transaction, waiting for the relay to enqueue them.
CREATE TABLE outbox_events (
  id text NOT NULL PRIMARY KEY,
  job_type text NOT NULL,
  payload bytea NOT NULL,
  idempotency_key text,
  ordering_key text,
  run_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL
);

-- Pending and running jobs of the database queue. A job is deleted when it finishes and moved
-- to dead_jobs when it fails for good. A claim sets locked_by and locked_until together.
CREATE TABLE jobs (
  id text NOT NULL PRIMARY KEY,
  type text NOT NULL,
  payload bytea NOT NULL,
  idempotency_key text,
  ordering_key text,
  attempt integer NOT NULL,
  run_at timestamptz NOT NULL,
  locked_by text,
  locked_until timestamptz,
  last_error text,
  created_at timestamptz NOT NULL,
  CONSTRAINT jobs_attempt_check CHECK (attempt >= 0),
  CONSTRAINT jobs_lock_check CHECK ((locked_by IS NULL) = (locked_until IS NULL))
);
CREATE UNIQUE INDEX jobs_idempotency_key ON jobs (idempotency_key);
CREATE INDEX jobs_run_at ON jobs (run_at);
CREATE INDEX jobs_ordering_key_run_at ON jobs (ordering_key, run_at);

-- Jobs that failed for good, kept for the admin screen and the CLI to requeue.
CREATE TABLE dead_jobs (
  id text NOT NULL PRIMARY KEY,
  type text NOT NULL,
  payload bytea NOT NULL,
  idempotency_key text,
  ordering_key text,
  attempt integer NOT NULL,
  last_error text NOT NULL,
  created_at timestamptz NOT NULL,
  failed_at timestamptz NOT NULL
);
CREATE INDEX dead_jobs_failed_at ON dead_jobs (failed_at);

-- ---------------------------------------------------------------------------------------------
-- identity: user preferences (data-model.md §4.1, FR-007, FR-208).
-- ---------------------------------------------------------------------------------------------

-- Current value: a user's settings. locale and default_post_language are canonical BCP 47
-- tags; time_zone is an IANA name.
CREATE TABLE user_preferences (
  user_id text COLLATE "C" NOT NULL PRIMARY KEY,
  locale text NOT NULL,
  time_zone text NOT NULL,
  default_visibility text NOT NULL CHECK (default_visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  default_post_language text NOT NULL,
  require_media_description boolean NOT NULL,
  updated_at timestamptz NOT NULL
);

-- Current value: the languages a user's timelines show; no rows show every language.
CREATE TABLE user_timeline_languages (
  user_id text COLLATE "C" NOT NULL,
  language text NOT NULL,
  PRIMARY KEY (user_id, language)
);

-- ---------------------------------------------------------------------------------------------
-- instance: settings and texts (data-model §4.9). Owned by internal/app/instance.
-- Keys are defined in internal/domain/instance; add a key there and to its CHECK here together.
-- key and locale use COLLATE "C" so that listings sort as in SQLite.
-- ---------------------------------------------------------------------------------------------

-- Event (revision): one change of a setting. value is the canonical string form of the key's
-- scalar type. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_setting_revisions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  key text COLLATE "C" NOT NULL CHECK (key IN (
    'registration_mode', 'max_status_length', 'default_locale', 'local_timeline_enabled', 'federated_timeline_enabled',
    'preview_image', 'remote_media_policy', 'remote_media_cache_ttl', 'authorized_fetch',
    'domain_actions_visibility', 'domain_actions_rationale_visibility'
  )),
  value text NOT NULL,
  revised_by_user_id text COLLATE "C",
  revised_at timestamptz NOT NULL
);
CREATE INDEX instance_setting_revisions_key_id ON instance_setting_revisions (key, id DESC);

-- Projection: the current value of each setting that has been set. Unset keys use the default
-- defined in code.
CREATE TABLE instance_settings (
  key text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id text COLLATE "C" NOT NULL,
  value text NOT NULL
);

-- Event (revision): one version of a localized text. An empty body clears the text in that
-- locale. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_text_revisions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  key text COLLATE "C" NOT NULL CHECK (key IN (
    'name', 'short_description', 'description', 'terms', 'privacy_policy', 'preview_image_alt'
  )),
  locale text COLLATE "C" NOT NULL,
  body text NOT NULL,
  revised_by_user_id text COLLATE "C",
  revised_at timestamptz NOT NULL
);
CREATE INDEX instance_text_revisions_key_locale_id ON instance_text_revisions (key, locale, id DESC);

-- Projection: the current revision of each text in each locale.
CREATE TABLE instance_texts (
  key text COLLATE "C" NOT NULL,
  locale text COLLATE "C" NOT NULL,
  revision_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (key, locale)
);

-- ---------------------------------------------------------------------------------------------
-- instance: rules (data-model §4.9, FR-705). Owned by internal/app/instance.
-- A rule keeps its ID across revisions, so reports cite it by rule_id; retired rules stay.
-- ---------------------------------------------------------------------------------------------

-- Resource: a rule of the instance.
CREATE TABLE instance_rules (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  created_at timestamptz NOT NULL
);

-- Event (revision): one version of a rule, with its position and texts. revised_by_user_id is
-- NULL for a change from the CLI.
CREATE TABLE instance_rule_revisions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  rule_id text COLLATE "C" NOT NULL,
  position integer NOT NULL,
  revised_by_user_id text COLLATE "C",
  revised_at timestamptz NOT NULL
);
CREATE INDEX instance_rule_revisions_rule_id_id ON instance_rule_revisions (rule_id, id DESC);

-- Event (child of a revision): the text of a rule revision in one locale. An empty hint means none.
CREATE TABLE instance_rule_texts (
  rule_revision_id text COLLATE "C" NOT NULL,
  locale text COLLATE "C" NOT NULL,
  text text NOT NULL,
  hint text NOT NULL,
  PRIMARY KEY (rule_revision_id, locale)
);

-- Event: the retirement of a rule. retired_by_user_id is NULL for the CLI.
CREATE TABLE instance_rule_retirements (
  rule_id text COLLATE "C" NOT NULL PRIMARY KEY,
  retired_by_user_id text COLLATE "C",
  retired_at timestamptz NOT NULL
);

-- Projection: the current revision and position of each rule in effect (not retired).
CREATE TABLE instance_rule_heads (
  rule_id text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id text COLLATE "C" NOT NULL,
  position integer NOT NULL
);
CREATE INDEX instance_rule_heads_position ON instance_rule_heads (position);

-- ---------------------------------------------------------------------------------------------
-- content: media files (data-model §4.4, ADR-0005, ADR-0019 §8). Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Resource: a media file, uploaded by a local account or fetched from a remote server.
CREATE TABLE media_files (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  owner_account_id text COLLATE "C" NOT NULL,
  origin text NOT NULL CHECK (origin IN ('upload', 'remote')),
  created_at timestamptz NOT NULL
);
CREATE INDEX media_files_owner_account_id_id ON media_files (owner_account_id, id);

-- Resource (subtype): an upload as received. content_type is detected from the content. The
-- blob under blob_key is private and is deleted once processing has a result.
CREATE TABLE uploaded_media (
  media_id text COLLATE "C" NOT NULL PRIMARY KEY,
  blob_key text NOT NULL,
  content_type text NOT NULL,
  file_size bigint NOT NULL CHECK (file_size >= 0)
);

-- Event: one processing of a media file. The newest row of a file is its current state.
CREATE TABLE media_processing_results (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  media_id text COLLATE "C" NOT NULL,
  outcome text NOT NULL CHECK (outcome IN ('ready', 'failed')),
  media_type text NOT NULL CHECK (media_type IN ('image', 'gifv', 'video', 'audio', 'unknown')),
  width integer,
  height integer,
  duration_ms integer,
  blurhash text,
  processed_at timestamptz NOT NULL
);
CREATE INDEX media_processing_results_media_id_id ON media_processing_results (media_id, id DESC);

-- Event (child of a result): a processed rendition in blob storage, under the public prefix.
CREATE TABLE media_variants (
  result_id text COLLATE "C" NOT NULL,
  kind text NOT NULL CHECK (kind IN ('full', 'thumbnail')),
  blob_key text NOT NULL,
  content_type text NOT NULL,
  file_size bigint NOT NULL CHECK (file_size >= 0),
  width integer NOT NULL,
  height integer NOT NULL,
  PRIMARY KEY (result_id, kind)
);

-- Current value: the focal point of a media file. No row means the center.
CREATE TABLE media_focus (
  media_id text COLLATE "C" NOT NULL PRIMARY KEY,
  focus_x double precision NOT NULL CHECK (focus_x BETWEEN -1.0 AND 1.0),
  focus_y double precision NOT NULL CHECK (focus_y BETWEEN -1.0 AND 1.0)
);

-- Current value: the description that the uploader set on an upload, which a post copies into
-- its attachment (client API #112). No row means none.
CREATE TABLE media_descriptions (
  media_id text COLLATE "C" NOT NULL PRIMARY KEY,
  description text NOT NULL CHECK (description <> '')
);

-- The cleanup of unattached uploads (FR-803) looks up the references to a media file.
CREATE INDEX status_revision_attachments_media_id ON status_revision_attachments (media_id);
CREATE INDEX account_profile_revisions_avatar_media_id ON account_profile_revisions (avatar_media_id);
CREATE INDEX account_profile_revisions_header_media_id ON account_profile_revisions (header_media_id);

-- ---------------------------------------------------------------------------------------------
-- identity: browser sessions of the server-rendered screens (data-model.md §4.1, ADR-0019 §4).
-- ---------------------------------------------------------------------------------------------

-- Resource: a sign-in. token_hash is the SHA-256 digest of the session cookie's value.
CREATE TABLE sessions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  token_hash bytea NOT NULL,
  user_agent text,
  ip text,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX sessions_token_hash ON sessions (token_hash);
CREATE INDEX sessions_user_id ON sessions (user_id);

-- Event: a session ended before it expired.
CREATE TABLE session_revocations (
  session_id text COLLATE "C" NOT NULL PRIMARY KEY,
  reason text NOT NULL CHECK (reason IN ('logout', 'replaced', 'password_changed', 'moderation', 'expired_cleanup')),
  revoked_at timestamptz NOT NULL
);

-- Current value: when a session was last used, updated at most once per interval.
CREATE TABLE session_activity (
  session_id text COLLATE "C" NOT NULL PRIMARY KEY,
  last_seen_at timestamptz NOT NULL
);

-- ---------------------------------------------------------------------------------------------
-- identity: OAuth 2.0 authorization server (data-model.md §4.1, ADR-0019 §4, ADR-0030).
-- Every client_id column except oauth_clients.client_id refers to oauth_clients.id.
-- ---------------------------------------------------------------------------------------------

-- Resource: a registered client. client_secret_hash is NULL for public clients.
CREATE TABLE oauth_clients (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  client_id text COLLATE "C" NOT NULL,
  client_secret_hash bytea,
  name text NOT NULL,
  website text,
  scope text NOT NULL,
  is_first_party boolean NOT NULL,
  owner_user_id text COLLATE "C",
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX oauth_clients_client_id ON oauth_clients (client_id);

-- Current value: the redirect URIs a client may use, matched exactly.
CREATE TABLE oauth_client_redirect_uris (
  client_id text COLLATE "C" NOT NULL,
  redirect_uri text NOT NULL,
  PRIMARY KEY (client_id, redirect_uri)
);

-- Resource: an authorization code, keyed by its SHA-256 digest. session_id is the browser session
-- that authorized it (NULL when it was issued outside one).
CREATE TABLE oauth_authorization_codes (
  code_hash bytea NOT NULL PRIMARY KEY,
  client_id text COLLATE "C" NOT NULL,
  user_id text COLLATE "C" NOT NULL,
  redirect_uri text NOT NULL,
  scope text NOT NULL,
  code_challenge text NOT NULL,
  session_id text COLLATE "C",
  issued_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL
);

-- Event: a code was exchanged; the primary key makes it one-time. token_id is the first token
-- of the family it started.
CREATE TABLE oauth_authorization_code_redemptions (
  code_hash bytea NOT NULL PRIMARY KEY,
  token_id text COLLATE "C" NOT NULL,
  redeemed_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX oauth_authorization_code_redemptions_token_id ON oauth_authorization_code_redemptions (token_id);

-- Resource: an access token with its refresh token. A rotation issues a new row in the same
-- family (family_id is the id of the family's first row).
CREATE TABLE oauth_access_tokens (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  family_id text COLLATE "C" NOT NULL,
  client_id text COLLATE "C" NOT NULL,
  user_id text COLLATE "C" NOT NULL,
  access_token_hash bytea NOT NULL,
  refresh_token_hash bytea,
  scope text NOT NULL,
  rotated_from_token_id text COLLATE "C",
  issued_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  refresh_expires_at timestamptz,
  CONSTRAINT oauth_access_tokens_refresh_pair CHECK ((refresh_token_hash IS NULL) = (refresh_expires_at IS NULL))
);
CREATE UNIQUE INDEX oauth_access_tokens_access_token_hash ON oauth_access_tokens (access_token_hash);
CREATE UNIQUE INDEX oauth_access_tokens_refresh_token_hash ON oauth_access_tokens (refresh_token_hash);
CREATE INDEX oauth_access_tokens_family_id ON oauth_access_tokens (family_id);
CREATE INDEX oauth_access_tokens_user_id ON oauth_access_tokens (user_id);

-- Event: a token row (its access and refresh token) stopped working.
CREATE TABLE oauth_token_revocations (
  token_id text COLLATE "C" NOT NULL PRIMARY KEY,
  reason text NOT NULL CHECK (reason IN ('user', 'rotation', 'refresh_reuse', 'code_reuse', 'password_changed', 'moderation')),
  revoked_at timestamptz NOT NULL
);

-- ---------------------------------------------------------------------------------------------
-- 基盤: key-value store (data-model.md §4.10). Owned by internal/adapter/kv/db.
-- ---------------------------------------------------------------------------------------------

-- Entries of the KeyValueStore port. expires_at is NULL for entries that never expire.
CREATE TABLE kv_entries (
  namespace text NOT NULL,
  key text NOT NULL,
  value bytea NOT NULL,
  expires_at timestamptz,
  PRIMARY KEY (namespace, key)
);
CREATE INDEX kv_entries_expires_at ON kv_entries (expires_at);

-- ---------------------------------------------------------------------------------------------
-- identity: registration, invites, email verification, and one-time tokens
-- (data-model.md §4.1, FR-001, FR-002).
-- ---------------------------------------------------------------------------------------------

-- Event: an email address was verified, at most once per address.
CREATE TABLE user_email_verifications (
  email_address_id text COLLATE "C" NOT NULL PRIMARY KEY,
  verified_at timestamptz NOT NULL
);

-- Resource: a sign-up from the registration page, one per user. reason is the applicant's
-- answer in approval mode; invite_id is the invite it redeemed in invite mode.
CREATE TABLE registration_applications (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  reason text,
  invite_id text COLLATE "C",
  submitted_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX registration_applications_user_id ON registration_applications (user_id);

-- Event: an application was approved or rejected. decided_by_user_id is NULL for the system
-- and for a decision from the CLI.
CREATE TABLE registration_decisions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  application_id text COLLATE "C" NOT NULL,
  decision text NOT NULL CHECK (decision IN ('approved', 'rejected')),
  decided_by text NOT NULL CHECK (decided_by IN ('moderator', 'system')),
  decided_by_user_id text COLLATE "C",
  note text,
  decided_at timestamptz NOT NULL
);
CREATE INDEX registration_decisions_application_id_id ON registration_decisions (application_id, id DESC);

-- Projection: the registration state of each user who signed up; users without a row (created
-- with the CLI) are approved.
CREATE TABLE user_registration_states (
  user_id text COLLATE "C" NOT NULL PRIMARY KEY,
  state text NOT NULL CHECK (state IN ('pending', 'approved', 'rejected')),
  since timestamptz NOT NULL
);
CREATE INDEX user_registration_states_state_since ON user_registration_states (state, since);

-- Resource: an invite code. created_by_user_id is NULL for an invite from the CLI; max_uses
-- and expires_at are NULL for no limit.
CREATE TABLE invites (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  code text COLLATE "C" NOT NULL,
  created_by_user_id text COLLATE "C",
  max_uses integer,
  expires_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX invites_code ON invites (code);

-- Event: one use of an invite. seq is 1 + the uses before it, so that the primary key keeps
-- concurrent sign-ups from exceeding max_uses.
CREATE TABLE invite_redemptions (
  invite_id text COLLATE "C" NOT NULL,
  seq integer NOT NULL,
  user_id text COLLATE "C" NOT NULL,
  redeemed_at timestamptz NOT NULL,
  PRIMARY KEY (invite_id, seq)
);

-- Event: an invite was revoked. revoked_by_user_id is NULL for a revocation from the CLI.
CREATE TABLE invite_revocations (
  invite_id text COLLATE "C" NOT NULL PRIMARY KEY,
  revoked_by_user_id text COLLATE "C",
  revoked_at timestamptz NOT NULL
);

-- Resource: a one-time token sent to a user. token_hash is the SHA-256 digest of the token.
CREATE TABLE user_tokens (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  user_id text COLLATE "C" NOT NULL,
  purpose text NOT NULL CHECK (purpose IN ('email_verification', 'password_reset')),
  email_address_id text COLLATE "C",
  token_hash bytea NOT NULL,
  expires_at timestamptz NOT NULL,
  issued_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX user_tokens_token_hash ON user_tokens (token_hash);

-- Event: a token was used; the primary key makes every token single-use.
CREATE TABLE user_token_redemptions (
  token_id text COLLATE "C" NOT NULL PRIMARY KEY,
  redeemed_at timestamptz NOT NULL
);

-- ---------------------------------------------------------------------------------------------
-- content: boosts, favourites, bookmarks, and pins (data-model §4.4, FR-107, FR-113).
-- Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Resource: a boost (Announce) of a status. visibility copies the status's and never changes;
-- a boost of a local status stays on this instance (ADR-0017).
CREATE TABLE boosts (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text NOT NULL,
  booster_account_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  visibility text NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  is_local boolean NOT NULL,
  boosted_at timestamptz NOT NULL,
  -- only local accounts boost local statuses, and such boosts never federate (ADR-0017).
  CONSTRAINT boosts_local_check CHECK (visibility <> 'local' OR is_local)
);
CREATE UNIQUE INDEX boosts_uri ON boosts (uri);
CREATE INDEX boosts_status_id ON boosts (status_id);

-- Event: a boost was retracted (Undo Announce). A boost is retracted at most once.
CREATE TABLE boost_retractions (
  boost_id text COLLATE "C" NOT NULL PRIMARY KEY,
  origin text NOT NULL CHECK (origin IN ('booster', 'status_deleted', 'moderation', 'remote')),
  retracted_at timestamptz NOT NULL
);

-- Projection: the live boost of each pair, not retracted, of a status that is not deleted.
CREATE TABLE boost_edges (
  booster_account_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  boost_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (booster_account_id, status_id)
);
CREATE INDEX boost_edges_status_id ON boost_edges (status_id);

-- Current value: an account's favourite (Like) of a status.
CREATE TABLE favourites (
  account_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  uri text NOT NULL,
  created_at timestamptz NOT NULL,
  id text COLLATE "C" NOT NULL,
  PRIMARY KEY (account_id, status_id)
);
CREATE UNIQUE INDEX favourites_uri ON favourites (uri);
CREATE INDEX favourites_account_id_id ON favourites (account_id, id);
CREATE INDEX favourites_status_id_id ON favourites (status_id, id);

-- Current value: an account's private bookmark of a status.
CREATE TABLE bookmarks (
  account_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  created_at timestamptz NOT NULL,
  id text COLLATE "C" NOT NULL,
  PRIMARY KEY (account_id, status_id)
);
CREATE INDEX bookmarks_account_id_id ON bookmarks (account_id, id);
-- The retention purge keeps a bookmarked remote status (ADR-0034).
CREATE INDEX bookmarks_status_id ON bookmarks (status_id);

-- Current value: a status its author pinned to the profile (the Featured collection). Each pin
-- holds one of the account's slots 1..5 (content.MaxPins), so the unique slot index caps the pins
-- of an account even under concurrent inserts.
CREATE TABLE status_pins (
  account_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  slot integer NOT NULL CHECK (slot BETWEEN 1 AND 5),
  created_at timestamptz NOT NULL,
  id text COLLATE "C" NOT NULL,
  PRIMARY KEY (account_id, status_id)
);
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
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  kind text NOT NULL CHECK (kind IN ('post', 'boost')),
  status_id text COLLATE "C" NOT NULL,
  author_account_id text COLLATE "C" NOT NULL,
  author_domain text NOT NULL,
  original_author_account_id text COLLATE "C" NOT NULL,
  original_author_domain text NOT NULL,
  visibility text NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  is_local boolean NOT NULL,
  language text NOT NULL,
  revision_seq integer NOT NULL CHECK (revision_seq >= 1),
  is_reply boolean NOT NULL
);
CREATE INDEX feed_entries_author_account_id_id ON feed_entries (author_account_id, id);
CREATE INDEX feed_entries_status_id ON feed_entries (status_id);
CREATE INDEX feed_entries_local ON feed_entries (id) WHERE is_local AND kind = 'post' AND visibility IN ('public', 'local');
CREATE INDEX feed_entries_public ON feed_entries (id) WHERE kind = 'post' AND visibility = 'public';

-- Projection: the recipients (current mentions) of a direct post.
CREATE TABLE feed_direct_recipients (
  entry_id text COLLATE "C" NOT NULL,
  recipient_account_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (recipient_account_id, entry_id)
);
CREATE INDEX feed_direct_recipients_entry_id ON feed_direct_recipients (entry_id);

-- Projection: the current hashtags of a post, with its immutable author and visibility.
CREATE TABLE feed_tag_entries (
  tag_id text COLLATE "C" NOT NULL,
  entry_id text COLLATE "C" NOT NULL,
  author_account_id text COLLATE "C" NOT NULL,
  visibility text NOT NULL CHECK (visibility IN ('public', 'local', 'unlisted', 'private', 'direct')),
  is_local boolean NOT NULL,
  PRIMARY KEY (tag_id, entry_id)
);
CREATE INDEX feed_tag_entries_entry_id ON feed_tag_entries (entry_id);

-- Projection: the active follows, with the follower's show_boosts preference.
CREATE TABLE feed_follow_edges (
  follower_account_id text COLLATE "C" NOT NULL,
  followee_account_id text COLLATE "C" NOT NULL,
  show_boosts boolean NOT NULL,
  PRIMARY KEY (follower_account_id, followee_account_id)
);

-- Projection: the accounts whose entries a viewer does not see: those the viewer blocks, those
-- that block the viewer, and those the viewer mutes (until expires_at; NULL never expires).
CREATE TABLE feed_exclusions (
  viewer_account_id text COLLATE "C" NOT NULL,
  target_account_id text COLLATE "C" NOT NULL,
  reason text NOT NULL CHECK (reason IN ('block', 'blocked_by', 'mute')),
  expires_at timestamptz,
  PRIMARY KEY (viewer_account_id, target_account_id, reason)
);

-- Projection: the moderation restrictions of accounts that bear on timelines, until
-- effective_until (NULL: until lifted).
CREATE TABLE feed_author_restrictions (
  account_id text COLLATE "C" NOT NULL,
  restriction text NOT NULL CHECK (restriction IN ('profile_view', 'visibility_ceiling_private', 'public_discovery')),
  effective_until timestamptz,
  PRIMARY KEY (account_id, restriction)
);

-- Projection: the moderation restrictions of domains (and their subdomains) that bear on
-- timelines.
CREATE TABLE feed_domain_restrictions (
  domain text NOT NULL,
  restriction text NOT NULL CHECK (restriction IN ('profile_view', 'visibility_ceiling_private', 'public_discovery')),
  effective_until timestamptz,
  PRIMARY KEY (domain, restriction)
);

-- ---------------------------------------------------------------------------------------------
-- notification: notifications, their dismissals, read markers, and the exclusions and deleted
-- statuses that hide them (data-model §4.7, FR-205).
-- Owned by internal/app/notification.
-- ---------------------------------------------------------------------------------------------

-- Event: something happened to a local account. dedupe_key makes generating it again from the
-- same domain event, or from an action the source repeats, insert nothing.
CREATE TABLE notifications (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  recipient_account_id text COLLATE "C" NOT NULL,
  type text NOT NULL CHECK (type IN (
    'follow', 'follow_request', 'follow_accepted', 'mention', 'boost', 'favourite', 'poll_closed', 'moderation_notice'
  )),
  source_account_id text COLLATE "C",
  status_id text COLLATE "C",
  moderation_action_id text COLLATE "C",
  dedupe_key text NOT NULL,
  created_at timestamptz NOT NULL,
  -- nobody is notified of their own actions.
  CONSTRAINT notifications_self_check CHECK (source_account_id IS NULL OR source_account_id <> recipient_account_id)
);
CREATE UNIQUE INDEX notifications_recipient_account_id_dedupe_key ON notifications (recipient_account_id, dedupe_key);
CREATE INDEX notifications_recipient_account_id_id ON notifications (recipient_account_id, id);
CREATE INDEX notifications_recipient_account_id_type_id ON notifications (recipient_account_id, type, id);
-- The retention purge keeps a remote status that a notification is about (ADR-0034).
CREATE INDEX notifications_status_id ON notifications (status_id);

-- Event: the recipient dismissed a notification. A notification is dismissed at most once.
CREATE TABLE notification_dismissals (
  notification_id text COLLATE "C" NOT NULL PRIMARY KEY,
  dismissed_at timestamptz NOT NULL
);

-- Current value: the newest notification an account has read; greater IDs are unread.
CREATE TABLE notification_read_markers (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  last_read_id text COLLATE "C" NOT NULL
);

-- Projection of the graph events: while in force (expires_at NULL or later than now), the
-- notifications from target_account_id are hidden from viewer_account_id at read time.
CREATE TABLE notification_exclusions (
  viewer_account_id text COLLATE "C" NOT NULL,
  target_account_id text COLLATE "C" NOT NULL,
  reason text NOT NULL CHECK (reason IN ('block', 'blocked_by', 'mute')),
  expires_at timestamptz,
  PRIMARY KEY (viewer_account_id, target_account_id, reason)
);

-- Projection of content.status_deleted: the notifications about these statuses are hidden at
-- read time; the erasure job deletes them and the row.
CREATE TABLE notification_deleted_statuses (
  status_id text COLLATE "C" NOT NULL PRIMARY KEY
);

-- ---------------------------------------------------------------------------------------------
-- account: what the owner of a local profile typed (data-model §4.2). Owned by
-- internal/app/account; the client API returns it for editing the profile.
-- ---------------------------------------------------------------------------------------------

-- Event (subtype of account_profile_revisions): the profile text of a local revision as typed,
-- from which note_html is rendered.
CREATE TABLE account_profile_revision_sources (
  revision_id text COLLATE "C" NOT NULL PRIMARY KEY,
  note_source text NOT NULL
);

-- Event (subtype of account_profile_fields): the value of a local field as typed, from which
-- value_html is rendered.
CREATE TABLE account_profile_field_sources (
  revision_id text COLLATE "C" NOT NULL,
  position integer NOT NULL,
  value_source text NOT NULL,
  PRIMARY KEY (revision_id, position)
);

-- ---------------------------------------------------------------------------------------------
-- account: remote actors and key rotation (data-model §4.2, docs/architecture/federation.md).
-- Owned by internal/app/account.
-- ---------------------------------------------------------------------------------------------

-- Resource (subtype): an actor of another server.
CREATE TABLE remote_accounts (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  first_seen_at timestamptz NOT NULL
);

-- Event: one fetch of a remote actor's document whose content differed from the previous one.
-- document_digest is the SHA-256 of the document.
CREATE TABLE remote_account_snapshots (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  account_id text COLLATE "C" NOT NULL,
  preferred_username text NOT NULL,
  profile_url text,
  inbox_url text NOT NULL,
  shared_inbox_url text,
  outbox_url text,
  followers_url text,
  following_url text,
  featured_url text,
  followers_total bigint,
  following_total bigint,
  statuses_total bigint,
  document_digest bytea NOT NULL,
  fetched_at timestamptz NOT NULL
);
CREATE INDEX remote_account_snapshots_account_id_id ON remote_account_snapshots (account_id, id DESC);

-- Projection: the newest snapshot of each remote actor, with the endpoints delivery reads and
-- the fetch state of the periodic refresh.
CREATE TABLE remote_account_heads (
  account_id text COLLATE "C" NOT NULL PRIMARY KEY,
  snapshot_id text COLLATE "C" NOT NULL,
  inbox_url text NOT NULL,
  shared_inbox_url text,
  last_fetched_at timestamptz NOT NULL,
  consecutive_fetch_failures integer NOT NULL CHECK (consecutive_fetch_failures >= 0)
);
CREATE INDEX remote_account_heads_last_fetched_at ON remote_account_heads (last_fetched_at);
CREATE INDEX remote_account_heads_shared_inbox_url ON remote_account_heads (shared_inbox_url);

-- Event: a WebFinger check of acct:{username}@{domain}. verified means that its self link points
-- at the actor's uri; mismatch means that it answered otherwise, and the account has no handle
-- until a later check verifies one.
CREATE TABLE remote_account_handle_verifications (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  account_id text COLLATE "C" NOT NULL,
  username text NOT NULL,
  domain text NOT NULL,
  outcome text NOT NULL CHECK (outcome IN ('verified', 'mismatch')),
  checked_at timestamptz NOT NULL
);
CREATE INDEX remote_account_handle_verifications_account_id_id ON remote_account_handle_verifications (account_id, id DESC);

-- Event (subtype of account_profile_revisions): the snapshot a remote revision came from and the
-- version of the HTML policy that sanitized its note_html and value_html (ADR-0019 §2).
CREATE TABLE remote_profile_revisions (
  revision_id text COLLATE "C" NOT NULL PRIMARY KEY,
  snapshot_id text COLLATE "C" NOT NULL,
  html_policy_version integer NOT NULL
);

-- Event: a key was replaced or withdrawn. Verification uses the newest key of a key_id without
-- a revocation.
CREATE TABLE account_key_revocations (
  account_key_id text COLLATE "C" NOT NULL PRIMARY KEY,
  revoked_at timestamptz NOT NULL
);

-- ---------------------------------------------------------------------------------------------
-- content: remote statuses and media (data-model §4.4, docs/architecture/federation.md §9).
-- Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Event (subtype of status_revisions): the activity that carried a remote revision (NULL for a
-- revision made from a fetch) and the version of the HTML policy that sanitized its bodies.
CREATE TABLE remote_status_revisions (
  revision_id text COLLATE "C" NOT NULL PRIMARY KEY,
  activity_uri text,
  html_policy_version integer NOT NULL
);

-- Resource (subtype of media_files): a remote file as the received document declared it. The
-- file itself is not fetched. owner_account_id copies media_files.owner_account_id.
CREATE TABLE remote_media (
  media_id text COLLATE "C" NOT NULL PRIMARY KEY,
  owner_account_id text COLLATE "C" NOT NULL,
  remote_url text NOT NULL,
  declared_content_type text,
  declared_media_type text NOT NULL CHECK (declared_media_type IN ('image', 'gifv', 'video', 'audio', 'unknown')),
  width integer,
  height integer,
  blurhash text,
  focus_x double precision CHECK (focus_x BETWEEN -1.0 AND 1.0),
  focus_y double precision CHECK (focus_y BETWEEN -1.0 AND 1.0),
  description text
);
CREATE UNIQUE INDEX remote_media_owner_account_id_remote_url ON remote_media (owner_account_id, remote_url);

-- Current value (cache): a remote file copied into blob storage under remote/ (FR-409,
-- NFR-COST-4). The cleanup removes rows past expires_at and the least recently accessed ones.
CREATE TABLE remote_media_cache (
  url text NOT NULL PRIMARY KEY,
  blob_key text NOT NULL,
  content_type text NOT NULL,
  file_size bigint NOT NULL,
  fetched_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  last_accessed_at timestamptz NOT NULL
);
CREATE INDEX remote_media_cache_expires_at ON remote_media_cache (expires_at);
CREATE INDEX remote_media_cache_last_accessed_at ON remote_media_cache (last_accessed_at);

-- ---------------------------------------------------------------------------------------------
-- federation: inbox, delivery, and remote instances (data-model §4.8,
-- docs/architecture/federation.md). Owned by internal/app/federation.
-- ---------------------------------------------------------------------------------------------

-- Event: an activity accepted at an inbox, once per activity_uri (synthesized from the body's
-- digest when the activity has no id). trust is refetch when the signer is not the actor.
CREATE TABLE received_activities (
  activity_uri text NOT NULL PRIMARY KEY,
  actor_uri text NOT NULL,
  type text NOT NULL,
  signature_scheme text NOT NULL CHECK (signature_scheme IN ('rfc9421', 'cavage')),
  trust text NOT NULL CHECK (trust IN ('signed', 'refetch')),
  received_at timestamptz NOT NULL
);
CREATE INDEX received_activities_received_at ON received_activities (received_at);

-- Event: the received document with the bytes it arrived with, read by the asynchronous
-- processing and kept briefly for investigation and fixtures.
CREATE TABLE received_activity_documents (
  activity_uri text NOT NULL PRIMARY KEY,
  document bytea NOT NULL
);

-- Event: a Delete of an unknown object or an Undo of an unknown target. A Create, Update, or
-- Follow of the uri that arrives later is dropped.
CREATE TABLE remote_object_tombstones (
  uri text NOT NULL PRIMARY KEY,
  actor_uri text NOT NULL,
  activity_uri text NOT NULL,
  received_at timestamptz NOT NULL
);
CREATE INDEX remote_object_tombstones_received_at ON remote_object_tombstones (received_at);

-- Event: an activity a local actor sends about one of its objects.
CREATE TABLE outbound_activities (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  activity_uri text NOT NULL,
  type text NOT NULL,
  actor_account_id text COLLATE "C" NOT NULL,
  object_type text NOT NULL CHECK (object_type IN ('status', 'boost', 'favourite', 'follow', 'block', 'account', 'report')),
  object_id text COLLATE "C" NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX outbound_activities_activity_uri ON outbound_activities (activity_uri);
CREATE INDEX outbound_activities_object_type_object_id_id ON outbound_activities (object_type, object_id, id);

-- Event: the body of an outbound activity, built once by the fan-out so that every delivery
-- and retry sends the same bytes.
CREATE TABLE outbound_activity_documents (
  outbound_activity_id text COLLATE "C" NOT NULL PRIMARY KEY,
  document bytea NOT NULL,
  built_at timestamptz NOT NULL
);

-- Event: an inbox (shared inboxes merged) an outbound activity is delivered to. It is also the
-- record of where a status went, for its later Update and Delete.
CREATE TABLE deliveries (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  outbound_activity_id text COLLATE "C" NOT NULL,
  inbox_url text NOT NULL,
  domain text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX deliveries_outbound_activity_id_inbox_url ON deliveries (outbound_activity_id, inbox_url);

-- Event: one attempt of a delivery. signature_scheme is NULL when no response came back.
CREATE TABLE delivery_attempts (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  delivery_id text COLLATE "C" NOT NULL,
  outcome text NOT NULL CHECK (outcome IN ('succeeded', 'retryable_failure', 'permanent_failure', 'skipped')),
  signature_scheme text CHECK (signature_scheme IN ('rfc9421', 'cavage')),
  http_status integer,
  error text,
  attempted_at timestamptz NOT NULL
);
CREATE INDEX delivery_attempts_delivery_id_id ON delivery_attempts (delivery_id, id);

-- Resource: a server this instance has seen.
CREATE TABLE remote_instances (
  domain text NOT NULL PRIMARY KEY,
  first_seen_at timestamptz NOT NULL
);

-- Event: the software a server reported in its NodeInfo.
CREATE TABLE instance_nodeinfo_observations (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  domain text NOT NULL,
  software_name text NOT NULL,
  software_version text NOT NULL,
  observed_at timestamptz NOT NULL
);
CREATE INDEX instance_nodeinfo_observations_domain_id ON instance_nodeinfo_observations (domain, id DESC);

-- Event: a server was marked unavailable or available as a delivery target, with the reason.
CREATE TABLE instance_availability_events (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  domain text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('marked_unavailable', 'marked_available')),
  reason text NOT NULL CHECK (reason IN (
    'failure_days', 'delivery_succeeded', 'inbound_request', 'probe_succeeded', 'operator'
  )),
  occurred_at timestamptz NOT NULL
);
CREATE INDEX instance_availability_events_domain_id ON instance_availability_events (domain, id DESC);

-- Projection: whether a server is a delivery target, from delivery_attempts and
-- instance_availability_events. failure_days counts the distinct UTC days of the latest run of
-- failures; last_failure_day is YYYYMMDD, or '' without a failure.
CREATE TABLE instance_health (
  domain text NOT NULL PRIMARY KEY,
  state text NOT NULL CHECK (state IN ('available', 'unavailable')),
  since timestamptz NOT NULL,
  failure_days integer NOT NULL CHECK (failure_days >= 0),
  last_failure_day text NOT NULL CHECK (length(last_failure_day) IN (0, 8))
);
CREATE INDEX instance_health_state ON instance_health (state);

-- ---------------------------------------------------------------------------------------------
-- content: custom emoji and polls of statuses (data-model §4.4). Owned by internal/app/content.
-- ---------------------------------------------------------------------------------------------

-- Resource: a custom emoji, unique by its shortcode on a domain ('' for this instance). uri is
-- the id of a remote emoji's Emoji object.
CREATE TABLE custom_emojis (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  shortcode text NOT NULL,
  domain text NOT NULL,
  uri text,
  created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX custom_emojis_shortcode_domain ON custom_emojis (shortcode, domain);
CREATE UNIQUE INDEX custom_emojis_uri ON custom_emojis (uri);

-- Event (revision): the image of a custom emoji. The newest row is the current image.
CREATE TABLE custom_emoji_images (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  emoji_id text COLLATE "C" NOT NULL,
  media_id text COLLATE "C" NOT NULL,
  revised_at timestamptz NOT NULL
);
CREATE INDEX custom_emoji_images_emoji_id_id ON custom_emoji_images (emoji_id, id DESC);
-- The remote media cache looks up whether a media file is a custom emoji image (avatars policy).
CREATE INDEX custom_emoji_images_media_id ON custom_emoji_images (media_id);

-- Event (child of a revision): the poll of a status revision (FR-112). closes_at is NULL for a
-- poll without an end.
CREATE TABLE polls (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id text COLLATE "C" NOT NULL,
  multiple boolean NOT NULL,
  closes_at timestamptz
);
CREATE UNIQUE INDEX polls_revision_id ON polls (revision_id);

-- Event (child of a poll): one choice of a poll.
CREATE TABLE poll_options (
  poll_id text COLLATE "C" NOT NULL,
  position integer NOT NULL CHECK (position >= 0),
  title text NOT NULL,
  PRIMARY KEY (poll_id, position)
);

-- Event: the counts of a remote poll as its server reported them (in a Create or an Update).
CREATE TABLE poll_tally_observations (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  poll_id text COLLATE "C" NOT NULL,
  voters_count bigint CHECK (voters_count >= 0),
  observed_at timestamptz NOT NULL
);
CREATE INDEX poll_tally_observations_poll_id_id ON poll_tally_observations (poll_id, id DESC);

-- Event (child of an observation): the votes of one choice.
CREATE TABLE poll_tally_observation_counts (
  observation_id text COLLATE "C" NOT NULL,
  position integer NOT NULL CHECK (position >= 0),
  votes bigint NOT NULL CHECK (votes >= 0),
  PRIMARY KEY (observation_id, position)
);

-- Projection: the votes of each choice of a poll, from its newest observation (remote polls).
-- voters_count is NULL when the server did not report it.
CREATE TABLE poll_tallies (
  poll_id text COLLATE "C" NOT NULL,
  position integer NOT NULL CHECK (position >= 0),
  votes bigint NOT NULL CHECK (votes >= 0),
  voters_count bigint CHECK (voters_count >= 0),
  PRIMARY KEY (poll_id, position)
);

-- graph and federation: re-sent Follows and early Undo Follows (data-model §4.5, §4.8) ----------

-- Event: a Follow received again, under another URI, for a follow that is already active. The
-- follow keeps its row; its Accept is sent again for the new URI.
CREATE TABLE follow_resends (
  uri text NOT NULL PRIMARY KEY,
  follow_id text COLLATE "C" NOT NULL,
  received_at timestamptz NOT NULL
);

-- Event: an Undo of an unknown Follow, by the pair of its actor and the object that the embedded
-- Follow names. The first Follow of the pair that arrives later is dropped, and its URI recorded
-- in remote_object_tombstones with this activity_uri.
CREATE TABLE remote_follow_tombstones (
  activity_uri text NOT NULL PRIMARY KEY,
  actor_uri text NOT NULL,
  object_uri text NOT NULL,
  received_at timestamptz NOT NULL
);
CREATE INDEX remote_follow_tombstones_actor_uri_object_uri ON remote_follow_tombstones (actor_uri, object_uri, received_at);
CREATE INDEX remote_follow_tombstones_received_at ON remote_follow_tombstones (received_at);
CREATE INDEX remote_object_tombstones_activity_uri ON remote_object_tombstones (activity_uri);

-- ---------------------------------------------------------------------------------------------
-- moderation: actions on accounts and domains, the restrictions they impose, and reports
-- (data-model §4.3, §5.2, docs/architecture/federation.md §11). Owned by internal/app/moderation.
-- ---------------------------------------------------------------------------------------------

-- Event: a moderation action on an account. A ban never expires.
CREATE TABLE moderation_actions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  target_account_id text COLLATE "C" NOT NULL,
  kind text NOT NULL CHECK (kind IN (
    'warn', 'mark_statuses_sensitive', 'delete_statuses', 'force_sensitive', 'limit',
    'restrict_posting', 'restrict_interaction', 'disable_login', 'suspend', 'ban'
  )),
  reason_category text NOT NULL CHECK (reason_category IN ('spam', 'legal', 'violation', 'other')),
  public_reason text,
  private_note text,
  report_id text COLLATE "C",
  issuer text NOT NULL CHECK (issuer IN ('moderator', 'system', 'remote')),
  issued_by_user_id text COLLATE "C",
  notify_target boolean NOT NULL,
  expires_at timestamptz,
  issued_at timestamptz NOT NULL,
  CHECK (kind <> 'ban' OR expires_at IS NULL),
  CHECK (expires_at IS NULL OR expires_at > issued_at)
);
CREATE INDEX moderation_actions_target_account_id_id ON moderation_actions (target_account_id, id DESC);
CREATE INDEX moderation_actions_report_id ON moderation_actions (report_id);

-- Event: a moderation action on an account was lifted. Expiry is not a lift.
-- lifted_by_user_id is NULL for the system.
CREATE TABLE moderation_action_lifts (
  action_id text COLLATE "C" NOT NULL PRIMARY KEY,
  lifted_by_user_id text COLLATE "C",
  reason text,
  lifted_at timestamptz NOT NULL
);

-- Projection: the restrictions that the actions on an account impose, one row per restriction
-- and action, until effective_until (NULL: until lifted).
CREATE TABLE account_restrictions (
  account_id text COLLATE "C" NOT NULL,
  restriction text NOT NULL CHECK (restriction IN (
    'login', 'post', 'interact', 'visibility_ceiling_private', 'public_discovery',
    'unsolicited_reach', 'media_sensitive', 'profile_view', 'federation', 'media_fetch',
    'report_intake'
  )),
  source_action_id text COLLATE "C" NOT NULL,
  effective_until timestamptz,
  PRIMARY KEY (account_id, restriction, source_action_id)
);
CREATE INDEX account_restrictions_source_action_id ON account_restrictions (source_action_id);
-- The removal of expired rows, which tells the end of expired suspensions (federation.md §11.1).
CREATE INDEX account_restrictions_effective_until ON account_restrictions (effective_until);

-- Event: a moderation action on a domain and its subdomains. issued_by_user_id is NULL for the
-- CLI.
CREATE TABLE domain_moderation_actions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  domain text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('limit', 'suspend', 'reject_media', 'reject_reports', 'force_sensitive')),
  public_comment text,
  private_note text,
  obfuscate boolean NOT NULL,
  issued_by_user_id text COLLATE "C",
  expires_at timestamptz,
  issued_at timestamptz NOT NULL,
  CHECK (expires_at IS NULL OR expires_at > issued_at)
);
CREATE INDEX domain_moderation_actions_domain_id ON domain_moderation_actions (domain, id DESC);

-- Event: a domain action was lifted. lifted_by_user_id is NULL for the CLI.
CREATE TABLE domain_moderation_action_lifts (
  action_id text COLLATE "C" NOT NULL PRIMARY KEY,
  lifted_by_user_id text COLLATE "C",
  lifted_at timestamptz NOT NULL
);

-- Projection: the restrictions that the actions on a domain impose on it and its subdomains,
-- until effective_until (NULL: until lifted).
CREATE TABLE domain_restrictions (
  domain text NOT NULL,
  restriction text NOT NULL CHECK (restriction IN (
    'public_discovery', 'unsolicited_reach', 'profile_view', 'federation', 'media_fetch',
    'report_intake', 'media_sensitive'
  )),
  source_action_id text COLLATE "C" NOT NULL,
  effective_until timestamptz,
  PRIMARY KEY (domain, restriction, source_action_id)
);
CREATE INDEX domain_restrictions_source_action_id ON domain_restrictions (source_action_id);

-- Resource: a report of an account. uri is the id of a received Flag (or the URI synthesized
-- for a Flag without one), NULL for a local report.
CREATE TABLE reports (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  uri text,
  reporter_account_id text COLLATE "C" NOT NULL,
  target_account_id text COLLATE "C" NOT NULL,
  category text NOT NULL CHECK (category IN ('spam', 'legal', 'violation', 'other')),
  comment text NOT NULL,
  forwarded boolean NOT NULL,
  filed_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX reports_uri ON reports (uri);
CREATE INDEX reports_target_account_id_id ON reports (target_account_id, id);

-- Resource (child of a report): the statuses a report points at.
CREATE TABLE report_statuses (
  report_id text COLLATE "C" NOT NULL,
  status_id text COLLATE "C" NOT NULL,
  PRIMARY KEY (report_id, status_id)
);

-- Projection: whether a report is open, how it was resolved, and who handles it. outcome is
-- that of the latest resolution, set exactly when the report is resolved.
CREATE TABLE report_states (
  report_id text COLLATE "C" NOT NULL PRIMARY KEY,
  state text NOT NULL CHECK (state IN ('open', 'resolved')),
  outcome text CHECK (outcome IN ('actioned', 'dismissed')),
  assignee_user_id text COLLATE "C",
  since timestamptz NOT NULL,
  CHECK ((state = 'resolved') = (outcome IS NOT NULL))
);
CREATE INDEX report_states_state_report_id ON report_states (state, report_id);

-- Event: a moderator took a report, or gave it up (assignee_user_id NULL).
-- assigned_by_user_id is NULL for the CLI.
CREATE TABLE report_assignments (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  report_id text COLLATE "C" NOT NULL,
  assignee_user_id text COLLATE "C",
  assigned_by_user_id text COLLATE "C",
  assigned_at timestamptz NOT NULL
);
CREATE INDEX report_assignments_report_id_id ON report_assignments (report_id, id DESC);

-- Event: a moderator resolved a report. resolved_by_user_id is NULL for the CLI.
CREATE TABLE report_resolutions (
  id text COLLATE "C" NOT NULL PRIMARY KEY,
  report_id text COLLATE "C" NOT NULL,
  outcome text NOT NULL CHECK (outcome IN ('actioned', 'dismissed')),
  resolved_by_user_id text COLLATE "C",
  resolved_at timestamptz NOT NULL
);
CREATE INDEX report_resolutions_report_id_id ON report_resolutions (report_id, id DESC);
