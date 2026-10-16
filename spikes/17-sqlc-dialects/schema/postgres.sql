-- SPDX-License-Identifier: AGPL-3.0-only
-- PostgreSQL schema for spike #17. Timestamps are timestamptz; the adapter truncates to milliseconds.

CREATE TABLE accounts (
    id              text        COLLATE "C" NOT NULL PRIMARY KEY, -- UUIDv7; "C" = byte order
    username        text        NOT NULL,
    domain          text        NOT NULL DEFAULT '',
    uri             text        NOT NULL UNIQUE,
    display_name    text        NOT NULL DEFAULT '',
    followers_count bigint      NOT NULL DEFAULT 0,
    following_count bigint      NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL
);

CREATE TABLE follows (
    id          text        COLLATE "C" NOT NULL PRIMARY KEY, -- UUIDv7 (pagination cursor)
    follower_id text        COLLATE "C" NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    target_id   text        COLLATE "C" NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL,
    UNIQUE (follower_id, target_id)
);

CREATE INDEX follows_target_id_id ON follows (target_id, id);
