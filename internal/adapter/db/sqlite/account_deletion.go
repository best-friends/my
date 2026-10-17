// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"
	"time"

	"github.com/best-friends/my/internal/adapter/db/sqlite/sqlitegen"
	"github.com/best-friends/my/internal/domain/account"
)

// ListDueDeletions implements account.Repository.
func (r *AccountRepository) ListDueDeletions(ctx context.Context, now time.Time, limit int) ([]account.LifecycleState, error) {
	rows, err := r.db.reader().ListDueDeletions(ctx, sqlitegen.ListDueDeletionsParams{Now: toMillis(now), Lim: int64(limit)})
	if err != nil {
		return nil, wrap(err, "list due deletions")
	}
	out := make([]account.LifecycleState, len(rows))
	for i, row := range rows {
		out[i] = lifecycleState(row)
	}
	return out, nil
}
