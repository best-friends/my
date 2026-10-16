-- SPDX-License-Identifier: AGPL-3.0-only
-- timeline: feeds (data-model 4.6, 6). Every query reads only the feed_* tables.

-- Projector writes ------------------------------------------------------------------------------

-- name: InsertFeedEntry :execrows
INSERT INTO feed_entries (
  id, kind, status_id, author_account_id, author_domain, original_author_account_id,
  original_author_domain, visibility, is_local, language, revision_seq
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (id) DO NOTHING;

-- name: GetFeedEntry :one
SELECT id, kind, status_id, author_account_id, author_domain, original_author_account_id,
  original_author_domain, visibility, is_local, language, revision_seq
FROM feed_entries WHERE id = $1;

-- UpdateFeedEntriesRevision moves the entries of a status (the post and its boosts) to a
-- newer revision; an older or repeated revision changes nothing.
-- name: UpdateFeedEntriesRevision :execrows
UPDATE feed_entries SET language = sqlc.arg(language), revision_seq = sqlc.arg(revision_seq)
WHERE status_id = sqlc.arg(status_id) AND revision_seq < sqlc.arg(revision_seq);

-- name: DeleteFeedEntry :exec
DELETE FROM feed_entries WHERE id = $1;

-- name: DeleteFeedEntriesOfStatus :exec
DELETE FROM feed_entries WHERE status_id = $1;

-- name: InsertFeedDirectRecipient :exec
INSERT INTO feed_direct_recipients (entry_id, recipient_account_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteFeedDirectRecipients :exec
DELETE FROM feed_direct_recipients WHERE entry_id = $1;

-- name: InsertFeedTagEntry :exec
INSERT INTO feed_tag_entries (tag_id, entry_id, author_account_id, visibility, is_local) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;

-- name: DeleteFeedTagEntries :exec
DELETE FROM feed_tag_entries WHERE entry_id = $1;

-- name: UpsertFeedFollowEdge :exec
INSERT INTO feed_follow_edges (follower_account_id, followee_account_id, show_boosts) VALUES ($1, $2, $3)
ON CONFLICT (follower_account_id, followee_account_id) DO UPDATE SET show_boosts = excluded.show_boosts;

-- name: DeleteFeedFollowEdge :exec
DELETE FROM feed_follow_edges WHERE follower_account_id = $1 AND followee_account_id = $2;

-- name: UpsertFeedExclusion :exec
INSERT INTO feed_exclusions (viewer_account_id, target_account_id, reason, expires_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (viewer_account_id, target_account_id, reason) DO UPDATE SET expires_at = excluded.expires_at;

-- name: DeleteFeedExclusion :exec
DELETE FROM feed_exclusions WHERE viewer_account_id = $1 AND target_account_id = $2 AND reason = $3;

-- name: UpsertFeedAuthorRestriction :exec
INSERT INTO feed_author_restrictions (account_id, restriction, effective_until) VALUES ($1, $2, $3)
ON CONFLICT (account_id, restriction) DO UPDATE SET effective_until = excluded.effective_until;

-- name: DeleteFeedAuthorRestriction :exec
DELETE FROM feed_author_restrictions WHERE account_id = $1 AND restriction = $2;

-- name: UpsertFeedDomainRestriction :exec
INSERT INTO feed_domain_restrictions (domain, restriction, effective_until) VALUES ($1, $2, $3)
ON CONFLICT (domain, restriction) DO UPDATE SET effective_until = excluded.effective_until;

-- name: DeleteFeedDomainRestriction :exec
DELETE FROM feed_domain_restrictions WHERE domain = $1 AND restriction = $2;

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
  original_author_domain, visibility, is_local, language, revision_seq
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
-- before = '' starts at the newest entry. A language filter applies when all_languages is 0:
-- entries in an undetermined language ('und') always pass. A restriction or mute applies while
-- its end is NULL or after now. A domain restriction also covers the subdomains of its domain.

-- ListHomeEntries is the home timeline (FR-201) of a viewer: the posts of the viewer and of the
-- accounts the viewer follows, their boosts unless the follow hides them, and the viewer's own
-- boosts, within the read window (id >= window_floor).
-- name: ListHomeEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq
FROM feed_entries e
WHERE e.author_account_id IN (
    SELECT f.followee_account_id FROM feed_follow_edges f WHERE f.follower_account_id = sqlc.arg(viewer)
    UNION ALL SELECT sqlc.arg(viewer)::text)
  AND (sqlc.arg(before)::text = '' OR e.id < sqlc.arg(before)::text)
  AND e.id >= sqlc.arg(window_floor)::text
  AND (e.kind = 'post' OR e.author_account_id = sqlc.arg(viewer) OR EXISTS (
    SELECT 1 FROM feed_follow_edges f
    WHERE f.follower_account_id = sqlc.arg(viewer) AND f.followee_account_id = e.author_account_id AND f.show_boosts))
  AND (e.visibility <> 'direct' OR e.author_account_id = sqlc.arg(viewer) OR EXISTS (
    SELECT 1 FROM feed_direct_recipients d WHERE d.recipient_account_id = sqlc.arg(viewer) AND d.entry_id = e.id))
  AND NOT EXISTS (
    SELECT 1 FROM feed_exclusions x
    WHERE x.viewer_account_id = sqlc.arg(viewer)
      AND x.target_account_id IN (e.author_account_id, e.original_author_account_id)
      AND (x.expires_at IS NULL OR x.expires_at > sqlc.arg(now)::timestamptz))
  AND NOT EXISTS (
    SELECT 1 FROM feed_author_restrictions r
    WHERE r.account_id IN (e.author_account_id, e.original_author_account_id) AND r.restriction = 'profile_view'
      AND (r.effective_until IS NULL OR r.effective_until > sqlc.arg(now)::timestamptz))
  AND NOT EXISTS (
    SELECT 1 FROM feed_domain_restrictions dr
    WHERE dr.restriction = 'profile_view'
      AND (dr.effective_until IS NULL OR dr.effective_until > sqlc.arg(now)::timestamptz)
      AND (dr.domain IN (e.author_domain, e.original_author_domain)
        OR right(e.author_domain, length(dr.domain) + 1) = '.' || dr.domain
        OR right(e.original_author_domain, length(dr.domain) + 1) = '.' || dr.domain))
  AND (sqlc.arg(all_languages)::boolean OR e.language = 'und' OR e.language = ANY(sqlc.arg(languages)::text[]))
ORDER BY e.id DESC
LIMIT sqlc.arg(page_limit);

-- ListLocalEntries is the local timeline (FR-202): the public and local posts of local accounts
-- (public only when include_local is 0, for visitors that are not logged in), except those of
-- authors kept from discovery and those the viewer ('' for none) blocks, is blocked by, or mutes.
-- name: ListLocalEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq
FROM feed_entries e
WHERE e.is_local AND e.kind = 'post' AND e.visibility IN ('public', 'local')
  AND (sqlc.arg(include_local)::boolean OR e.visibility = 'public')
  AND (sqlc.arg(before)::text = '' OR e.id < sqlc.arg(before)::text)
  AND NOT EXISTS (
    SELECT 1 FROM feed_author_restrictions r
    WHERE r.account_id = e.author_account_id
      AND r.restriction IN ('public_discovery', 'visibility_ceiling_private', 'profile_view')
      AND (r.effective_until IS NULL OR r.effective_until > sqlc.arg(now)::timestamptz))
  AND NOT EXISTS (
    SELECT 1 FROM feed_exclusions x
    WHERE x.viewer_account_id = sqlc.arg(viewer) AND x.target_account_id = e.author_account_id
      AND (x.expires_at IS NULL OR x.expires_at > sqlc.arg(now)::timestamptz))
  AND (sqlc.arg(all_languages)::boolean OR e.language = 'und' OR e.language = ANY(sqlc.arg(languages)::text[]))
ORDER BY e.id DESC
LIMIT sqlc.arg(page_limit);

-- ListTagEntries is the timeline of a hashtag (FR-203): the public and local posts with the tag
-- (public only when include_local is 0), with the exclusions of ListLocalEntries and of the
-- author's domain.
-- name: ListTagEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq
FROM feed_tag_entries t
JOIN feed_entries e ON e.id = t.entry_id
WHERE t.tag_id = sqlc.arg(tag_id) AND t.visibility IN ('public', 'local')
  AND (sqlc.arg(include_local)::boolean OR t.visibility = 'public')
  AND (sqlc.arg(before)::text = '' OR t.entry_id < sqlc.arg(before)::text)
  AND NOT EXISTS (
    SELECT 1 FROM feed_author_restrictions r
    WHERE r.account_id = t.author_account_id
      AND r.restriction IN ('public_discovery', 'visibility_ceiling_private', 'profile_view')
      AND (r.effective_until IS NULL OR r.effective_until > sqlc.arg(now)::timestamptz))
  AND NOT EXISTS (
    SELECT 1 FROM feed_domain_restrictions dr
    WHERE dr.restriction IN ('public_discovery', 'visibility_ceiling_private', 'profile_view')
      AND (dr.effective_until IS NULL OR dr.effective_until > sqlc.arg(now)::timestamptz)
      AND (dr.domain = e.author_domain OR right(e.author_domain, length(dr.domain) + 1) = '.' || dr.domain))
  AND NOT EXISTS (
    SELECT 1 FROM feed_exclusions x
    WHERE x.viewer_account_id = sqlc.arg(viewer) AND x.target_account_id = t.author_account_id
      AND (x.expires_at IS NULL OR x.expires_at > sqlc.arg(now)::timestamptz))
  AND (sqlc.arg(all_languages)::boolean OR e.language = 'und' OR e.language = ANY(sqlc.arg(languages)::text[]))
ORDER BY t.entry_id DESC
LIMIT sqlc.arg(page_limit);

-- ListProfileEntries is the profile timeline (FR-204) of an author: the author's posts and
-- boosts of each visibility whose include_* flag is 1, and, when include_direct is 1, the
-- direct posts addressed to the viewer (all of them when the viewer is the author). Boosts of
-- statuses whose author the viewer excludes, or whose author or domain is hidden from
-- profiles, are left out.
-- name: ListProfileEntries :many
SELECT e.id, e.kind, e.status_id, e.author_account_id, e.author_domain, e.original_author_account_id,
  e.original_author_domain, e.visibility, e.is_local, e.language, e.revision_seq
FROM feed_entries e
WHERE e.author_account_id = sqlc.arg(author)
  AND (sqlc.arg(before)::text = '' OR e.id < sqlc.arg(before)::text)
  AND ((e.visibility = 'public' AND sqlc.arg(include_public)::boolean)
    OR (e.visibility = 'local' AND sqlc.arg(include_local)::boolean)
    OR (e.visibility = 'unlisted' AND sqlc.arg(include_unlisted)::boolean)
    OR (e.visibility = 'private' AND sqlc.arg(include_private)::boolean)
    OR (sqlc.arg(include_direct)::boolean
    AND e.visibility = 'direct' AND (e.author_account_id = sqlc.arg(viewer) OR EXISTS (
      SELECT 1 FROM feed_direct_recipients d WHERE d.recipient_account_id = sqlc.arg(viewer) AND d.entry_id = e.id))))
  AND (e.original_author_account_id = e.author_account_id OR (
    NOT EXISTS (
      SELECT 1 FROM feed_exclusions x
      WHERE x.viewer_account_id = sqlc.arg(viewer) AND x.target_account_id = e.original_author_account_id
        AND (x.expires_at IS NULL OR x.expires_at > sqlc.arg(now)::timestamptz))
    AND NOT EXISTS (
      SELECT 1 FROM feed_author_restrictions r
      WHERE r.account_id = e.original_author_account_id AND r.restriction = 'profile_view'
        AND (r.effective_until IS NULL OR r.effective_until > sqlc.arg(now)::timestamptz))
    AND NOT EXISTS (
      SELECT 1 FROM feed_domain_restrictions dr
      WHERE dr.restriction = 'profile_view'
        AND (dr.effective_until IS NULL OR dr.effective_until > sqlc.arg(now)::timestamptz)
        AND (dr.domain = e.original_author_domain
          OR right(e.original_author_domain, length(dr.domain) + 1) = '.' || dr.domain))))
  AND (sqlc.arg(all_languages)::boolean OR e.language = 'und' OR e.language = ANY(sqlc.arg(languages)::text[]))
ORDER BY e.id DESC
LIMIT sqlc.arg(page_limit);

-- Relation facts of a page, for re-applying CanView ---------------------------------------------

-- name: ListFeedFollowedAmong :many
SELECT followee_account_id FROM feed_follow_edges
WHERE follower_account_id = sqlc.arg(viewer) AND followee_account_id = ANY(sqlc.arg(account_ids)::text[]);

-- name: ListFeedExclusionsAmong :many
SELECT target_account_id, reason FROM feed_exclusions
WHERE viewer_account_id = sqlc.arg(viewer) AND (expires_at IS NULL OR expires_at > sqlc.arg(now)::timestamptz)
  AND target_account_id = ANY(sqlc.arg(account_ids)::text[]);

-- name: ListFeedAuthorRestrictionsAmong :many
SELECT account_id, restriction FROM feed_author_restrictions
WHERE (effective_until IS NULL OR effective_until > sqlc.arg(now)::timestamptz)
  AND account_id = ANY(sqlc.arg(account_ids)::text[]);

-- name: ListFeedDomainRestrictionsAmong :many
SELECT domain, restriction FROM feed_domain_restrictions
WHERE (effective_until IS NULL OR effective_until > sqlc.arg(now)::timestamptz)
  AND domain = ANY(sqlc.arg(domains)::text[]);

-- name: ListFeedDirectEntriesAmong :many
SELECT entry_id FROM feed_direct_recipients
WHERE recipient_account_id = sqlc.arg(viewer) AND entry_id = ANY(sqlc.arg(entry_ids)::text[]);
