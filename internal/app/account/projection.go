// SPDX-License-Identifier: AGPL-3.0-only

package account

import (
	"context"
	"errors"
	"fmt"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
)

// Event types of the account module, for uow.Projectors.
const (
	EventLocalAccountCreated    = "account.local_account_created"
	EventProfileRevised         = "account.profile_revised"
	EventLifecycleEventRecorded = "account.lifecycle_event_recorded"
)

// LocalAccountCreated is emitted when a local account and its subtype row are inserted.
type LocalAccountCreated struct {
	Account account.Account
	Local   account.LocalAccount
}

// EventType implements uow.Event.
func (LocalAccountCreated) EventType() string { return EventLocalAccountCreated }

// ProfileRevised is emitted when a profile revision is inserted.
type ProfileRevised struct {
	Revision account.ProfileRevision
}

// EventType implements uow.Event.
func (ProfileRevised) EventType() string { return EventProfileRevised }

// LifecycleEventRecorded is emitted when a lifecycle event is inserted.
type LifecycleEventRecorded struct {
	Event account.LifecycleEvent
}

// EventType implements uow.Event.
func (LifecycleEventRecorded) EventType() string { return EventLifecycleEventRecorded }

// RegisterProjectors registers the account module's synchronous projectors (data-model §9):
// account_handles from local accounts, account_profile_heads from profile revisions, and
// account_lifecycle_states from lifecycle events. Each projector applies the same rule as
// RebuildProjections, so updating incrementally and rebuilding agree.
func RegisterProjectors(r *uow.Projectors, repo Repository) {
	r.Register(EventLocalAccountCreated, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(LocalAccountCreated)
		return repo.UpsertHandle(ctx, tx, localHandle(ev.Local))
	})
	r.Register(EventProfileRevised, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		rev := e.(ProfileRevised).Revision
		head, err := repo.ProfileHead(ctx, tx, rev.AccountID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		if !rev.Supersedes(head) {
			return nil
		}
		return repo.UpsertProfileHead(ctx, tx, account.ProfileHead{AccountID: rev.AccountID, RevisionID: rev.ID, Seq: rev.Seq})
	})
	r.Register(EventLifecycleEventRecorded, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(LifecycleEventRecorded).Event
		state, err := repo.LifecycleState(ctx, tx, ev.AccountID)
		if errors.Is(err, ErrNotFound) {
			state, err = account.InitialLifecycleState(ev.AccountID), nil
		}
		if err != nil {
			return err
		}
		return repo.UpsertLifecycleState(ctx, tx, state.Apply(ev))
	})
}

func localHandle(la account.LocalAccount) account.Handle {
	return account.Handle{
		AccountID:          la.AccountID,
		Username:           la.Username,
		UsernameNormalized: la.UsernameNormalized,
		Domain:             account.LocalDomain,
	}
}

// RebuildProjections empties the account module's projections and derives them again from
// their sources (data-model §9), in one transaction per projection.
func RebuildProjections(ctx context.Context, u uow.UnitOfWork, repo Repository) error {
	steps := []struct {
		name string
		fn   func(tx uow.Tx) error
	}{
		{"account_handles", func(tx uow.Tx) error { return rebuildHandles(ctx, tx, repo) }},
		{"account_profile_heads", func(tx uow.Tx) error { return rebuildProfileHeads(ctx, tx, repo) }},
		{"account_lifecycle_states", func(tx uow.Tx) error { return rebuildLifecycleStates(ctx, tx, repo) }},
	}
	for _, s := range steps {
		if err := u.Tx(ctx, s.fn); err != nil {
			return fmt.Errorf("rebuild %s: %w", s.name, err)
		}
	}
	return nil
}

func rebuildHandles(ctx context.Context, tx uow.Tx, repo Repository) error {
	if err := repo.DeleteAllHandles(ctx, tx); err != nil {
		return err
	}
	locals, err := repo.ListLocalAccounts(ctx, tx)
	if err != nil {
		return err
	}
	for _, la := range locals {
		if err := repo.UpsertHandle(ctx, tx, localHandle(la)); err != nil {
			return err
		}
	}
	return nil
}

func rebuildProfileHeads(ctx context.Context, tx uow.Tx, repo Repository) error {
	if err := repo.DeleteAllProfileHeads(ctx, tx); err != nil {
		return err
	}
	revs, err := repo.ListProfileRevisionSeqs(ctx, tx)
	if err != nil {
		return err
	}
	heads := map[string]account.ProfileHead{}
	var order []string
	for _, r := range revs {
		head, seen := heads[r.AccountID]
		if !seen {
			order = append(order, r.AccountID)
		}
		if (account.ProfileRevision{ID: r.RevisionID, Seq: r.Seq}).Supersedes(head) {
			heads[r.AccountID] = r
		}
	}
	for _, id := range order {
		if err := repo.UpsertProfileHead(ctx, tx, heads[id]); err != nil {
			return err
		}
	}
	return nil
}

func rebuildLifecycleStates(ctx context.Context, tx uow.Tx, repo Repository) error {
	if err := repo.DeleteAllLifecycleStates(ctx, tx); err != nil {
		return err
	}
	events, err := repo.ListLifecycleEvents(ctx, tx)
	if err != nil {
		return err
	}
	states := map[string]account.LifecycleState{}
	var order []string
	for _, e := range events {
		s, seen := states[e.AccountID]
		if !seen {
			order = append(order, e.AccountID)
			s = account.InitialLifecycleState(e.AccountID)
		}
		states[e.AccountID] = s.Apply(e)
	}
	for _, id := range order {
		if err := repo.UpsertLifecycleState(ctx, tx, states[id]); err != nil {
			return err
		}
	}
	return nil
}
