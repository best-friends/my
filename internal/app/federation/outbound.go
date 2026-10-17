// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"encoding/json"
	"fmt"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// Job types of outbound delivery (federation.md §7).
const (
	// JobFanOut computes the inboxes of one outbound activity, one batch at a time, and
	// enqueues a JobDeliver per new delivery.
	JobFanOut = "federation.fanout"
	// JobDeliver signs and POSTs one delivery. Its payload is deliveryPayload.
	JobDeliver = "federation.deliver"
)

// fanOutPayload is the payload of JobFanOut. Phase and After are the cursor: the source of
// recipients the job reads and the position in it, both "" for the first batch.
type fanOutPayload struct {
	OutboundActivityID string `json:"outbound_activity_id"`
	// RevisionID is the revision a Create or Update presents (federation.md §7.1), "" for other
	// activities.
	RevisionID string `json:"revision_id,omitempty"`
	Phase      string `json:"phase,omitempty"`
	After      string `json:"after,omitempty"`
}

func (p fanOutPayload) job() (queue.NewJob, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return queue.NewJob{}, err
	}
	return queue.NewJob{
		Type: JobFanOut, Payload: b,
		IdempotencyKey: "fanout:" + p.OutboundActivityID + ":" + p.Phase + ":" + p.After,
	}, nil
}

// deliveryPayload is the payload of JobDeliver.
type deliveryPayload struct {
	DeliveryID string `json:"delivery_id"`
}

func deliverJob(deliveryID string) (queue.NewJob, error) {
	b, err := json.Marshal(deliveryPayload{DeliveryID: deliveryID})
	if err != nil {
		return queue.NewJob{}, err
	}
	return queue.NewJob{Type: JobDeliver, Payload: b, IdempotencyKey: "deliver:" + deliveryID}, nil
}

// RecorderDeps are the ports and settings of the outbound projectors.
type RecorderDeps struct {
	Repository Repository
	// Outbox enqueues the fan-out in the transaction of the local command.
	Outbox queue.Outbox
	IDs    kernel.IDGenerator
	// Host is the serving domain of the activity URIs (ADR-0018).
	Host string
	// Accounts tells local accounts from remote ones in the transaction, for the activities of
	// follows; nil records none of them.
	Accounts LocalityReader
}

