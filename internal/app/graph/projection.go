// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/internal/domain/graph"
)

var (
	// errEdgeExists reports that a concurrent transaction created the pair's edge first. It is
	// retryable: the next attempt sees that edge and treats the follow as already made.
	errEdgeExists = errors.New("graph: the pair already has a live follow")
	// errBlockEdgeExists is errEdgeExists for block_edges.
	errBlockEdgeExists = errors.New("graph: the pair already has a block in force")
)

// RegisterProjectors registers the graph module's synchronous projectors (data-model §9):
// follow_edges and follow_counters from follows and their events, and block_edges from blocks
// and their retractions. Each projector applies the same rule as RebuildProjections, so
// updating incrementally and rebuilding agree.
func RegisterProjectors(r *uow.Projectors, repo Repository) {
	r.Register(EventFollowed, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(Followed)
		if err := insertEdge(ctx, tx, repo, graph.ActiveEdge(ev.Follow, ev.Acceptance)); err != nil {
			return err
		}
		return countFollow(ctx, tx, repo, ev.Follow, 1)
	})
	r.Register(EventFollowRequested, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		return insertEdge(ctx, tx, repo, graph.PendingEdge(e.(FollowRequested).Follow))
	})
	r.Register(EventFollowAccepted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(FollowAccepted)
		if err := repo.UpdateEdge(ctx, tx, graph.ActiveEdge(ev.Follow, ev.Acceptance)); err != nil {
			return err
		}
		return countFollow(ctx, tx, repo, ev.Follow, 1)
	})
	r.Register(EventFollowRejected, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		f := e.(FollowRejected).Follow
		return repo.DeleteEdge(ctx, tx, f.FollowerAccountID, f.FolloweeAccountID, f.ID)
	})
	r.Register(EventFollowTerminated, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(FollowTerminated)
		f := ev.Follow
		if err := repo.DeleteEdge(ctx, tx, f.FollowerAccountID, f.FolloweeAccountID, f.ID); err != nil {
			return err
		}
		if ev.PreviousState != graph.StateActive {
			return nil
		}
		return countFollow(ctx, tx, repo, f, -1)
	})
	r.Register(EventBlocked, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		inserted, err := repo.InsertBlockEdge(ctx, tx, graph.EdgeOf(e.(Blocked).Block))
		if err != nil {
			return err
		}
		if !inserted {
			return uow.MarkRetryable(errBlockEdgeExists)
		}
		return nil
	})
	r.Register(EventUnblocked, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		b := e.(Unblocked).Block
		return repo.DeleteBlockEdge(ctx, tx, b.BlockerAccountID, b.BlockedAccountID, b.ID)
	})
}

func insertEdge(ctx context.Context, tx uow.Tx, repo Repository, e graph.Edge) error {
	inserted, err := repo.InsertEdge(ctx, tx, e)
	if err != nil {
		return err
	}
	if !inserted {
		return uow.MarkRetryable(errEdgeExists)
	}
	return nil
}

// countFollow adds delta to the follower's following count and the followee's followers count.
func countFollow(ctx context.Context, tx uow.Tx, repo Repository, f graph.Follow, delta int64) error {
	if err := repo.AddCounters(ctx, tx, f.FollowerAccountID, 0, delta); err != nil {
		return err
	}
	return repo.AddCounters(ctx, tx, f.FolloweeAccountID, delta, 0)
}

// RebuildProjections empties follow_edges, follow_counters, and block_edges and derives them
// again from the follows, the blocks, and their events (data-model §9), in one transaction.
func RebuildProjections(ctx context.Context, u uow.UnitOfWork, repo Repository) error {
	err := u.Tx(ctx, func(tx uow.Tx) error {
		if err := repo.DeleteAllEdges(ctx, tx); err != nil {
			return err
		}
		if err := repo.DeleteAllCounters(ctx, tx); err != nil {
			return err
		}
		histories, err := repo.ListHistories(ctx, tx)
		if err != nil {
			return err
		}
		var edges []graph.Edge
		for _, h := range histories {
			e, ok := h.Edge()
			if !ok {
				continue
			}
			inserted, err := repo.InsertEdge(ctx, tx, e)
			if err != nil {
				return err
			}
			if !inserted {
				return fmt.Errorf("graph: follows %s and another follow are both live", e.FollowID)
			}
			edges = append(edges, e)
		}
		counters := graph.CountEdges(edges)
		for _, id := range slices.Sorted(maps.Keys(counters)) {
			if err := repo.UpsertCounters(ctx, tx, counters[id]); err != nil {
				return err
			}
		}
		return rebuildBlockEdges(ctx, tx, repo)
	})
	if err != nil {
		return fmt.Errorf("rebuild follow_edges, follow_counters, and block_edges: %w", err)
	}
	return nil
}

func rebuildBlockEdges(ctx context.Context, tx uow.Tx, repo Repository) error {
	if err := repo.DeleteAllBlockEdges(ctx, tx); err != nil {
		return err
	}
	histories, err := repo.ListBlockHistories(ctx, tx)
	if err != nil {
		return err
	}
	for _, h := range histories {
		e, ok := h.Edge()
		if !ok {
			continue
		}
		inserted, err := repo.InsertBlockEdge(ctx, tx, e)
		if err != nil {
			return err
		}
		if !inserted {
			return fmt.Errorf("graph: blocks %s and another block are both in force", e.BlockID)
		}
	}
	return nil
}
