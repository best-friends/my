// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// ErrNotOwnActor reports an activity that the actor handlers were given although it is not an
// Update or a Delete of its own actor.
var ErrNotOwnActor = errors.New("federation: the activity is not about its own actor")

// IsOwnActorActivity reports whether act is an Update or a Delete whose object is its own actor
// (actor == object), the activities ReceiveActorUpdate and ReceiveActorDelete handle
// (federation.md §3.4). The inbox dispatches every other Update and Delete to the handlers of
// statuses.
func IsOwnActorActivity(act *federation.Activity) bool {
	if act == nil || (act.Type != federation.ActivityUpdate && act.Type != federation.ActivityDelete) {
		return false
	}
	return act.Actor != "" && act.Object().ID == act.Actor
}

// ReceiveActorUpdate handles the Update of a remote actor by itself (federation.md §3.4). raw is
// the activity's document as the inbox job read it. An embedded actor document with the
// activity's origin and the actor's id is stored as if it were fetched: a snapshot when the
// document changed, a profile revision when the sanitized profile changed, the keys it declares,
// and the handle checked again with WebFinger (account.UpsertRemoteActor). Any other Update of
// the actor (an IRI only, or a document of another origin) makes the actor be fetched again from
// its server.
//
// The Update of an actor that is deleted, or local, is a no-op: deletion is final.
//
// The caller has checked that the signer of the request is the activity's actor, or fetched the
// activity again from its origin (§3.2). Handling an Update again is a no-op: the document is
// the same, so is its snapshot, and so is the profile. A fetch failure is returned as it is
// (ErrFetchFailed and ErrFetchUnavailable are temporary).
func (r *RemoteActors) ReceiveActorUpdate(ctx context.Context, act *federation.Activity, raw []byte) error {
	if !IsOwnActorActivity(act) || act.Type != federation.ActivityUpdate {
		return fmt.Errorf("%w: %s %s", ErrNotOwnActor, activityType(act), activityID(act))
	}
	switch a, err := r.d.Accounts.FindByURI(ctx, string(act.Actor)); {
	case err == nil && (a.Account.IsLocal() || a.Deleted()):
		return nil // a deleted actor stays deleted, and a local one is not another server's
	case err != nil && !errors.Is(err, appaccount.ErrNotFound):
		return err
	}
	embedded := act.Object().Trusted()
	if embedded == nil || embedded.Actor == nil || embedded.Actor.ID != act.Actor {
		_, err := r.Refresh(ctx, string(act.Actor))
		return err
	}
	actor, doc, err := usableActor(&Fetched{Document: embedded, Raw: objectOf(raw)})
	if err != nil {
		return err
	}
	in := r.input(ctx, actor, doc)
	return r.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if _, err := r.d.Accounts.UpsertRemoteActor(ctx, tx, in); err != nil {
			return err
		}
		return r.seeInstance(ctx, tx, in.Domain)
	})
}

// ReceiveActorDelete handles the Delete of a remote actor by itself (federation.md §3.4): the
// account becomes remote_deleted (account.MarkRemoteDeleted), which hides its profile and posts
// at once; the purge removes them later. The Delete of an actor that is unknown or local, and a
// Delete handled again, are no-ops. The caller has checked the signer as for ReceiveActorUpdate.
func (r *RemoteActors) ReceiveActorDelete(ctx context.Context, act *federation.Activity) error {
	if !IsOwnActorActivity(act) || act.Type != federation.ActivityDelete {
		return fmt.Errorf("%w: %s %s", ErrNotOwnActor, activityType(act), activityID(act))
	}
	a, err := r.d.Accounts.FindByURI(ctx, string(act.Actor))
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		return nil
	case err != nil:
		return err
	case a.Account.IsLocal():
		return nil
	}
	return r.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return r.d.Accounts.MarkRemoteDeleted(ctx, tx, a.Account.ID) })
}

// objectOf returns the JSON of the object of an activity document, nil when it has none. The
// keys of an embedded actor are read from it.
func objectOf(raw []byte) []byte {
	var env struct {
		Object json.RawMessage `json:"object"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return nil
	}
	return env.Object
}

func activityType(act *federation.Activity) federation.ActivityType {
	if act == nil {
		return federation.ActivityUnknown
	}
	return act.Type
}

func activityID(act *federation.Activity) federation.IRI {
	if act == nil {
		return ""
	}
	return act.ID
}
