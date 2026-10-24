-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Desired-state schema for PostgreSQL (ADR-0023): the tables of the instance module
-- (docs/architecture/data-model/04-tables/09-instance.md). schema/README.md describes the layout
-- and the conventions of this directory.

-- ---------------------------------------------------------------------------------------------
-- instance: settings and texts (data-model §4.9). Owned by internal/app/instance.
-- Keys are defined in internal/domain/instance; add a key there and to its CHECK here together.
-- key uses COLLATE "C" so that listings sort as in SQLite.
-- ---------------------------------------------------------------------------------------------

-- Event (revision): one change of a setting. value is the canonical string form of the key's
-- scalar type. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_setting_revisions (
  id uuid NOT NULL PRIMARY KEY,
  key text COLLATE "C" NOT NULL CHECK (key IN (
    'registration_mode', 'max_status_length', 'default_locale', 'local_timeline_access', 'federated_timeline_access',
    'preview_image', 'contact_account', 'contact_email', 'thumbnail', 'remote_media_policy', 'remote_media_cache_ttl', 'authorized_fetch',
    'domain_actions_visibility', 'domain_actions_rationale_visibility', 'appeal_window',
    'retention_session_origins', 'retention_registration_origins', 'retention_reports',
    'retention_moderation_actions', 'retention_audit_log',
    'search_indexing', 'ai_usage', 'indexable_default', 'security_contact', 'require_media_description', 'link_cards_enabled',
    'finance_visibility', 'finance_viewer_item_names', 'finance_viewer_item_memos', 'finance_viewer_income',
    'supporter_list_visibility', 'income_visibility', 'federation_mode',
    'user_app_registration'
  )),
  value text NOT NULL,
  revised_by_user_id uuid,
  revised_at timestamptz NOT NULL
);
CREATE INDEX instance_setting_revisions_key_id ON instance_setting_revisions (key, id DESC);

-- Projection: the current value of each setting that has been set. Unset keys use the default
-- defined in code.
CREATE TABLE instance_settings (
  key text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id uuid NOT NULL,
  value text NOT NULL
);

-- Event (revision): one version of an instance text, one value shown in every locale. An empty
-- body clears the text. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_text_revisions (
  id uuid NOT NULL PRIMARY KEY,
  key text COLLATE "C" NOT NULL CHECK (key IN (
    'name', 'short_description', 'description', 'terms', 'privacy_policy', 'preview_image_alt', 'thumbnail_alt',
    'commerce_disclosure'
  )),
  body text NOT NULL,
  revised_by_user_id uuid,
  revised_at timestamptz NOT NULL
);
CREATE INDEX instance_text_revisions_key_id ON instance_text_revisions (key, id DESC);

-- Projection: the current revision of each text.
CREATE TABLE instance_texts (
  key text COLLATE "C" NOT NULL PRIMARY KEY,
  revision_id uuid NOT NULL
);

-- ---------------------------------------------------------------------------------------------
-- instance: theme (data-model §4.9, ADR-0059, FR-605). Owned by internal/app/instance.
-- ---------------------------------------------------------------------------------------------

-- Event (revision): one version of the instance theme: the administrator's custom CSS, with its
-- token overrides in instance_theme_tokens. A revision without tokens and CSS resets the theme.
-- revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE instance_theme_revisions (
  id uuid NOT NULL PRIMARY KEY,
  custom_css text NOT NULL,
  revised_by_user_id uuid,
  revised_at timestamptz NOT NULL
);

-- Event (child of a revision): the value of one overridden design token in one color scheme
-- (light and dark for a color token, all for the others). The token list is generated from the
-- web client's theme.css (internal/domain/instance/theme_tokens.gen.go).
CREATE TABLE instance_theme_tokens (
  theme_revision_id uuid NOT NULL,
  token text COLLATE "C" NOT NULL CHECK (token IN (
    'color-surface', 'color-surface-raised', 'color-surface-sunken', 'color-soft', 'color-fg',
   'color-fg-muted', 'color-accent', 'color-accent-hover', 'color-accent-fg', 'color-accent-fill',
   'color-accent-fill-hover', 'color-accent-fill-fg', 'color-danger', 'color-danger-fg',
   'color-danger-bg', 'color-success', 'color-success-fg', 'color-success-bg', 'color-warning',
   'color-warning-fg', 'color-warning-bg', 'color-border', 'color-border-strong', 'color-focus',
   'color-toast', 'color-toast-fg', 'color-toast-action', 'color-toast-success',
   'color-toast-danger', 'color-toast-muted', 'color-overlay', 'color-alt-overlay', 'color-pressed', 'color-pressed-fg', 'color-media-chip', 'color-wave', 'color-spark', 'font-sans',
   'font-mono', 'font-serif', 'text-xs', 'text-xs--line-height', 'text-sm', 'text-sm--line-height',
   'text-base', 'text-base--line-height', 'text-lg', 'text-lg--line-height', 'text-xl',
   'text-xl--line-height', 'text-2xl', 'text-2xl--line-height', 'text-3xl', 'text-3xl--line-height',
   'spacing', 'radius-sm', 'radius-md', 'radius-lg', 'radius-xl', 'radius-2xl', 'elevation-card',
   'elevation-sm', 'elevation-md', 'elevation-lg', 'ease-standard', 'ease-enter', 'ease-exit',
   'transition-duration-fast', 'transition-duration-normal', 'transition-duration-slow',
   'default-transition-duration', 'default-transition-timing-function'
  )),
  scheme text COLLATE "C" NOT NULL CHECK (scheme IN ('light', 'dark', 'all')),
  value text NOT NULL,
  PRIMARY KEY (theme_revision_id, token, scheme)
);

