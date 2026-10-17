// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
)

// UpsertRemoteHead implements account.Repository.
func (r *AccountRepository) UpsertRemoteHead(ctx context.Context, tx uow.Tx, h account.RemoteHead) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	failures, err := toInt32(h.ConsecutiveFetchFailures, "consecutive fetch failures")
	if err != nil {
		return err
	}
	err = q.UpsertRemoteAccountHead(ctx, postgresgen.UpsertRemoteAccountHeadParams{
		AccountID: h.AccountID, SnapshotID: h.SnapshotID, InboxUrl: h.InboxURL,
		SharedInboxUrl: toNullString(h.SharedInboxURL), LastFetchedAt: h.LastFetchedAt,
		ConsecutiveFetchFailures: failures,
	})
	return wrap(err, "upsert remote account head")
}

// FindRemoteHead implements account.Repository.
func (r *AccountRepository) FindRemoteHead(ctx context.Context, accountID string) (account.RemoteHead, error) {
	row, err := r.db.reader().GetRemoteAccountHead(ctx, accountID)
	if err != nil {
		return account.RemoteHead{}, notFound(err, appaccount.ErrNotFound, "remote head of "+accountID)
	}
	return account.RemoteHead{
		AccountID: row.AccountID, SnapshotID: row.SnapshotID, InboxURL: row.InboxUrl,
		SharedInboxURL: fromNullString(row.SharedInboxUrl), LastFetchedAt: utc(row.LastFetchedAt),
		ConsecutiveFetchFailures: int(row.ConsecutiveFetchFailures),
	}, nil
}
