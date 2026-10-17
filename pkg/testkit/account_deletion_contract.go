// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"errors"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
)

// accountExecutesDeletions checks the deletions that moderation schedules and their execution
// (data-model §4.2, §8): a ban's deletion_scheduled names the action and keeps an earlier due
// time, a remote account takes none, and ExecuteDueDeletions deletes the accounts whose
// deletion is due, once, leaving the others and the cancelled ones.
func accountExecutesDeletions(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	banned, later, cancelled := e.register(t, "banned"), e.register(t, "later"), e.register(t, "cancelled")
	schedule := func(id, action string, due time.Time) {
		t.Helper()
		err := e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			return e.accounts.ScheduleDeletionByModeration(ctx, tx, id, action, due)
		})
		if err != nil {
			t.Fatalf("ScheduleDeletionByModeration(%s) = %v", id, err)
		}
	}
	state := func(id string) account.LifecycleState {
		t.Helper()
		a, err := e.accounts.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("FindByID = %v", err)
		}
		return a.Lifecycle
	}
	day := 24 * time.Hour
	due := e.clock.Now().Add(30 * day)
	schedule(banned.AccountID, "action-1", due)
	// A later due time keeps the earlier one; an earlier one replaces it.
	schedule(banned.AccountID, "action-2", due.Add(day))
	if s := state(banned.AccountID); s.State != account.StateDeletionScheduled || s.DeletionDueAt == nil || !s.DeletionDueAt.Equal(due) {
		t.Errorf("state after a later schedule = %+v, want due %v", s, due)
	}
	schedule(later.AccountID, "action-3", due.Add(day))
	schedule(cancelled.AccountID, "action-4", due)
	if _, err := e.accounts.RecordLifecycleEvent(ctx, appaccount.LifecycleInput{
		AccountID: cancelled.AccountID, Kind: account.LifecycleDeletionCancelled, InitiatedBy: account.InitiatedByModerator,
	}); err != nil {
		t.Fatal(err)
	}
	err := e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		events, err := e.h.Accounts.ListLifecycleEvents(ctx, tx)
		for _, ev := range events {
			if ev.AccountID == banned.AccountID && (ev.ModerationActionID != "action-1" || ev.InitiatedBy != account.InitiatedByModerator) {
				t.Errorf("event of the ban = %+v, want one by a moderator naming action-1", ev)
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	run := func(want int) {
		t.Helper()
		if n, err := e.accounts.ExecuteDueDeletions(ctx); err != nil || n != want {
			t.Errorf("ExecuteDueDeletions = %d, %v; want %d", n, err, want)
		}
	}
	run(0)
	e.clock.Advance(30 * day)
	run(1)
	if s := state(banned.AccountID); s.State != account.StateDeleted {
		t.Errorf("banned account after its due time = %+v, want deleted", s)
	}
	if s := state(later.AccountID); s.State != account.StateDeletionScheduled {
		t.Errorf("account due later = %+v, want deletion_scheduled", s)
	}
	run(0)
	e.clock.Advance(day)
	run(1)
	if s := state(cancelled.AccountID); s.State != account.StateActive {
		t.Errorf("cancelled account = %+v, want active", s)
	}
	// A deleted account stays deleted.
	schedule(banned.AccountID, "action-5", e.clock.Now().Add(day))
	if s := state(banned.AccountID); s.State != account.StateDeleted {
		t.Errorf("deleted account after a schedule = %+v, want deleted", s)
	}

	remote := account.Account{
		ID: e.ids.NewID(), URI: "https://remote.example/users/r", Domain: "remote.example",
		ActorType: account.ActorPerson, CreatedAt: e.clock.Now(),
	}
	err = e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := e.h.Accounts.InsertAccount(ctx, tx, remote); err != nil {
			return err
		}
		return e.accounts.ScheduleDeletionByModeration(ctx, tx, remote.ID, "action-6", e.clock.Now().Add(day))
	})
	if !errors.Is(err, appaccount.ErrNotLocal) {
		t.Errorf("ScheduleDeletionByModeration(remote) = %v, want ErrNotLocal", err)
	}
}
