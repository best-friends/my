-- SPDX-License-Identifier: AGPL-3.0-only
-- timeline: feeds (data-model 4.6, 6). Every query reads only the feed_* tables.

-- Projector writes ------------------------------------------------------------------------------

-- name: InsertFeedEntry :execrows
INSERT INTO feed_entries (
  id, kind, status_id, author_account_id, author_domain, original_author_account_id,
  original_author_domain, visibility, is_local, language, revision_seq, is_reply
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO NOTHING;

-- name: GetFeedEntry :one
SELECT id, kind, status_id, author_account_id, author_domain, original_author_account_id,
  original_author_domain, visibility, is_local, language, revision_seq, is_reply
FROM feed_entries WHERE id = ?;

-- UpdateFeedEntriesRevision moves the entries of a status (the post and its boosts) to a
-- newer revision; an older or repeated revision changes nothing.
-- name: UpdateFeedEntriesRevision :execrows
UPDATE feed_entries SET language = sqlc.arg(language), revision_seq = sqlc.arg(revision_seq)
WHERE status_id = sqlc.arg(status_id) AND revision_seq < sqlc.arg(revision_seq);

-- name: DeleteFeedEntry :exec
DELETE FROM feed_entries WHERE id = ?;

-- name: DeleteFeedEntriesOfStatus :exec
DELETE FROM feed_entries WHERE status_id = ?;

-- name: InsertFeedDirectRecipient :exec
INSERT INTO feed_direct_recipients (entry_id, recipient_account_id) VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: DeleteFeedDirectRecipients :exec
DELETE FROM feed_direct_recipients WHERE entry_id = ?;

-- name: InsertFeedTagEntry :exec
INSERT INTO feed_tag_entries (tag_id, entry_id, author_account_id, visibility, is_local) VALUES (?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: DeleteFeedTagEntries :exec
DELETE FROM feed_tag_entries WHERE entry_id = ?;

-- name: UpsertFeedFollowEdge :exec
INSERT INTO feed_follow_edges (follower_account_id, followee_account_id, show_boosts) VALUES (?, ?, ?)
ON CONFLICT (follower_account_id, followee_account_id) DO UPDATE SET show_boosts = excluded.show_boosts;

-- name: DeleteFeedFollowEdge :exec
DELETE FROM feed_follow_edges WHERE follower_account_id = ? AND followee_account_id = ?;

-- name: UpsertFeedExclusion :exec
INSERT INTO feed_exclusions (viewer_account_id, target_account_id, reason, expires_at) VALUES (?, ?, ?, ?)
ON CONFLICT (viewer_account_id, target_account_id, reason) DO UPDATE SET expires_at = excluded.expires_at;

-- name: DeleteFeedExclusion :exec
DELETE FROM feed_exclusions WHERE viewer_account_id = ? AND target_account_id = ? AND reason = ?;

-- name: UpsertFeedAuthorRestriction :exec
INSERT INTO feed_author_restrictions (account_id, restriction, effective_until) VALUES (?, ?, ?)
ON CONFLICT (account_id, restriction) DO UPDATE SET effective_until = excluded.effective_until;

-- name: DeleteFeedAuthorRestriction :exec
DELETE FROM feed_author_restrictions WHERE account_id = ? AND restriction = ?;

-- name: UpsertFeedDomainRestriction :exec
INSERT INTO feed_domain_restrictions (domain, restriction, effective_until) VALUES (?, ?, ?)
ON CONFLICT (domain, restriction) DO UPDATE SET effective_until = excluded.effective_until;

-- name: DeleteFeedDomainRestriction :exec
DELETE FROM feed_domain_restrictions WHERE domain = ? AND restriction = ?;

-- Rebuild ---------------------------------------------------------------------------------------

-- name: DeleteAllFeedEntries :exec
DELETE FROM feed_entries;

-- name: DeleteAllFeedDirectRecipients :exec
DELETE FROM feed_direct_recipients;

-- name: DeleteAllFeedTagEntries :exec
DELETE FROM feed_tag_entries;

-- name: DeleteAllFeedFollowEdges :exec
DELETE FROM feed_follow_edges;

-- name: DeleteAllFeedExclusions :exec
DELETE FROM feed_exclusions;

-- Projection contents, for verifying the projections -----------------------------------------

-- name: ListAllFeedEntries :many
SELECT id, kind, status_id, author_account_id, author_domain, original_author_account_id,
  original_author_domain, visibility, is_local, language, revision_seq, is_reply
FROM feed_entries ORDER BY id;

-- name: ListAllFeedDirectRecipients :many
SELECT entry_id, recipient_account_id FROM feed_direct_recipients ORDER BY entry_id, recipient_account_id;

-- name: ListAllFeedTagEntries :many
SELECT tag_id, entry_id, author_account_id, visibility, is_local FROM feed_tag_entries ORDER BY tag_id, entry_id;

-- name: ListAllFeedFollowEdges :many
SELECT follower_account_id, followee_account_id, show_boosts FROM feed_follow_edges
ORDER BY follower_account_id, followee_account_id;

-- name: ListAllFeedExclusions :many
SELECT viewer_account_id, target_account_id, reason, expires_at FROM feed_exclusions
ORDER BY viewer_account_id, target_account_id, reason;

-- Timelines (data-model 6) ----------------------------------------------------------------------
-- A list argument is a JSON array of strings, read with json_each: the SQLite engine of sqlc
-- expands sqlc.slice into plain ? placeholders, which SQLite numbers after the largest ?N, so a
-- slice followed by another argument would bind the wrong values.
-- before = '' starts at the newest entry. A language filter applies when all_languages is 0:
-- entries in an undetermined language ('und') always pass. A restriction or mute applies while
-- its end is NULL or after now. A domain restriction also covers the subdomains of its domain.

-- ListHomeEntries is the home timeline (FR-201) of a viewer: the posts of the viewer and of the
-- accounts the viewer follows, their boosts unless the follow hides them, and the viewer's own
-- boosts, within the read window (id >= window_floor).
-- name: ListHomeEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq, e.is_reply
FROM feed_entries e
WHERE e.author_account_id IN (
    SELECT f.followee_account_id FROM feed_follow_edges f WHERE f.follower_account_id = sqlc.arg(viewer)
    UNION ALL SELECT CAST(sqlc.arg(viewer) AS TEXT))
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR e.id < CAST(sqlc.arg(before) AS TEXT))
  AND e.id >= CAST(sqlc.arg(window_floor) AS TEXT)
  AND (e.kind = 'post' OR e.author_account_id = sqlc.arg(viewer) OR EXISTS (
    SELECT 1 FROM feed_follow_edges f
    WHERE f.follower_account_id = sqlc.arg(viewer) AND f.followee_account_id = e.author_account_id AND f.show_boosts = 1))
  AND (e.visibility <> 'direct' OR e.author_account_id = sqlc.arg(viewer) OR EXISTS (
    SELECT 1 FROM feed_direct_recipients d WHERE d.recipient_account_id = sqlc.arg(viewer) AND d.entry_id = e.id))
  AND NOT EXISTS (
    SELECT 1 FROM feed_exclusions x
    WHERE x.viewer_account_id = sqlc.arg(viewer)
      AND x.target_account_id IN (e.author_account_id, e.original_author_account_id)
      AND (x.expires_at IS NULL OR x.expires_at > CAST(sqlc.arg(now) AS INTEGER)))
  AND NOT EXISTS (
    SELECT 1 FROM feed_author_restrictions r
    WHERE r.account_id IN (e.author_account_id, e.original_author_account_id) AND r.restriction = 'profile_view'
      AND (r.effective_until IS NULL OR r.effective_until > CAST(sqlc.arg(now) AS INTEGER)))
  AND NOT EXISTS (
    SELECT 1 FROM feed_domain_restrictions dr
    WHERE dr.restriction = 'profile_view'
      AND (dr.effective_until IS NULL OR dr.effective_until > CAST(sqlc.arg(now) AS INTEGER))
      AND (dr.domain IN (e.author_domain, e.original_author_domain)
        OR substr(e.author_domain, -(length(dr.domain) + 1)) = '.' || dr.domain
        OR substr(e.original_author_domain, -(length(dr.domain) + 1)) = '.' || dr.domain))
  AND (CAST(sqlc.arg(all_languages) AS INTEGER) = 1 OR e.language = 'und' OR e.language IN (SELECT value FROM json_each(CAST(sqlc.arg(languages) AS TEXT))))
ORDER BY e.id DESC
LIMIT sqlc.arg(page_limit);

-- ListLocalEntries is the local timeline (FR-202): the public and local posts of local accounts
-- (public only when include_local is 0, for visitors that are not logged in), except those of
-- authors kept from discovery and those the viewer ('' for none) blocks, is blocked by, or mutes.
-- name: ListLocalEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq, e.is_reply
FROM feed_entries e
WHERE e.is_local = 1 AND e.kind = 'post' AND e.visibility IN ('public', 'local')
  AND (CAST(sqlc.arg(include_local) AS INTEGER) = 1 OR e.visibility = 'public')
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR e.id < CAST(sqlc.arg(before) AS TEXT))
  AND NOT EXISTS (
    SELECT 1 FROM feed_author_restrictions r
    WHERE r.account_id = e.author_account_id
      AND r.restriction IN ('public_discovery', 'visibility_ceiling_private', 'profile_view')
      AND (r.effective_until IS NULL OR r.effective_until > CAST(sqlc.arg(now) AS INTEGER)))
  AND NOT EXISTS (
    SELECT 1 FROM feed_exclusions x
    WHERE x.viewer_account_id = sqlc.arg(viewer) AND x.target_account_id = e.author_account_id
      AND (x.expires_at IS NULL OR x.expires_at > CAST(sqlc.arg(now) AS INTEGER)))
  AND (CAST(sqlc.arg(all_languages) AS INTEGER) = 1 OR e.language = 'und' OR e.language IN (SELECT value FROM json_each(CAST(sqlc.arg(languages) AS TEXT))))
