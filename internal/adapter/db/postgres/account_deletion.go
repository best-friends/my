// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"time"

	"github.com/mycroblog/my/internal/adapter/db/postgres/postgresgen"
	"github.com/mycroblog/my/internal/domain/account"
)

// ListDueDeletions implements account.Repository.
func (r *AccountRepository) ListDueDeletions(ctx context.Context, now time.Time, limit int) ([]account.LifecycleState, error) {
	lim, err := toInt32(limit, "limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.db.reader().ListDueDeletions(ctx, postgresgen.ListDueDeletionsParams{Now: now, Lim: lim})
	if err != nil {
		return nil, wrap(err, "list due deletions")
	}
	out := make([]account.LifecycleState, len(rows))
	for i, row := range rows {
		out[i] = lifecycleState(row)
	}
	return out, nil
}
