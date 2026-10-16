-- SPDX-License-Identifier: AGPL-3.0-only
-- SQLite schema for spike #17. Timestamps are INTEGER unix milliseconds (UTC).

CREATE TABLE accounts (
    id              TEXT    NOT NULL PRIMARY KEY, -- UUIDv7, 36 chars
    username        TEXT    NOT NULL,
    domain          TEXT    NOT NULL DEFAULT '',  -- '' for local accounts
    uri             TEXT    NOT NULL UNIQUE,
    display_name    TEXT    NOT NULL DEFAULT '',
    followers_count INTEGER NOT NULL DEFAULT 0,
    following_count INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
) STRICT;

CREATE TABLE follows (
    id          TEXT    NOT NULL PRIMARY KEY, -- UUIDv7
    follower_id TEXT    NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    target_id   TEXT    NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL,
    UNIQUE (follower_id, target_id)
) STRICT;

CREATE INDEX follows_target_id_id ON follows (target_id, id);