ORDER BY e.id DESC
LIMIT sqlc.arg(page_limit);

-- ListTagEntries is the timeline of a hashtag (FR-203): the public and local posts with the tag
-- (public only when include_local is 0), with the exclusions of ListLocalEntries and of the
-- author's domain.
-- name: ListTagEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq, e.is_reply
FROM feed_tag_entries t
JOIN feed_entries e ON e.id = t.entry_id
WHERE t.tag_id = sqlc.arg(tag_id) AND t.visibility IN ('public', 'local')
  AND (CAST(sqlc.arg(include_local) AS INTEGER) = 1 OR t.visibility = 'public')
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR t.entry_id < CAST(sqlc.arg(before) AS TEXT))
  AND NOT EXISTS (
    SELECT 1 FROM feed_author_restrictions r
    WHERE r.account_id = t.author_account_id
      AND r.restriction IN ('public_discovery', 'visibility_ceiling_private', 'profile_view')
      AND (r.effective_until IS NULL OR r.effective_until > CAST(sqlc.arg(now) AS INTEGER)))
  AND NOT EXISTS (
    SELECT 1 FROM feed_domain_restrictions dr
    WHERE dr.restriction IN ('public_discovery', 'visibility_ceiling_private', 'profile_view')
      AND (dr.effective_until IS NULL OR dr.effective_until > CAST(sqlc.arg(now) AS INTEGER))
      AND (dr.domain = e.author_domain OR substr(e.author_domain, -(length(dr.domain) + 1)) = '.' || dr.domain))
  AND NOT EXISTS (
    SELECT 1 FROM feed_exclusions x
    WHERE x.viewer_account_id = sqlc.arg(viewer) AND x.target_account_id = t.author_account_id
      AND (x.expires_at IS NULL OR x.expires_at > CAST(sqlc.arg(now) AS INTEGER)))
  AND (CAST(sqlc.arg(all_languages) AS INTEGER) = 1 OR e.language = 'und' OR e.language IN (SELECT value FROM json_each(CAST(sqlc.arg(languages) AS TEXT))))
