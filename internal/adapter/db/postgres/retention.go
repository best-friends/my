// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"time"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

var (
	_ appcontent.RetentionRepository    = (*StatusRepository)(nil)
	_ appfederation.PurgeRepository     = (*FederationRepository)(nil)
	_ appaccount.ProfilePruneRepository = (*AccountRepository)(nil)
)

// ListRemoteStatusesToPurge implements content.RetentionRepository.
func (r *StatusRepository) ListRemoteStatusesToPurge(ctx context.Context, tx uow.Tx, pq appcontent.PurgeQuery) ([]content.Status, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	n, err := toInt32(pq.Limit, "batch limit")
	if err != nil {
		return nil, err
	}
	rows, err := q.ListRemoteStatusesToPurge(ctx, postgresgen.ListRemoteStatusesToPurgeParams{
		After:              pq.After,
		IDBefore:           purgeIDBound(pq),
		AnyBefore:          pq.Before,
		UnreferencedBefore: pq.UnreferencedBefore,
		BatchLimit:         n,
	})
	if err != nil {
		return nil, wrap(err, "list remote statuses to purge")
	}
	out := make([]content.Status, 0, len(rows))
	for _, row := range rows {
		out = append(out, statusFrom(row))
	}
	return out, nil
}

// purgeIDBound is the UUIDv7 floor just past the later of the query's times: the ID of a remote
// status is stamped with its publication time, so no status published before them has a greater
// ID, and the scan of the primary key stops there. A zero Before counts as no time.
func purgeIDBound(pq appcontent.PurgeQuery) string {
	t := pq.UnreferencedBefore
	if pq.Before.After(t) {
		t = pq.Before
	}
	return kernel.UUIDv7Floor(t.Add(time.Millisecond))
}

// PurgeStatuses implements content.RetentionRepository.
func (r *StatusRepository) PurgeStatuses(ctx context.Context, tx uow.Tx, statusIDs []string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	ids := statusIDs
	// Children before their parents: the revisions' rows, the polls', then the boosts and the
	// statuses.
	steps := []func(context.Context, []string) error{
		q.PurgeStatusRevisionBodies, q.PurgeStatusRevisionSources, q.PurgeStatusRevisionAttachments,
		q.PurgeStatusRevisionMentions, q.PurgeStatusRevisionTags, q.PurgeStatusRevisionEmojis,
		q.PurgeRemoteStatusRevisions, q.PurgePollTallyObservationCounts, q.PurgePollTallyObservations,
		q.PurgePollVotes, q.PurgePollTallies, q.PurgePollOptions, q.PurgePolls, q.PurgeStatusRevisions,
		q.PurgeRemoteBoostRetractions, q.PurgeRemoteBoosts, q.PurgeStatuses,
	}
	for _, step := range steps {
		if err := step(ctx, ids); err != nil {
			return wrap(err, "purge statuses")
		}
	}
	return nil
}

// StatusHasNotifications implements notification.Repository.
func (r *NotificationRepository) StatusHasNotifications(ctx context.Context, tx uow.Tx, statusID string) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	ok, err := q.StatusHasNotifications(ctx, &statusID)
	return ok, wrap(err, "find notifications of a status")
}

// DeleteReceivedDocumentsBefore implements federation.PurgeRepository.
func (r *FederationRepository) DeleteReceivedDocumentsBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteReceivedActivityDocumentsBefore(ctx, postgresgen.DeleteReceivedActivityDocumentsBeforeParams{
		ReceivedBefore: t, BatchLimit: batch,
	})
	return n, wrap(err, "delete received documents")
}

// DeleteReceivedActivitiesBefore implements federation.PurgeRepository.
func (r *FederationRepository) DeleteReceivedActivitiesBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteReceivedActivitiesBefore(ctx, postgresgen.DeleteReceivedActivitiesBeforeParams{
		ReceivedBefore: t, BatchLimit: batch,
	})
	return n, wrap(err, "delete received activities")
}

// DeleteRemoteFollowTombstonesBefore implements federation.PurgeRepository.
func (r *FederationRepository) DeleteRemoteFollowTombstonesBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteRemoteFollowTombstonesBefore(ctx, postgresgen.DeleteRemoteFollowTombstonesBeforeParams{
		ReceivedBefore: t, BatchLimit: batch,
	})
	return n, wrap(err, "delete remote follow tombstones")
}

// DeleteRemoteObjectTombstonesBefore implements federation.PurgeRepository.
func (r *FederationRepository) DeleteRemoteObjectTombstonesBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteRemoteObjectTombstonesBefore(ctx, postgresgen.DeleteRemoteObjectTombstonesBeforeParams{
		ReceivedBefore: t, BatchLimit: batch,
	})
	return n, wrap(err, "delete remote object tombstones")
}

// DeleteOutboundDocumentsBefore implements federation.PurgeRepository.
func (r *FederationRepository) DeleteOutboundDocumentsBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteOutboundActivityDocumentsBefore(ctx, postgresgen.DeleteOutboundActivityDocumentsBeforeParams{
		IDBefore: kernel.UUIDv7Floor(t.Add(time.Millisecond)), BuiltBefore: t, BatchLimit: batch,
	})
	return n, wrap(err, "delete outbound documents")
}

// DeleteDeliveryAttemptsBefore implements federation.PurgeRepository.
func (r *FederationRepository) DeleteDeliveryAttemptsBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteDeliveryAttemptsBefore(ctx, postgresgen.DeleteDeliveryAttemptsBeforeParams{
		IDBefore: kernel.UUIDv7Floor(t.Add(time.Millisecond)), AttemptedBefore: t, BatchLimit: batch,
	})
	return n, wrap(err, "delete delivery attempts")
}

// ListRemoteAccountsWithPastProfiles implements account.ProfilePruneRepository.
func (r *AccountRepository) ListRemoteAccountsWithPastProfiles(ctx context.Context, after string, limit int) ([]string, error) {
	batch, err := toInt32(limit, "batch limit")
	if err != nil {
		return nil, err
	}
	ids, err := r.db.reader().ListRemoteAccountsWithPastProfiles(ctx, postgresgen.ListRemoteAccountsWithPastProfilesParams{
		After: after, BatchLimit: batch,
	})
	return ids, wrap(err, "list remote accounts with past profiles")
}

// PrunePastProfiles implements account.ProfilePruneRepository.
func (r *AccountRepository) PrunePastProfiles(ctx context.Context, tx uow.Tx, accountID string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	for _, step := range []func(context.Context, string) error{
		q.PrunePastProfileFields, q.PrunePastProfileRevisionEmojis, q.PrunePastRemoteProfileRevisions, q.PrunePastProfileRevisions, q.PrunePastRemoteSnapshots,
	} {
		if err := step(ctx, accountID); err != nil {
			return wrap(err, "prune past profiles")
		}
	}
	return nil
}