-- Projection: the current theme revision, one row once a theme has been saved.
CREATE TABLE instance_theme (
  id integer NOT NULL PRIMARY KEY CHECK (id = 1),
  revision_id uuid NOT NULL
);

-- ---------------------------------------------------------------------------------------------
-- instance: rules (data-model §4.9, FR-705). Owned by internal/app/instance.
-- A rule keeps its ID across revisions, so reports cite it by rule_id; retired rules stay.
-- ---------------------------------------------------------------------------------------------

-- Resource: a rule of the instance.
CREATE TABLE instance_rules (
  id uuid NOT NULL PRIMARY KEY,
  created_at timestamptz NOT NULL
);

-- Event (revision): one version of a rule, with its position, its text, and its hint (an optional
-- explanation, empty for none), one value shown in every locale. revised_by_user_id is NULL for a
-- change from the CLI.
CREATE TABLE instance_rule_revisions (
  id uuid NOT NULL PRIMARY KEY,
  rule_id uuid NOT NULL,
  position integer NOT NULL,
  text text NOT NULL,
  hint text NOT NULL,
  revised_by_user_id uuid,
  revised_at timestamptz NOT NULL
);
CREATE INDEX instance_rule_revisions_rule_id_id ON instance_rule_revisions (rule_id, id DESC);

-- Event: the retirement of a rule. retired_by_user_id is NULL for the CLI.
CREATE TABLE instance_rule_retirements (
  rule_id uuid NOT NULL PRIMARY KEY,
  retired_by_user_id uuid,
  retired_at timestamptz NOT NULL
);

-- Projection: the current revision and position of each rule in effect (not retired).
CREATE TABLE instance_rule_heads (
  rule_id uuid NOT NULL PRIMARY KEY,
  revision_id uuid NOT NULL,
  position integer NOT NULL
);
CREATE INDEX instance_rule_heads_position ON instance_rule_heads (position);

-- ---------------------------------------------------------------------------------------------
-- instance: server-wide announcements (data-model §4.9, FR-726). Owned by internal/app/instance.
-- ---------------------------------------------------------------------------------------------

-- Resource: an announcement to every local user.
CREATE TABLE announcements (
  id uuid NOT NULL PRIMARY KEY,
  created_at timestamptz NOT NULL
);

-- Event (revision): one version of an announcement: its body in Markdown and the window in which
-- it shows once published. A NULL starts_at shows it from its publication, a NULL ends_at until
-- it is withdrawn. revised_by_user_id is NULL for a change from the CLI.
CREATE TABLE announcement_revisions (
  id uuid NOT NULL PRIMARY KEY,
  announcement_id uuid NOT NULL,
  body text NOT NULL,
  starts_at timestamptz,
  ends_at timestamptz,
  revised_by_user_id uuid,
  revised_at timestamptz NOT NULL,
  CHECK (starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at)
);
CREATE INDEX announcement_revisions_announcement_id_id ON announcement_revisions (announcement_id, id DESC);

-- Event: an announcement was published or withdrawn; a withdrawal is for good.
-- changed_by_user_id is NULL for the CLI.
CREATE TABLE announcement_state_changes (
  id uuid NOT NULL PRIMARY KEY,
  announcement_id uuid NOT NULL,
  state text NOT NULL CHECK (state IN ('published', 'withdrawn')),
  changed_by_user_id uuid,
  changed_at timestamptz NOT NULL
);
CREATE INDEX announcement_state_changes_announcement_id_id ON announcement_state_changes (announcement_id, id DESC);

-- Projection: the current revision and state of each announcement. published_at is the time of
-- its latest publication, set exactly when it has been published.
CREATE TABLE announcement_heads (
  announcement_id uuid NOT NULL PRIMARY KEY,
  revision_id uuid NOT NULL,
  state text NOT NULL CHECK (state IN ('draft', 'published', 'withdrawn')),
  published_at timestamptz,
  CHECK ((state = 'draft') = (published_at IS NULL))
);
CREATE INDEX announcement_heads_state ON announcement_heads (state);

-- Current value: the announcements a user has read (ADR-0020 rule 5: the history means nothing).
CREATE TABLE announcement_reads (
  user_id uuid NOT NULL,
  announcement_id uuid NOT NULL,
  read_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, announcement_id)
);
