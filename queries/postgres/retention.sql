-- SPDX-License-Identifier: AGPL-3.0-only

-- Retention and purges of federated data (ADR-0034, FR-803, data-model 4.4, 4.8, 8). Each purge
-- deletes a bounded batch per statement; the jobs repeat it in short transactions.

-- ListRemoteStatusesToPurge returns, in ID order after the cursor @after and below @id_before,
-- the remote statuses that are not deleted and were published before @any_before, or before
-- @unreferenced_before while nothing local refers to them: a live local reply, a live local
-- status in their thread, a local boost that is not retracted, a favourite, a reaction, a
-- bookmark, or a pin.
-- name: ListRemoteStatusesToPurge :many
SELECT s.id, s.uri, s.url, s.author_account_id, s.is_local, s.visibility, s.in_reply_to_uri,
  s.conversation_uri, s.published_at
FROM statuses s
WHERE s.id > sqlc.arg(after)::text AND s.id < sqlc.arg(id_before)::text
  AND NOT s.is_local
  AND NOT EXISTS (SELECT 1 FROM status_deletions d WHERE d.status_id = s.id)
  AND (s.published_at < sqlc.arg(any_before)::timestamptz OR (
    s.published_at < sqlc.arg(unreferenced_before)::timestamptz
    AND NOT EXISTS (
      SELECT 1 FROM statuses r WHERE r.in_reply_to_uri = s.uri AND r.is_local
        AND NOT EXISTS (SELECT 1 FROM status_deletions rd WHERE rd.status_id = r.id))
    AND (s.conversation_uri IS NULL OR NOT EXISTS (
      SELECT 1 FROM statuses l WHERE l.conversation_uri = s.conversation_uri AND l.is_local
        AND NOT EXISTS (SELECT 1 FROM status_deletions ld WHERE ld.status_id = l.id)))
    AND NOT EXISTS (
      SELECT 1 FROM boosts b WHERE b.status_id = s.id AND b.is_local
        AND NOT EXISTS (SELECT 1 FROM boost_retractions br WHERE br.boost_id = b.id))
    AND NOT EXISTS (SELECT 1 FROM favourites f WHERE f.status_id = s.id)
    AND NOT EXISTS (SELECT 1 FROM reactions x WHERE x.status_id = s.id)
    AND NOT EXISTS (SELECT 1 FROM bookmarks k WHERE k.status_id = s.id)
    AND NOT EXISTS (SELECT 1 FROM status_pins p WHERE p.status_id = s.id)))
ORDER BY s.id
LIMIT sqlc.arg(batch_limit)::integer;

-- The Purge* statements delete the rows of deleted statuses (@ids) that the purge removes: the
-- revisions with their children and polls, the remote boosts, and the statuses. The deletions
-- and local accounts' favourites, bookmarks, pins, and boosts stay.

