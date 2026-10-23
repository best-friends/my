// SPDX-License-Identifier: AGPL-3.0-only

package supporters

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	// Plan and price ids are ids of this server, so they are UUIDv7 values.
	planBasic       = "01890a5d-ac96-7000-8000-000000000001"
	planGold        = "01890a5d-ac96-7000-8000-000000000002"
	planA           = "01890a5d-ac96-7000-8000-000000000003"
	planB           = "01890a5d-ac96-7000-8000-000000000004"
	priceBasicMonth = "01890a5d-ac96-7000-8000-0000000000a1"
	priceGoldMonth  = "01890a5d-ac96-7000-8000-0000000000a2"
	priceA1         = "01890a5d-ac96-7000-8000-0000000000b1"
	priceA2         = "01890a5d-ac96-7000-8000-0000000000b2"
	priceB1         = "01890a5d-ac96-7000-8000-0000000000b3"
)

var (
	t0         = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	day        = 24 * time.Hour
	periodA    = t0.Add(30 * day)
	periodB    = periodA.Add(30 * day)
	failedTime = t0.Add(10 * day)
)

func active(plan, price string) Transition {
	return Transition{State: StateActive, PlanID: plan, PriceID: price, PeriodEndsAt: periodA}
}

func snapshot(status ProviderStatus) *Snapshot {
	return &Snapshot{Status: status, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEnd: periodA}
}

func with(s *Snapshot, f func(*Snapshot)) *Snapshot {
	f(s)
	return s
}

