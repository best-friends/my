// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"reflect"
	"testing"
	"time"

	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

// moderationSuspensions checks AccountSuspensionChanged (federation.md §11.1): an action that
// suspends an account that was not suspended tells it, the lift or the expiry of its last
// suspension tells the end once, whichever removes the suspension first, and the expiry job
// removes the expired restrictions of every kind.
func moderationSuspensions(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	issue := func(kind moderation.ActionKind, d time.Duration) moderation.Action {
		t.Helper()
		a, err := e.svc.IssueAccountAction(ctx, appmoderation.AccountActionInput{
			TargetAccountID: e.local, Kind: kind, ReasonCategory: moderation.CategoryOther, Duration: d,
		})
		if err != nil {
			t.Fatalf("IssueAccountAction(%s) = %v", kind, err)
		}
		return a
	}
	lift := func(a moderation.Action) {
		t.Helper()
		if err := e.svc.LiftAccountAction(ctx, a.ID, "", ""); err != nil {
			t.Fatalf("LiftAccountAction(%s) = %v", a.Kind, err)
		}
	}
	expire := func(want int) {
		t.Helper()
		n, err := e.svc.ExpireAccountRestrictions(ctx)
		if err != nil || n != want {
			t.Fatalf("ExpireAccountRestrictions = %d, %v; want %d removed", n, err, want)
		}
	}
	told := func(what string, want ...appmoderation.AccountSuspensionChanged) {
		t.Helper()
		got := e.suspensions.take()
		if len(got) == 0 && len(want) == 0 {
			return
		}
		ws := make([]any, len(want))
		for i, w := range want {
			ws[i] = w
		}
		gs := make([]any, len(got))
		for i, g := range got {
			gs[i] = g
		}
		if !reflect.DeepEqual(gs, ws) {
			t.Errorf("%s: suspension events = %+v, want %+v", what, gs, ws)
		}
	}
	change := func(suspended bool) appmoderation.AccountSuspensionChanged {
		return appmoderation.AccountSuspensionChanged{AccountID: e.local, Suspended: suspended, ChangedAt: e.clock.Now()}
	}
	suspendRows := len(moderation.Action{Kind: moderation.ActionSuspend, ExpiresAt: &time.Time{}}.Rows())

	// The first suspension is told; a second one, and the lift of one of the two, are not.
	issue(moderation.ActionSuspend, time.Hour)
	told("first suspension", change(true))
	forever := issue(moderation.ActionSuspend, 0)
	told("second suspension")
	lift(forever)
	told("lift of one of two")
	// Another kind of action tells nothing.
	issue(moderation.ActionDisableLogin, time.Hour)
	told("disable_login")
	expire(0)

	// The expiry of the last suspension is told once, by the first run after it; the run removes
	// the expired rows of every kind.
	e.clock.Advance(2 * time.Hour)
	expire(suspendRows + 1)
	told("expiry", change(false))
	expire(0)
	told("expiry again")
	var rows []moderation.RestrictionRow
	err := e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		var err error
		rows, err = e.h.Moderation.AccountRestrictionRows(ctx, tx, e.local)
		return err
	})
	if err != nil || len(rows) != 0 {
		t.Errorf("rows after the expiry = %+v, %v; want none", rows, err)
	}

	// A lift after the expiry and before the run tells the end, and the run then has nothing
	// to tell.
	a := issue(moderation.ActionSuspend, time.Hour)
	told("suspension", change(true))
	e.clock.Advance(2 * time.Hour)
	lift(a)
	told("lift of an expired suspension", change(false))
	expire(0)
	told("expiry after the lift")

	// A suspension over an expired one that the run has not removed is told, and removing the
	// expired one tells nothing.
	issue(moderation.ActionSuspend, time.Hour)
	told("suspension", change(true))
	e.clock.Advance(2 * time.Hour)
	issue(moderation.ActionSuspend, time.Hour)
	told("suspension over an expired one", change(true))
	expire(suspendRows)
	told("removal of the expired suspension under another")
	if ok, err := e.policy.AccountFederationSuspended(ctx, e.local); err != nil || !ok {
		t.Errorf("AccountFederationSuspended after the removal = %v, %v; want true", ok, err)
	}
}
