// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

var errBoostEdgeExists = errors.New("content: the account already boosts the status")

// RegisterInteractionProjectors registers the synchronous projectors of the interactions with
// statuses (data-model §9). Register them after RegisterProjectors, which creates the counters
// of a status.
//   - boost_edges holds each boost that is not retracted, of a status that is not deleted;
//   - status_counters.boosts_count counts those boosts and favourites_count the favourites of
//     the status. Both change only when an event says a row was actually written.
//
// The rebuild queries state the same rules in SQL (InteractionRepository.RebuildBoostEdges and
// Repository.RebuildCounters); the property test in testkit.InteractionRepositoryContract
// checks that both agree.
func RegisterInteractionProjectors(r *uow.Projectors, repo InteractionRepository) {
	r.Register(EventBoosted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		b := e.(Boosted).Boost
		inserted, err := repo.InsertBoostEdge(ctx, tx, content.EdgeOf(b))
		if err != nil {
			return err
		}
		if !inserted {
			// A concurrent boost of the same pair won; a new attempt finds its edge.
			return uow.MarkRetryable(errBoostEdgeExists)
		}
		return repo.AddBoosts(ctx, tx, b.StatusID, 1)
	})
	r.Register(EventBoostRetracted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		b := e.(BoostRetracted).Boost
		deleted, err := repo.DeleteBoostEdge(ctx, tx, content.EdgeOf(b))
		if err != nil || !deleted {
			return err
		}
		return repo.AddBoosts(ctx, tx, b.StatusID, -1)
	})
	r.Register(EventFavourited, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		return repo.AddFavourites(ctx, tx, e.(Favourited).Favourite.StatusID, 1)
	})
	r.Register(EventUnfavourited, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		return repo.AddFavourites(ctx, tx, e.(Unfavourited).Favourite.StatusID, -1)
	})
	// A deleted status leaves every status projection; its counters go with RegisterProjectors.
	r.Register(EventStatusDeleted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		return repo.DeleteStatusBoostEdges(ctx, tx, e.(StatusDeleted).Status.ID)
	})
}

// RebuildInteractionProjections empties boost_edges and derives it again from the boosts and
// their retractions (data-model §9). The boosts and favourites counts are rebuilt with the
// other status counters by RebuildProjections.
func RebuildInteractionProjections(ctx context.Context, u uow.UnitOfWork, repo InteractionRepository) error {
	if err := u.Tx(ctx, func(tx uow.Tx) error { return repo.RebuildBoostEdges(ctx, tx) }); err != nil {
		return fmt.Errorf("rebuild boost_edges: %w", err)
	}
	return nil
}