-- name: PurgeStatusRevisionBodies :exec
DELETE FROM status_revision_bodies WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeStatusRevisionSources :exec
DELETE FROM status_revision_sources WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeStatusRevisionAttachments :exec
DELETE FROM status_revision_attachments WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeStatusRevisionMentions :exec
DELETE FROM status_revision_mentions WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeStatusRevisionTags :exec
DELETE FROM status_revision_tags WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeStatusRevisionEmojis :exec
DELETE FROM status_revision_emojis WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeRemoteStatusRevisions :exec
DELETE FROM remote_status_revisions WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgePollTallyObservationCounts :exec
DELETE FROM poll_tally_observation_counts WHERE observation_id IN (
  SELECT o.id FROM poll_tally_observations o
  JOIN polls p ON p.id = o.poll_id
  JOIN status_revisions r ON r.id = p.revision_id
  WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgePollTallyObservations :exec
DELETE FROM poll_tally_observations WHERE poll_id IN (
  SELECT p.id FROM polls p JOIN status_revisions r ON r.id = p.revision_id
  WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgePollTallies :exec
DELETE FROM poll_tallies WHERE poll_id IN (
  SELECT p.id FROM polls p JOIN status_revisions r ON r.id = p.revision_id
  WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgePollOptions :exec
DELETE FROM poll_options WHERE poll_id IN (
  SELECT p.id FROM polls p JOIN status_revisions r ON r.id = p.revision_id
  WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgePolls :exec
DELETE FROM polls WHERE revision_id IN (
  SELECT r.id FROM status_revisions r WHERE r.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeStatusRevisions :exec
DELETE FROM status_revisions WHERE status_id = ANY(sqlc.arg(ids)::text[]);

-- name: PurgeRemoteBoostRetractions :exec
DELETE FROM boost_retractions WHERE boost_id IN (
  SELECT b.id FROM boosts b WHERE NOT b.is_local AND b.status_id = ANY(sqlc.arg(ids)::text[]));

-- name: PurgeRemoteBoosts :exec
DELETE FROM boosts WHERE NOT is_local AND status_id = ANY(sqlc.arg(ids)::text[]);

-- name: PurgeStatuses :exec
DELETE FROM statuses WHERE id = ANY(sqlc.arg(ids)::text[]);

-- StatusHasNotifications reports whether a notification is about the status.
-- name: StatusHasNotifications :one
SELECT EXISTS (SELECT 1 FROM notifications WHERE status_id = $1);

-- ListRemoteAccountsWithPastProfiles returns, in ID order after @after, the remote accounts that
-- keep a profile revision other than the current one, or a snapshot that neither the head nor a
-- remaining profile revision points to.
-- name: ListRemoteAccountsWithPastProfiles :many
SELECT ra.account_id FROM remote_accounts ra
WHERE ra.account_id > sqlc.arg(after)::text
  AND (EXISTS (
      SELECT 1 FROM account_profile_revisions r
      JOIN account_profile_heads h ON h.account_id = r.account_id
      WHERE r.account_id = ra.account_id AND r.id <> h.revision_id)
    OR EXISTS (
      SELECT 1 FROM remote_account_snapshots s
      JOIN remote_account_heads h ON h.account_id = s.account_id
      WHERE s.account_id = ra.account_id AND s.id <> h.snapshot_id
        AND NOT EXISTS (
          SELECT 1 FROM remote_profile_revisions pr
          JOIN account_profile_heads ph ON ph.revision_id = pr.revision_id
          WHERE ph.account_id = ra.account_id AND pr.snapshot_id = s.id)))
ORDER BY ra.account_id
LIMIT sqlc.arg(batch_limit)::integer;

-- The PrunePast* statements delete the profile revisions of a remote account other than its
-- current one, with their children, and then its snapshots that neither the head nor the
-- remaining revision points to. Without a head, the comparison with the missing head is NULL and
-- nothing is deleted.

-- name: PrunePastProfileFields :exec
DELETE FROM account_profile_fields WHERE revision_id IN (
  SELECT r.id FROM account_profile_revisions r
  WHERE r.account_id = sqlc.arg(account_id)
    AND r.id <> (SELECT h.revision_id FROM account_profile_heads h WHERE h.account_id = sqlc.arg(account_id)));

-- name: PrunePastRemoteProfileRevisions :exec
DELETE FROM remote_profile_revisions WHERE revision_id IN (
  SELECT r.id FROM account_profile_revisions r
  WHERE r.account_id = sqlc.arg(account_id)
    AND r.id <> (SELECT h.revision_id FROM account_profile_heads h WHERE h.account_id = sqlc.arg(account_id)));

-- name: PrunePastProfileRevisions :exec
DELETE FROM account_profile_revisions
WHERE account_profile_revisions.account_id = sqlc.arg(account_id)
  AND account_profile_revisions.id <> (SELECT h.revision_id FROM account_profile_heads h WHERE h.account_id = sqlc.arg(account_id));

-- name: PrunePastRemoteSnapshots :exec
DELETE FROM remote_account_snapshots
WHERE remote_account_snapshots.account_id = sqlc.arg(account_id)
  AND remote_account_snapshots.id <> (SELECT h.snapshot_id FROM remote_account_heads h WHERE h.account_id = sqlc.arg(account_id))
  AND remote_account_snapshots.id NOT IN (
    SELECT pr.snapshot_id FROM remote_profile_revisions pr
    JOIN account_profile_revisions r ON r.id = pr.revision_id
    WHERE r.account_id = sqlc.arg(account_id));
