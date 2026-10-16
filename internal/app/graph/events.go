// SPDX-License-Identifier: AGPL-3.0-only

package graph

import "github.com/best-friends/my/internal/domain/graph"

// Event types of the graph module, for uow.Projectors (data-model §4.5). Every follow command
// emits exactly one of them inside its transaction, after writing the event rows, so that the
// projectors of this module (follow_edges, follow_counters, block_edges) and of other modules
// (timeline's feed_follow_edges, notification's follow notifications) see the same facts.
// Each event carries everything its subscribers need; they never read the graph tables.
// Blocking emits one EventFollowTerminated per follow it ends, then EventBlocked. Mutes are a
// current value read at query time and emit no events.
//
//   - EventFollowed: a follow became active on creation (the followee is not locked).
//   - EventFollowRequested: a follow awaits the followee's approval (the followee is locked).
//   - EventFollowAccepted: a pending follow became active.
//   - EventFollowRejected: a pending follow was declined.
//   - EventFollowTerminated: a pending or active follow ended.
//   - EventFollowPreferencesChanged: the follower changed the preferences of a live follow.
//   - EventBlocked: an account blocked another.
//   - EventUnblocked: a block was lifted.
const (
	EventFollowed                 = "graph.followed"
	EventFollowRequested          = "graph.follow_requested"
	EventFollowAccepted           = "graph.follow_accepted"
	EventFollowRejected           = "graph.follow_rejected"
	EventFollowTerminated         = "graph.follow_terminated"
	EventFollowPreferencesChanged = "graph.follow_preferences_changed"
	EventBlocked                  = "graph.blocked"
	EventUnblocked                = "graph.unblocked"
)

// Followed is emitted when a follow is created and accepted at once.
type Followed struct {
	Follow      graph.Follow
	Acceptance  graph.Acceptance
	Preferences graph.Preferences
}

// EventType implements uow.Event.
func (Followed) EventType() string { return EventFollowed }

// FollowRequested is emitted when a follow is created pending the followee's approval.
type FollowRequested struct {
	Follow      graph.Follow
	Preferences graph.Preferences
}

// EventType implements uow.Event.
func (FollowRequested) EventType() string { return EventFollowRequested }

// FollowAccepted is emitted when a pending follow is accepted.
type FollowAccepted struct {
	Follow     graph.Follow
	Acceptance graph.Acceptance
	// Preferences are the follow's preferences at acceptance, for subscribers that start
	// projecting the follow now that it is active.
	Preferences graph.Preferences
}

// EventType implements uow.Event.
func (FollowAccepted) EventType() string { return EventFollowAccepted }

// FollowRejected is emitted when a pending follow is rejected.
type FollowRejected struct {
	Follow    graph.Follow
	Rejection graph.Rejection
}

// EventType implements uow.Event.
func (FollowRejected) EventType() string { return EventFollowRejected }

// FollowTerminated is emitted when a pending or active follow ends.
type FollowTerminated struct {
	Follow      graph.Follow
	Termination graph.Termination
	// PreviousState is the state the follow ended in: graph.StatePending or graph.StateActive.
	PreviousState graph.State
}

// EventType implements uow.Event.
func (FollowTerminated) EventType() string { return EventFollowTerminated }

// FollowPreferencesChanged is emitted when the preferences of a live follow change.
type FollowPreferencesChanged struct {
	Follow      graph.Follow
	State       graph.State // the follow's state when its preferences changed
	Preferences graph.Preferences
}

// EventType implements uow.Event.
func (FollowPreferencesChanged) EventType() string { return EventFollowPreferencesChanged }

// Blocked is emitted when an account blocks another, after the FollowTerminated events of the
// follows between them that the block ended.
type Blocked struct {
	Block graph.Block
}

// EventType implements uow.Event.
func (Blocked) EventType() string { return EventBlocked }

// Unblocked is emitted when a block is lifted.
type Unblocked struct {
	Block      graph.Block
	Retraction graph.BlockRetraction
}

// EventType implements uow.Event.
func (Unblocked) EventType() string { return EventUnblocked }
