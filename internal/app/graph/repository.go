// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"context"
	"errors"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/graph"
)

var (
	// ErrNotFound reports that no row matches a lookup, or that a command finds no follow or
	// follow request to act on.
	ErrNotFound = errors.New("graph: not found")
	// ErrAlreadyRecorded reports a second acceptance, rejection, or termination of one follow.
	ErrAlreadyRecorded = errors.New("graph: the follow already has this event")
)

// Repository is the persistence port of the graph module's follows (data-model §4.5). The
// database adapters implement it and pass testkit.FollowRepositoryContract.
//
// Methods that take a uow.Tx run inside that transaction; the others read committed data outside
// any transaction. Lookups that find nothing return an error wrapping ErrNotFound.
type Repository interface {
	// Resources, events, and current values, written by commands.

	InsertFollow(ctx context.Context, tx uow.Tx, f graph.Follow) error
	// InsertAcceptance, InsertRejection, and InsertTermination fail with ErrAlreadyRecorded when
	// the follow already has an event of the same kind.
	InsertAcceptance(ctx context.Context, tx uow.Tx, a graph.Acceptance) error
	InsertRejection(ctx context.Context, tx uow.Tx, r graph.Rejection) error
	InsertTermination(ctx context.Context, tx uow.Tx, t graph.Termination) error
	UpsertPreferences(ctx context.Context, tx uow.Tx, p graph.Preferences) error
	// Follow returns a follow by ID.
	Follow(ctx context.Context, tx uow.Tx, id string) (graph.Follow, error)
	// Preferences returns the preferences of a follow.
	Preferences(ctx context.Context, tx uow.Tx, followID string) (graph.Preferences, error)

	// Projections, written only by this module's projectors and rebuild.

	// Edge returns the edge from follower to followee.
	Edge(ctx context.Context, tx uow.Tx, followerAccountID, followeeAccountID string) (graph.Edge, error)
	// InsertEdge inserts an edge and reports false, writing nothing, when the pair already has one.
	InsertEdge(ctx context.Context, tx uow.Tx, e graph.Edge) (bool, error)
	// UpdateEdge replaces the state and since of the pair's edge when it belongs to e.FollowID.
	UpdateEdge(ctx context.Context, tx uow.Tx, e graph.Edge) error
	// DeleteEdge deletes the pair's edge when it belongs to followID.
	DeleteEdge(ctx context.Context, tx uow.Tx, followerAccountID, followeeAccountID, followID string) error
	DeleteAllEdges(ctx context.Context, tx uow.Tx) error
	// AddCounters adds the deltas to an account's counters, creating the row from zero, and
	// deletes the row when both counters reach zero: no row means zero.
	AddCounters(ctx context.Context, tx uow.Tx, accountID string, followersDelta, followingDelta int64) error
	UpsertCounters(ctx context.Context, tx uow.Tx, c graph.Counters) error
	DeleteAllCounters(ctx context.Context, tx uow.Tx) error

	// Projection sources, in a stable order, for rebuilding the projections.

	// ListHistories returns every follow with its acceptance, rejection, and termination,
	// ordered by follow ID.
	ListHistories(ctx context.Context, tx uow.Tx) ([]graph.History, error)

	// Projection contents, for verifying the projections.

	// ListEdges returns every edge, ordered by follower and followee.
	ListEdges(ctx context.Context) ([]graph.Edge, error)
	// ListCounters returns every counters row, ordered by account ID.
	ListCounters(ctx context.Context) ([]graph.Counters, error)

	// Lookups.

	FindEdge(ctx context.Context, followerAccountID, followeeAccountID string) (graph.Edge, error)
	FindPreferences(ctx context.Context, followID string) (graph.Preferences, error)
	// FindCounters returns an account's counters; ErrNotFound means both are zero.
	FindCounters(ctx context.Context, accountID string) (graph.Counters, error)
	// ListFollowers returns the edges into followeeAccountID in state, newest follow first,
	// starting after the follow ID before ("" for the newest) and returning at most limit edges.
	ListFollowers(ctx context.Context, followeeAccountID string, state graph.State, before string, limit int) ([]graph.Edge, error)
	// ListFollowing is ListFollowers for the edges out of followerAccountID.
	ListFollowing(ctx context.Context, followerAccountID string, state graph.State, before string, limit int) ([]graph.Edge, error)
}