ORDER BY t.entry_id DESC
LIMIT sqlc.arg(page_limit);

-- ListProfileEntries is the profile timeline (FR-204) of an author: the author's posts and
-- boosts of each visibility whose include_* flag is 1, and, when include_direct is 1, the
-- direct posts addressed to the viewer (all of them when the viewer is the author). Boosts of
-- statuses whose author the viewer excludes, or whose author or domain is hidden from
-- profiles, are left out. exclude_replies leaves out the posts that reply to a status, and
-- exclude_boosts the boosts.
-- name: ListProfileEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq, e.is_reply
FROM feed_entries e
WHERE e.author_account_id = sqlc.arg(author)
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR e.id < CAST(sqlc.arg(before) AS TEXT))
  AND ((e.visibility = 'public' AND CAST(sqlc.arg(include_public) AS INTEGER) = 1)
    OR (e.visibility = 'local' AND CAST(sqlc.arg(include_local) AS INTEGER) = 1)
    OR (e.visibility = 'unlisted' AND CAST(sqlc.arg(include_unlisted) AS INTEGER) = 1)
    OR (e.visibility = 'private' AND CAST(sqlc.arg(include_private) AS INTEGER) = 1)
    OR (CAST(sqlc.arg(include_direct) AS INTEGER) = 1
    AND e.visibility = 'direct' AND (e.author_account_id = sqlc.arg(viewer) OR EXISTS (
      SELECT 1 FROM feed_direct_recipients d WHERE d.recipient_account_id = sqlc.arg(viewer) AND d.entry_id = e.id))))
  AND (e.original_author_account_id = e.author_account_id OR (
    NOT EXISTS (
      SELECT 1 FROM feed_exclusions x
      WHERE x.viewer_account_id = sqlc.arg(viewer) AND x.target_account_id = e.original_author_account_id
        AND (x.expires_at IS NULL OR x.expires_at > CAST(sqlc.arg(now) AS INTEGER)))
    AND NOT EXISTS (
      SELECT 1 FROM feed_author_restrictions r
      WHERE r.account_id = e.original_author_account_id AND r.restriction = 'profile_view'
        AND (r.effective_until IS NULL OR r.effective_until > CAST(sqlc.arg(now) AS INTEGER)))
    AND NOT EXISTS (
      SELECT 1 FROM feed_domain_restrictions dr
      WHERE dr.restriction = 'profile_view'
        AND (dr.effective_until IS NULL OR dr.effective_until > CAST(sqlc.arg(now) AS INTEGER))
        AND (dr.domain = e.original_author_domain
          OR substr(e.original_author_domain, -(length(dr.domain) + 1)) = '.' || dr.domain))))
  AND (CAST(sqlc.arg(all_languages) AS INTEGER) = 1 OR e.language = 'und' OR e.language IN (SELECT value FROM json_each(CAST(sqlc.arg(languages) AS TEXT))))
  AND (CAST(sqlc.arg(exclude_replies) AS INTEGER) = 0 OR e.is_reply = 0)
  AND (CAST(sqlc.arg(exclude_boosts) AS INTEGER) = 0 OR e.kind = 'post')
