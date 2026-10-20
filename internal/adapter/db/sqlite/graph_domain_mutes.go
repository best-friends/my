// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/sqlite/sqlitegen"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/graph"
)

// InsertDomainMute implements graph.Repository.
func (r *GraphRepository) InsertDomainMute(ctx context.Context, tx uow.Tx, m graph.DomainMute) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	rows, err := q.InsertDomainMute(ctx, sqlitegen.InsertDomainMuteParams{AccountID: m.AccountID, Domain: m.Domain, CreatedAt: toMillis(m.CreatedAt)})
	if err != nil {
		return false, wrap(err, "insert domain mute")
	}
	return rows == 1, nil
}

// DeleteDomainMute implements graph.Repository.
func (r *GraphRepository) DeleteDomainMute(ctx context.Context, tx uow.Tx, accountID, domain string) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	rows, err := q.DeleteDomainMute(ctx, sqlitegen.DeleteDomainMuteParams{AccountID: accountID, Domain: domain})
	if err != nil {
		return false, wrap(err, "delete domain mute")
	}
	return rows == 1, nil
}

// CountDomainMutes implements graph.Repository.
func (r *GraphRepository) CountDomainMutes(ctx context.Context, tx uow.Tx, accountID string) (int, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	n, err := q.CountDomainMutes(ctx, accountID)
	return int(n), wrap(err, "count domain mutes")
}

// ListDomainMutes implements graph.Repository.
func (r *GraphRepository) ListDomainMutes(ctx context.Context, accountID, after string, limit int) ([]graph.DomainMute, error) {
	rows, err := r.db.reader().ListDomainMutesPage(ctx, sqlitegen.ListDomainMutesPageParams{
		AccountID: accountID, BeforeDomain: after, PageLimit: int64(limit),
	})
	if err != nil {
		return nil, wrap(err, "list domain mutes")
	}
	return domainMutesFrom(rows), nil
}

// ListAllDomainMutes implements graph.Repository.
func (r *GraphRepository) ListAllDomainMutes(ctx context.Context, tx uow.Tx) ([]graph.DomainMute, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListAllDomainMutes(ctx)
	if err != nil {
		return nil, wrap(err, "list all domain mutes")
	}
	return domainMutesFrom(rows), nil
}

func domainMutesFrom(rows []sqlitegen.AccountDomainMute) []graph.DomainMute {
	out := make([]graph.DomainMute, len(rows))
	for i, row := range rows {
		out[i] = graph.DomainMute{AccountID: row.AccountID, Domain: row.Domain, CreatedAt: fromMillis(row.CreatedAt)}
	}
	return out
}
