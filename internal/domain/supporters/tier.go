// SPDX-License-Identifier: AGPL-3.0-only

package supporters

import (
	"errors"
	"fmt"
	"slices"
)

// A higher plan includes a lower one (ADR-0072 §4): a member holds the role of one plan only, so
// that the role of every plan carries everything the plans below it carry. Among the plans that
// are not retired, ordered by rank (a greater rank is a higher plan), each plan's role is above
// the role of the plan below it, has every permission of it, and, when both let their members
// create custom emoji, lets them create at least as many.

// Tier is what the order of the plans reads of a plan that is not retired and of its role.
type Tier struct {
	PlanID   string
	Rank     int
	Position int
	// Permissions are the role's permissions.
	Permissions []string
	// CreatesEmojis reports that the role lets its holders create custom emoji (ADR-0070 §4), and
	// EmojiLimit how many.
	CreatesEmojis bool
	EmojiLimit    int
}

// ErrTiersOutOfOrder is what every TierError matches.
var ErrTiersOutOfOrder = errors.New("supporters: a higher plan must include the plan below it")

// TierRule is the rule of the order of the plans that a pair breaks.
type TierRule string

// The rules.
const (
	// TierRank: two plans have the same rank.
	TierRank TierRule = "rank"
	// TierPosition: the higher plan's role is not above the lower plan's.
	TierPosition TierRule = "position"
	// TierPermissions: the higher plan's role lacks a permission of the lower plan's.
	TierPermissions TierRule = "permissions"
	// TierEmojiLimit: the higher plan lets its members create fewer custom emoji.
	TierEmojiLimit TierRule = "emoji_limit"
)

// TierError reports the pair of plans that breaks a rule.
type TierError struct {
	Rule          TierRule
	Lower, Higher string
}

func (e *TierError) Error() string {
	return fmt.Sprintf("supporters: plan %s above plan %s breaks the rule %s", e.Higher, e.Lower, e.Rule)
}

// Is makes the error match ErrTiersOutOfOrder.
func (e *TierError) Is(target error) bool { return target == ErrTiersOutOfOrder }

// CheckTiers checks the order of the plans that are not retired. It returns a *TierError for
// the first pair, from the lowest, that breaks a rule.
func CheckTiers(tiers []Tier) error {
	sorted := slices.Clone(tiers)
	slices.SortStableFunc(sorted, func(a, b Tier) int { return a.Rank - b.Rank })
	for i := 1; i < len(sorted); i++ {
		lo, hi := sorted[i-1], sorted[i]
		fail := func(r TierRule) error { return &TierError{Rule: r, Lower: lo.PlanID, Higher: hi.PlanID} }
		switch {
		case lo.Rank == hi.Rank:
			return fail(TierRank)
		case hi.Position <= lo.Position:
			return fail(TierPosition)
		case slices.ContainsFunc(lo.Permissions, func(p string) bool { return !slices.Contains(hi.Permissions, p) }):
			return fail(TierPermissions)
		case lo.CreatesEmojis && hi.CreatesEmojis && hi.EmojiLimit < lo.EmojiLimit:
			return fail(TierEmojiLimit)
		}
	}
	return nil
}

// Timing is when a change of plan applies (ADR-0072 §8).
type Timing string

// The timings.
const (
	// TimingNow: at once, with the prorated difference charged at once (an upgrade).
	TimingNow Timing = "now_prorated"
	// TimingPeriodEnd: at the end of the current period (a downgrade).
	TimingPeriodEnd Timing = "period_end"
)

// ErrSameChoice reports a change to the plan and the interval that a subscription has.
var ErrSameChoice = errors.New("supporters: the subscription has the plan and the interval already")

// Choice is a plan, by its rank, and an interval.
type Choice struct {
	Rank     int
	Interval Interval
}

// ChangeTiming returns when a change from one choice to another applies: a higher plan, or the
// yearly price of the same plan, at once; a lower plan, or the monthly price of the same plan,
// at the end of the period. A change of both is decided by the rank: from a higher monthly
// plan to a lower yearly one is a downgrade. It fails with ErrSameChoice for no change.
func ChangeTiming(from, to Choice) (Timing, error) {
	switch {
	case from == to:
		return "", ErrSameChoice
	case to.Rank > from.Rank:
		return TimingNow, nil
	case to.Rank < from.Rank:
		return TimingPeriodEnd, nil
	case to.Interval == IntervalYear:
		return TimingNow, nil
	default:
		return TimingPeriodEnd, nil
	}
}