// RegisterOutboundProjectors subscribes the federation module to the domain events of other
// modules that a local actor's activity follows from (federation.md §7.1). Each projector writes,
// in the transaction of the local command, an outbound_activities row and the JobFanOut that
// delivers it, so that an activity is recorded exactly when the command commits.
//
//   - content.status_created / status_revised / status_deleted of a local status: Create,
//     Update, and Delete.
//   - content.boosted / boost_retracted of a boost that federates: Announce and Undo Announce.
//     A boost of a local status (Boost.Federates is false) is not recorded (ADR-0017).
//   - graph.followed / follow_requested / follow_accepted / follow_rejected / follow_terminated
//     between a local and a remote account: Follow, Accept, Reject, and Undo Follow
//     (registerFollowProjectors).
//
// Other activities (Like, Block, profile updates, account deletion) are added here, one
// projector each, with the use cases that send them.
func RegisterOutboundProjectors(r *uow.Projectors, d RecorderDeps) {
	rec := recorder{d: d}
	if d.Accounts != nil {
		registerFollowProjectors(r, rec)
	}
	r.Register(appcontent.EventStatusCreated, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.StatusCreated)
		if !ok || !ev.Status.IsLocal {
			return nil
		}
		return rec.record(ctx, tx, federation.OutboundActivity{
			ActivityURI: federation.IRI(ev.Status.URI + "/activity"), Type: federation.ActivityCreate,
			ActorAccountID: ev.Status.AuthorAccountID, ObjectKind: federation.OutboundStatus, ObjectID: ev.Status.ID,
			CreatedAt: ev.Revision.RevisedAt,
		}, ev.Revision.ID)
	})
	r.Register(appcontent.EventStatusRevised, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.StatusRevised)
		if !ok || !ev.Status.IsLocal {
			return nil
		}
		return rec.record(ctx, tx, federation.OutboundActivity{
			Type: federation.ActivityUpdate, ActorAccountID: ev.Status.AuthorAccountID,
			ObjectKind: federation.OutboundStatus, ObjectID: ev.Status.ID, CreatedAt: ev.Revision.RevisedAt,
		}, ev.Revision.ID)
	})
	r.Register(appcontent.EventStatusDeleted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.StatusDeleted)
		if !ok || !ev.Status.IsLocal {
			return nil
		}
		act, err := rec.insert(ctx, tx, federation.OutboundActivity{
			Type: federation.ActivityDelete, ActorAccountID: ev.Status.AuthorAccountID,
			ObjectKind: federation.OutboundStatus, ObjectID: ev.Status.ID, CreatedAt: ev.Deletion.DeletedAt,
		})
		if err != nil {
			return err
		}
		// The document is stored with the deletion, so that every delivery of the Delete
		// sends it whether or not the status's rows are still there (federation.md §7.1).
		doc, err := deleteDocument(act, d.Host, ev.Status)
		if err != nil {
			return err
		}
		if _, err := d.Repository.InsertOutboundDocument(ctx, tx, act.ID, doc, act.CreatedAt); err != nil {
			return err
		}
		return rec.enqueue(ctx, tx, act, "")
	})
	r.Register(appcontent.EventBoosted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.Boosted)
		if !ok || !ev.Boost.Federates() {
			return nil
		}
		return rec.record(ctx, tx, federation.OutboundActivity{
			ActivityURI: federation.IRI(ev.Boost.URI), Type: federation.ActivityAnnounce,
			ActorAccountID: ev.Boost.BoosterAccountID, ObjectKind: federation.OutboundBoost, ObjectID: ev.Boost.ID,
			CreatedAt: ev.Boost.BoostedAt,
		}, "")
	})
	r.Register(appcontent.EventBoostRetracted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.BoostRetracted)
		if !ok || !ev.Boost.Federates() {
			return nil
		}
		return rec.record(ctx, tx, federation.OutboundActivity{
			Type: federation.ActivityUndo, ActorAccountID: ev.Boost.BoosterAccountID,
			ObjectKind: federation.OutboundBoost, ObjectID: ev.Boost.ID, CreatedAt: ev.Retraction.RetractedAt,
		}, "")
	})
}

type recorder struct{ d RecorderDeps }

// record inserts a and enqueues its first fan-out batch in tx. An activity without a URI gets
// /ap/activities/{id} (ADR-0018). The ID is generated here, in the transaction: a retried
// attempt starts over with a new one, and nothing of the rolled-back attempt remains.
func (r recorder) record(ctx context.Context, tx uow.Tx, a federation.OutboundActivity, revisionID string) error {
	a, err := r.insert(ctx, tx, a)
	if err != nil {
		return err
	}
	return r.enqueue(ctx, tx, a, revisionID)
}

// insert gives a its ID and URI and inserts it in tx.
func (r recorder) insert(ctx context.Context, tx uow.Tx, a federation.OutboundActivity) (federation.OutboundActivity, error) {
	a.ID = r.d.IDs.NewID()
	if a.ActivityURI == "" {
		a.ActivityURI = federation.IRI(appcontent.ActivityURI(r.d.Host, a.ID))
	}
	a.CreatedAt = kernel.Normalize(a.CreatedAt)
	if err := a.Validate(); err != nil {
		return federation.OutboundActivity{}, err
	}
	if err := r.d.Repository.InsertOutboundActivity(ctx, tx, a); err != nil {
		return federation.OutboundActivity{}, fmt.Errorf("record %s %s: %w", a.Type, a.ObjectID, err)
	}
	return a, nil
}

// enqueue enqueues the first fan-out batch of a in tx.
func (r recorder) enqueue(ctx context.Context, tx uow.Tx, a federation.OutboundActivity, revisionID string) error {
	job, err := fanOutPayload{OutboundActivityID: a.ID, RevisionID: revisionID}.job()
	if err != nil {
		return err
	}
	return r.d.Outbox.EnqueueTx(ctx, tx, job)
}