// The state of a subscription follows from the snapshot and our facts (ADR-0072 §7): the grace
// of 7 days, the end of the period, refunds and chargebacks, and the change of plan.
func TestDerive(t *testing.T) {
	pending := Transition{State: StatePending, PlanID: planBasic, PriceID: priceBasicMonth}
	pastDue := Transition{
		State: StatePastDue, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEndsAt: periodA,
		Grace: &Grace{FailedAt: failedTime, EndsAt: failedTime.Add(GracePeriod)},
	}
	lapsed := Transition{State: StateLapsed, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEndsAt: periodA}
	for _, tc := range []struct {
		name     string
		prev     Transition
		snap     *Snapshot
		facts    Facts
		want     State
		reason   Reason
		role     bool
		check    func(t *testing.T, got Transition)
		noChange bool
	}{
		{name: "first payment made", prev: pending, snap: snapshot(ProviderActive), want: StateActive, reason: ReasonSynced, role: true,
			check: func(t *testing.T, got Transition) {
				if !got.PeriodEndsAt.Equal(periodA) {
					t.Errorf("period end %v", got.PeriodEndsAt)
				}
			}},
		{name: "a trial is active", prev: pending, snap: snapshot(ProviderActive), want: StateActive, reason: ReasonSynced, role: true},
		{name: "checkout not paid yet", prev: pending, snap: snapshot(ProviderIncomplete), want: StatePending, noChange: true},
		{name: "no snapshot within a day", prev: pending, facts: Facts{Now: t0.Add(23 * time.Hour)}, want: StatePending, noChange: true},
		{name: "no snapshot after a day", prev: pending, facts: Facts{Now: t0.Add(CheckoutTimeout)}, want: StateAbandoned, reason: ReasonSynced},
		{name: "incomplete expired", prev: pending, snap: snapshot(ProviderIncompleteExpired), want: StateAbandoned, reason: ReasonSynced,
			check: func(t *testing.T, got Transition) {
				if !got.PeriodEndsAt.IsZero() {
					t.Errorf("abandoned with a period end %v", got.PeriodEndsAt)
				}
			}},
		{name: "same snapshot again", prev: active(planBasic, priceBasicMonth), snap: snapshot(ProviderActive), want: StateActive, role: true, noChange: true},
		{name: "renewed", prev: active(planBasic, priceBasicMonth), snap: with(snapshot(ProviderActive), func(s *Snapshot) { s.PeriodEnd = periodB }),
			want: StateActive, reason: ReasonSynced, role: true},
		{name: "canceled keeps the role to the period end", prev: active(planBasic, priceBasicMonth),
			snap: with(snapshot(ProviderActive), func(s *Snapshot) { s.CancelAtPeriodEnd = true }), want: StateCanceling, reason: ReasonSynced, role: true},
		{name: "cancellation withdrawn", prev: Transition{State: StateCanceling, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEndsAt: periodA},
			snap: snapshot(ProviderActive), want: StateActive, reason: ReasonSynced, role: true},
		{name: "period ended after a cancellation", prev: Transition{State: StateCanceling, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEndsAt: periodA},
			snap: snapshot(ProviderCanceled), facts: Facts{Now: periodA}, want: StateExpired, reason: ReasonSynced},
		{name: "payment failed", prev: active(planBasic, priceBasicMonth),
			snap:  with(snapshot(ProviderPastDue), func(s *Snapshot) { s.PaymentFailedAt = failedTime }),
			facts: Facts{Now: failedTime.Add(time.Hour)}, want: StatePastDue, reason: ReasonSynced, role: true,
			check: func(t *testing.T, got Transition) {
				if got.Grace == nil || !got.Grace.EndsAt.Equal(failedTime.Add(7*day)) {
					t.Errorf("grace %+v", got.Grace)
				}
			}},
		{name: "failure without a time starts the grace now", prev: active(planBasic, priceBasicMonth), snap: snapshot(ProviderUnpaid),
			facts: Facts{Now: failedTime}, want: StatePastDue, reason: ReasonSynced, role: true,
			check: func(t *testing.T, got Transition) {
				if got.Grace == nil || !got.Grace.FailedAt.Equal(failedTime) {
					t.Errorf("grace %+v", got.Grace)
				}
			}},
		{name: "failure without a time keeps the grace", prev: pastDue, snap: snapshot(ProviderPastDue),
			facts: Facts{Now: failedTime.Add(3 * day)}, want: StatePastDue, role: true, noChange: true},
		{name: "last moment of the grace", prev: pastDue, snap: with(snapshot(ProviderPastDue), func(s *Snapshot) { s.PaymentFailedAt = failedTime }),
			facts: Facts{Now: failedTime.Add(GracePeriod - time.Nanosecond)}, want: StatePastDue, role: true, noChange: true},
		{name: "grace expired", prev: pastDue, snap: with(snapshot(ProviderPastDue), func(s *Snapshot) { s.PaymentFailedAt = failedTime }),
			facts: Facts{Now: failedTime.Add(GracePeriod)}, want: StateLapsed, reason: ReasonGraceExpired},
		{name: "grace expired while we missed it", prev: active(planBasic, priceBasicMonth),
			snap:  with(snapshot(ProviderUnpaid), func(s *Snapshot) { s.PaymentFailedAt = failedTime }),
			facts: Facts{Now: failedTime.Add(8 * day)}, want: StateLapsed, reason: ReasonSynced},
		{name: "paid within the grace", prev: pastDue, snap: snapshot(ProviderActive), facts: Facts{Now: failedTime.Add(2 * day)},
			want: StateActive, reason: ReasonSynced, role: true,
			check: func(t *testing.T, got Transition) {
				if got.Grace != nil {
					t.Errorf("grace left %+v", got.Grace)
				}
			}},
		{name: "paid after the grace", prev: lapsed, snap: snapshot(ProviderActive), facts: Facts{Now: failedTime.Add(20 * day)},
			want: StateActive, reason: ReasonSynced, role: true},
		{name: "lapsed stays lapsed without a failure time", prev: lapsed, snap: snapshot(ProviderPastDue), facts: Facts{Now: failedTime.Add(20 * day)},
			want: StateLapsed, noChange: true},
		{name: "paused", prev: active(planBasic, priceBasicMonth), snap: snapshot(ProviderPaused), want: StateLapsed, reason: ReasonSynced},
		{name: "unknown provider state", prev: active(planBasic, priceBasicMonth), snap: snapshot("weird"), want: StateLapsed, reason: ReasonSynced},
		{name: "ended at the provider", prev: active(planBasic, priceBasicMonth), snap: snapshot(ProviderCanceled), want: StateExpired, reason: ReasonSynced},
		{name: "full refund or chargeback", prev: active(planBasic, priceBasicMonth), snap: snapshot(ProviderActive), facts: Facts{Reversed: true},
			want: StateRevoked, reason: ReasonReversed},
		{name: "revoked stays revoked after a won dispute", prev: Transition{State: StateRevoked, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEndsAt: periodA},
			snap: snapshot(ProviderActive), want: StateRevoked, noChange: true},
		{name: "expired is final", prev: Transition{State: StateExpired, PlanID: planBasic, PriceID: priceBasicMonth, PeriodEndsAt: periodA},
			snap: snapshot(ProviderActive), want: StateExpired, noChange: true},
		{name: "upgrade shows in the snapshot", prev: active(planBasic, priceBasicMonth),
			snap: with(snapshot(ProviderActive), func(s *Snapshot) { s.PlanID, s.PriceID = planGold, priceGoldMonth }),
			want: StateActive, reason: ReasonSynced, role: true,
			check: func(t *testing.T, got Transition) {
				if got.PlanID != planGold || got.PriceID != priceGoldMonth {
					t.Errorf("plan %s price %s", got.PlanID, got.PriceID)
				}
			}},
		{name: "downgrade waits for the period end", prev: active(planGold, priceGoldMonth),
			snap: &Snapshot{Status: ProviderActive, PlanID: planGold, PriceID: priceGoldMonth, PeriodEnd: periodA, ScheduledPriceID: priceBasicMonth},
			want: StateActive, reason: ReasonSynced, role: true,
			check: func(t *testing.T, got Transition) {
				if got.PlanID != planGold || got.Scheduled == nil || got.Scheduled.PriceID != priceBasicMonth || !got.Scheduled.EffectiveAt.Equal(periodA) {
					t.Errorf("plan %s scheduled %+v", got.PlanID, got.Scheduled)
				}
			}},
		{name: "unknown price keeps the plan", prev: active(planBasic, priceBasicMonth),
			snap: with(snapshot(ProviderActive), func(s *Snapshot) { s.PlanID, s.PriceID = "", "" }), want: StateActive, role: true, noChange: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.facts.Now.IsZero() {
				tc.facts.Now = t0.Add(time.Hour)
			}
			if tc.facts.StartedAt.IsZero() {
				tc.facts.StartedAt = t0
			}
			got, reason := Derive(tc.prev, tc.snap, tc.facts)
			if got.State != tc.want {
				t.Fatalf("state %s, want %s", got.State, tc.want)
			}
			if tc.noChange {
				if reason != "" || !got.Same(tc.prev) {
					t.Fatalf("a change %+v (%s) where none is", got, reason)
				}
			} else if reason != tc.reason {
				t.Fatalf("reason %q, want %q", reason, tc.reason)
			}
			if got.HoldsRole() != tc.role {
				t.Fatalf("holds the role %v, want %v", got.HoldsRole(), tc.role)
			}
			if got.State.HasPeriod() == got.PeriodEndsAt.IsZero() {
				t.Fatalf("state %s with period end %v", got.State, got.PeriodEndsAt)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

// The provider's events come in any order and more than once: applying the snapshots that the
// re-reads return, in any order, ends in the state of the newest one.
func TestDeriveIgnoresOrderAndDuplicates(t *testing.T) {
	newest := with(snapshot(ProviderActive), func(s *Snapshot) { s.PeriodEnd = periodB })
	start := Transition{State: StatePending, PlanID: planBasic, PriceID: priceBasicMonth}
	facts := Facts{Now: t0.Add(time.Hour), StartedAt: t0}
	for _, order := range [][]*Snapshot{
		{newest},
		{newest, newest, newest},
		{nil, newest, newest},
	} {
		cur := start
		for _, s := range order {
			cur, _ = Derive(cur, s, facts)
		}
		if cur.State != StateActive || !cur.PeriodEndsAt.Equal(periodB) {
			t.Errorf("after %d snapshots: %+v", len(order), cur)
		}
	}
}

func TestRoleChangeOf(t *testing.T) {
	for _, tc := range []struct {
		name         string
		prev, next   Transition
		revoke, give string
	}{
		{"granted when paid", Transition{State: StatePending, PlanID: planA}, active(planA, priceA1), "", planA},
		{"kept while canceling", active(planA, priceA1), Transition{State: StateCanceling, PlanID: planA}, "", ""},
		{"kept while past due", active(planA, priceA1), Transition{State: StatePastDue, PlanID: planA}, "", ""},
		{"revoked when lapsed", Transition{State: StatePastDue, PlanID: planA}, Transition{State: StateLapsed, PlanID: planA}, planA, ""},
		{"granted again after lapsing", Transition{State: StateLapsed, PlanID: planA}, active(planA, priceA1), "", planA},
		{"revoked when refunded", active(planA, priceA1), Transition{State: StateRevoked, PlanID: planA}, planA, ""},
		{"replaced on a change of plan", active(planA, priceA1), active(planB, priceB1), planA, planB},
		{"nothing for an interval change", active(planA, priceA1), active(planA, priceA2), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := RoleChangeOf(tc.prev, tc.next)
			if c.Revoke != tc.revoke || c.Grant != tc.give {
				t.Fatalf("%+v, want revoke %q grant %q", c, tc.revoke, tc.give)
			}
		})
	}
}

func TestClose(t *testing.T) {
	now := t0.Add(5 * day)
	if got, ok := Close(active(planA, priceA1), now); !ok || got.State != StateExpired || !got.PeriodEndsAt.Equal(now) {
		t.Errorf("active: %+v %v", got, ok)
	}
	if got, ok := Close(Transition{State: StatePending, PlanID: planA}, now); !ok || got.State != StateAbandoned || !got.PeriodEndsAt.IsZero() {
		t.Errorf("pending: %+v %v", got, ok)
	}
	if _, ok := Close(Transition{State: StateRevoked, PlanID: planA, PeriodEndsAt: periodA}, now); ok {
		t.Error("closed a final state")
	}
}

func TestDeadlineOf(t *testing.T) {
	if k, at, ok := DeadlineOf(Transition{State: StatePending}, t0); !ok || k != DeadlineCheckout || !at.Equal(t0.Add(CheckoutTimeout)) {
		t.Errorf("pending: %s %v %v", k, at, ok)
	}
	if k, at, ok := DeadlineOf(active(planA, priceA1), t0); !ok || k != DeadlinePeriodEnd || !at.Equal(periodA) {
		t.Errorf("active: %s %v %v", k, at, ok)
	}
	g := &Grace{FailedAt: failedTime, EndsAt: failedTime.Add(GracePeriod)}
	if k, at, ok := DeadlineOf(Transition{State: StatePastDue, Grace: g}, t0); !ok || k != DeadlineGrace || !at.Equal(g.EndsAt) {
		t.Errorf("past due: %s %v %v", k, at, ok)
	}
	for _, s := range []State{StateLapsed, StateExpired, StateRevoked, StateAbandoned} {
		if _, _, ok := DeadlineOf(Transition{State: s}, t0); ok {
			t.Errorf("%s has a deadline", s)
		}
	}
}

// The yearly price is the monthly one × 12, discounted, rounded down in minor units.
func TestYearlyAmount(t *testing.T) {
	for _, tc := range []struct {
		monthly  int64
		discount int
		want     int64
	}{
		{500, 0, 6000},
		{500, 10, 5400},
		{333, 15, 3396}, // 3996 × 0.85 = 3396.6
		{1, 90, 1},      // 12 × 0.1 = 1.2
		{999, 33, 8031}, // 11988 × 0.67 = 8031.96
	} {
		if got := YearlyAmount(tc.monthly, tc.discount); got != tc.want {
			t.Errorf("YearlyAmount(%d, %d) = %d, want %d", tc.monthly, tc.discount, got, tc.want)
		}
	}
	terms := PlanTerms{MonthlyAmountMinor: 500, AnnualDiscountPercent: 20}
	if terms.Amount(IntervalMonth) != 500 || terms.Amount(IntervalYear) != 4800 {
		t.Errorf("Amount = %d, %d", terms.Amount(IntervalMonth), terms.Amount(IntervalYear))
	}
}

func TestPlanTermsValidate(t *testing.T) {
	ok := PlanTerms{Name: "Supporter", Description: "Thanks\nfor the help", MonthlyAmountMinor: 500, AnnualDiscountPercent: 10,
		EmojiLimit: 5, Rank: 1, Visibility: VisibilityPublic}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid terms: %v", err)
	}
	for _, tc := range []struct {
		field PlanField
		edit  func(*PlanTerms)
	}{
		{FieldName, func(p *PlanTerms) { p.Name = "" }},
		{FieldName, func(p *PlanTerms) { p.Name = " padded" }},
		{FieldName, func(p *PlanTerms) { p.Name = "two\nlines" }},
		{FieldName, func(p *PlanTerms) { p.Name = strings.Repeat("あ", MaxPlanNameLength+1) }},
		{FieldDescription, func(p *PlanTerms) { p.Description = strings.Repeat("a", MaxPlanDescriptionLength+1) }},
		{FieldDescription, func(p *PlanTerms) { p.Description = "bell\a" }},
		{FieldAmount, func(p *PlanTerms) { p.MonthlyAmountMinor = 0 }},
		{FieldAmount, func(p *PlanTerms) { p.MonthlyAmountMinor = MaxMonthlyAmountMinor + 1 }},
		{FieldDiscount, func(p *PlanTerms) { p.AnnualDiscountPercent = -1 }},
		{FieldDiscount, func(p *PlanTerms) { p.AnnualDiscountPercent = 91 }},
		{FieldEmojiLimit, func(p *PlanTerms) { p.EmojiLimit = MaxEmojiLimit + 1 }},
		{FieldRank, func(p *PlanTerms) { p.Rank = -1 }},
		{FieldVisibility, func(p *PlanTerms) { p.Visibility = "secret" }},
	} {
		terms := ok
		tc.edit(&terms)
		var pe *PlanError
		if err := terms.Validate(); !errors.As(err, &pe) || pe.Field != tc.field || !errors.Is(err, ErrInvalidPlan) {
			t.Errorf("%s: %v", tc.field, err)
		}
	}
	if ValidateCurrency("JPY") != nil || ValidateCurrency("jpy") == nil || ValidateCurrency("YEN!") == nil {
		t.Error("ValidateCurrency")
	}
}

// A higher plan includes the one below it: a higher position, a superset of the permissions, and
// at least the emoji limit when both create emoji (ADR-0072 §4).
func TestCheckTiers(t *testing.T) {
	basic := Tier{PlanID: planBasic, Rank: 1, Position: 10, Permissions: []string{"create"}, CreatesEmojis: true, EmojiLimit: 5}
	gold := Tier{PlanID: planGold, Rank: 2, Position: 20, Permissions: []string{"create", "view"}, CreatesEmojis: true, EmojiLimit: 10}
	if err := CheckTiers([]Tier{gold, basic}); err != nil {
		t.Fatalf("ordered tiers: %v", err)
	}
	if err := CheckTiers(nil); err != nil {
		t.Fatalf("no tiers: %v", err)
	}
	for _, tc := range []struct {
		rule TierRule
		edit func(*Tier)
	}{
		{TierRank, func(g *Tier) { g.Rank = 1 }},
		{TierPosition, func(g *Tier) { g.Position = 10 }},
		{TierPermissions, func(g *Tier) { g.Permissions = []string{"view"} }},
		{TierEmojiLimit, func(g *Tier) { g.EmojiLimit = 4 }},
	} {
		g := gold
		tc.edit(&g)
		var te *TierError
		if err := CheckTiers([]Tier{basic, g}); !errors.As(err, &te) || te.Rule != tc.rule || !errors.Is(err, ErrTiersOutOfOrder) {
			t.Errorf("%s: %v", tc.rule, err)
		}
	}
	// A lower plan without emoji creation sets no floor.
	noEmoji := basic
	noEmoji.Permissions, noEmoji.CreatesEmojis = nil, false
	g := gold
	g.EmojiLimit = 0
	if err := CheckTiers([]Tier{noEmoji, g}); err != nil {
		t.Errorf("lower plan without emoji: %v", err)
	}
}

// Upgrades apply at once with proration, downgrades and yearly to monthly at the period end; a
// change of both follows the rank (ADR-0072 §8).
func TestChangeTiming(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to Choice
		want     Timing
	}{
		{"higher plan", Choice{1, IntervalMonth}, Choice{2, IntervalMonth}, TimingNow},
		{"monthly to yearly", Choice{1, IntervalMonth}, Choice{1, IntervalYear}, TimingNow},
		{"lower plan", Choice{2, IntervalMonth}, Choice{1, IntervalMonth}, TimingPeriodEnd},
		{"yearly to monthly", Choice{1, IntervalYear}, Choice{1, IntervalMonth}, TimingPeriodEnd},
		{"higher monthly to lower yearly", Choice{2, IntervalMonth}, Choice{1, IntervalYear}, TimingPeriodEnd},
		{"lower yearly to higher monthly", Choice{1, IntervalYear}, Choice{2, IntervalMonth}, TimingNow},
	} {
		got, err := ChangeTiming(tc.from, tc.to)
		if err != nil || got != tc.want {
			t.Errorf("%s: %s %v, want %s", tc.name, got, err, tc.want)
		}
	}
	if _, err := ChangeTiming(Choice{1, IntervalMonth}, Choice{1, IntervalMonth}); !errors.Is(err, ErrSameChoice) {
		t.Errorf("same choice: %v", err)
	}
}

func TestEndReasonOf(t *testing.T) {
	for _, tc := range []struct {
		state  State
		reason Reason
		want   EndReason
	}{
		{StateLapsed, ReasonGraceExpired, EndLapsed},
		{StateLapsed, ReasonSynced, EndLapsed},
		{StateRevoked, ReasonReversed, EndReversed},
		{StateExpired, ReasonPlanRetired, EndPlanRetired},
		{StateExpired, ReasonAccountSuspended, EndAccountSuspended},
		{StateExpired, ReasonAccountClosed, EndAccountClosed},
		{StateExpired, ReasonStaff, EndStaff},
		{StateExpired, ReasonSynced, EndCanceled},
		{StateActive, ReasonSynced, ""},
		{StateAbandoned, ReasonPlanRetired, ""},
	} {
		if got := EndReasonOf(tc.state, tc.reason); got != tc.want {
			t.Errorf("EndReasonOf(%s, %s) = %q, want %q", tc.state, tc.reason, got, tc.want)
		}
	}
}
