// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/sqlite/sqlitegen"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/graph"
)

// InsertResend implements graph.Repository.
func (r *GraphRepository) InsertResend(ctx context.Context, tx uow.Tx, rs graph.Resend) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertFollowResend(ctx, sqlitegen.InsertFollowResendParams{
		Uri: rs.URI, FollowID: rs.FollowID, ReceivedAt: toMillis(rs.ReceivedAt),
	})
	if err != nil {
		return false, wrap(err, "insert follow resend")
	}
	return n == 1, nil
}

// FollowByResendURI implements graph.Repository.
func (r *GraphRepository) FollowByResendURI(ctx context.Context, tx uow.Tx, uri string) (graph.Follow, error) {
	q, err := queries(tx)
	if err != nil {
		return graph.Follow{}, err
	}
	row, err := q.GetFollowByResendURI(ctx, uri)
	if err != nil {
		return graph.Follow{}, notFound(err, appgraph.ErrNotFound, "follow resent as "+uri)
	}
	return followFrom(row), nil
}