ORDER BY e.id DESC
LIMIT sqlc.arg(page_limit);

-- Relation facts of a page, for re-applying CanView ---------------------------------------------

-- name: ListFeedFollowedAmong :many
SELECT followee_account_id FROM feed_follow_edges
WHERE follower_account_id = sqlc.arg(viewer) AND followee_account_id IN (SELECT value FROM json_each(CAST(sqlc.arg(account_ids) AS TEXT)));

-- name: ListFeedExclusionsAmong :many
SELECT target_account_id, reason FROM feed_exclusions
WHERE viewer_account_id = sqlc.arg(viewer) AND (expires_at IS NULL OR expires_at > CAST(sqlc.arg(now) AS INTEGER))
  AND target_account_id IN (SELECT value FROM json_each(CAST(sqlc.arg(account_ids) AS TEXT)));

-- name: ListFeedAuthorRestrictionsAmong :many
SELECT account_id, restriction FROM feed_author_restrictions
WHERE (effective_until IS NULL OR effective_until > CAST(sqlc.arg(now) AS INTEGER))
  AND account_id IN (SELECT value FROM json_each(CAST(sqlc.arg(account_ids) AS TEXT)));

-- name: ListFeedDomainRestrictionsAmong :many
SELECT domain, restriction FROM feed_domain_restrictions
WHERE (effective_until IS NULL OR effective_until > CAST(sqlc.arg(now) AS INTEGER))
  AND domain IN (SELECT value FROM json_each(CAST(sqlc.arg(domains) AS TEXT)));

-- name: ListFeedDirectEntriesAmong :many
SELECT entry_id FROM feed_direct_recipients
WHERE recipient_account_id = sqlc.arg(viewer) AND entry_id IN (SELECT value FROM json_each(CAST(sqlc.arg(entry_ids) AS TEXT)));
