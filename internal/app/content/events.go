// SPDX-License-Identifier: AGPL-3.0-only

package content

import "github.com/best-friends/my/internal/domain/content"

// Event types of the content module's statuses, for uow.Projectors (data-model §4.4). Every
// status command emits exactly one of them inside its transaction, after writing the resource
// and event rows, so that the projectors of this module (status_heads, status_mention_index,
// status_reply_links, status_counters, account_post_counters) and of other modules (timeline's
// feed_* entries, notification's mention notifications) see the same facts. Each event carries
// everything its subscribers need; they never read the content tables.
//
//   - EventStatusCreated: a status was posted, with its first revision.
//   - EventStatusRevised: a status was edited; the event carries the new revision (FR-109).
//   - EventStatusDeleted: a status was deleted; the deletion is its tombstone (FR-108).
const (
	EventStatusCreated = "content.status_created"
	EventStatusRevised = "content.status_revised"
	EventStatusDeleted = "content.status_deleted"
)

// StatusCreated is emitted when a status and its first revision are inserted.
type StatusCreated struct {
	Status   content.Status
	Revision content.Revision
}

// EventType implements uow.Event.
func (StatusCreated) EventType() string { return EventStatusCreated }

// StatusRevised is emitted when a later revision of a status is inserted. Status is the
// status the revision belongs to; its facts (author, visibility, reply target) never change.
type StatusRevised struct {
	Status   content.Status
	Revision content.Revision
}

// EventType implements uow.Event.
func (StatusRevised) EventType() string { return EventStatusRevised }

// StatusDeleted is emitted when the deletion of a status is inserted.
type StatusDeleted struct {
	Deletion content.Deletion
	Status   content.Status
}

// EventType implements uow.Event.
func (StatusDeleted) EventType() string { return EventStatusDeleted }

// ByAccountErasure reports whether the erasure of the author's deleted account recorded the
// deletion (data-model §8): the Delete of the actor stands for it, so it sends no Delete.
func (e StatusDeleted) ByAccountErasure() bool {
	return e.Deletion.Origin == content.DeletedByAccountDeletion
}

// EventPollTallyObserved is emitted when new counts of a remote poll are recorded; the
// poll_tallies projection follows the newest observation.
const EventPollTallyObserved = "content.poll_tally_observed"

// PollTallyObserved is emitted when a tally observation of a poll is inserted.
type PollTallyObserved struct {
	Observation content.PollTallyObservation
}

// EventType implements uow.Event.
func (PollTallyObserved) EventType() string { return EventPollTallyObserved }
