// SPDX-License-Identifier: AGPL-3.0-only

package supporters

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The attributes of a plan (ADR-0072 §3).
const (
	// MaxPlanNameLength and MaxPlanDescriptionLength bound the name and the description, in
	// characters.
	MaxPlanNameLength        = 50
	MaxPlanDescriptionLength = 1000
	// MaxAnnualDiscountPercent bounds the discount of the yearly price.
	MaxAnnualDiscountPercent = 90
	// MaxEmojiLimit bounds the number of custom emoji that a plan lets its members create.
	MaxEmojiLimit = 10000
	// MaxMonthlyAmountMinor bounds the monthly amount, in minor units, far above what a provider
	// charges, so that the yearly price never overflows.
	MaxMonthlyAmountMinor int64 = 1_000_000_000_000
	// MaxRank bounds the rank of a plan.
	MaxRank = 1_000_000
)

// Visibility is whether a plan takes new subscriptions (supporter_plan_revisions.visibility).
type Visibility string

// The visibilities.
const (
	// VisibilityPublic: the plan is offered and takes new subscriptions and changes to it.
	VisibilityPublic Visibility = "public"
	// VisibilityHidden: the plan is not offered; its subscriptions continue (ADR-0072 §9).
	VisibilityHidden Visibility = "hidden"
)

// Valid reports whether v is one of the visibilities.
func (v Visibility) Valid() bool { return v == VisibilityPublic || v == VisibilityHidden }

// Interval is how often a price renews (supporter_plan_prices.interval).
type Interval string

// The intervals; every plan has a price of each (FR-911).
const (
	IntervalMonth Interval = "month"
	IntervalYear  Interval = "year"
)

// Intervals returns every interval, monthly first.
func Intervals() []Interval { return []Interval{IntervalMonth, IntervalYear} }

// Valid reports whether i is one of the intervals.
func (i Interval) Valid() bool { return i == IntervalMonth || i == IntervalYear }

// ErrInvalidPlan is what every PlanError matches.
var ErrInvalidPlan = errors.New("supporters: the plan breaks a rule")

// PlanField names the attribute of a plan that breaks a rule.
type PlanField string

// The fields of a plan.
const (
	FieldName        PlanField = "name"
	FieldDescription PlanField = "description"
	FieldAmount      PlanField = "monthly_amount"
	FieldCurrency    PlanField = "currency"
	FieldDiscount    PlanField = "annual_discount_percent"
	FieldEmojiLimit  PlanField = "emoji_limit"
	FieldRank        PlanField = "rank"
	FieldVisibility  PlanField = "visibility"
)

// PlanError reports the attribute of a plan that breaks a rule.
type PlanError struct {
	Field PlanField
}

func (e *PlanError) Error() string { return fmt.Sprintf("supporters: invalid plan %s", e.Field) }

// Is makes the error match ErrInvalidPlan.
func (e *PlanError) Is(target error) bool { return target == ErrInvalidPlan }

// PlanTerms are the attributes of a plan that its revisions change (supporter_plan_revisions).
type PlanTerms struct {
	Name        string
	Description string
	// MonthlyAmountMinor is the monthly price in minor units of the plan's currency.
	MonthlyAmountMinor    int64
	AnnualDiscountPercent int
	EmojiLimit            int
	Rank                  int
	Visibility            Visibility
}

// Validate checks t: a name of 1 to MaxPlanNameLength characters on one line without control
// characters, a description of at most MaxPlanDescriptionLength characters, a monthly amount of
// at least 1, a discount from 0 to MaxAnnualDiscountPercent, an emoji limit from 0 to
// MaxEmojiLimit, a rank from 0 to MaxRank, and a known visibility. The provider's minimum
// amount is checked by the caller. It returns a *PlanError.
func (t PlanTerms) Validate() error {
	invalid := func(f PlanField) error { return &PlanError{Field: f} }
	name := strings.TrimSpace(t.Name)
	switch {
	case name == "" || name != t.Name || utf8.RuneCountInString(t.Name) > MaxPlanNameLength || !utf8.ValidString(t.Name) ||
		strings.ContainsFunc(t.Name, unicode.IsControl):
		return invalid(FieldName)
	case !utf8.ValidString(t.Description) || utf8.RuneCountInString(t.Description) > MaxPlanDescriptionLength ||
		strings.ContainsFunc(t.Description, func(r rune) bool { return unicode.IsControl(r) && r != '\n' }):
		return invalid(FieldDescription)
	case t.MonthlyAmountMinor < 1 || t.MonthlyAmountMinor > MaxMonthlyAmountMinor:
		return invalid(FieldAmount)
	case t.AnnualDiscountPercent < 0 || t.AnnualDiscountPercent > MaxAnnualDiscountPercent:
		return invalid(FieldDiscount)
	case t.EmojiLimit < 0 || t.EmojiLimit > MaxEmojiLimit:
		return invalid(FieldEmojiLimit)
	case t.Rank < 0 || t.Rank > MaxRank:
		return invalid(FieldRank)
	case !t.Visibility.Valid():
		return invalid(FieldVisibility)
	}
	return nil
}

// ValidateCurrency checks that code looks like an ISO 4217 code: three upper-case ASCII letters.
// Whether the provider takes it is the provider's to say (MinimumAmount).
func ValidateCurrency(code string) error {
	if len(code) != 3 {
		return &PlanError{Field: FieldCurrency}
	}
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return &PlanError{Field: FieldCurrency}
		}
	}
	return nil
}

// YearlyAmount returns the yearly price of a monthly amount with a discount in percent:
// ⌊monthly × 12 × (100 − discount) / 100⌋ in minor units (ADR-0072 §3).
func YearlyAmount(monthlyMinor int64, discountPercent int) int64 {
	return monthlyMinor * 12 * int64(100-discountPercent) / 100
}

// Amount returns the price of t for an interval.
func (t PlanTerms) Amount(i Interval) int64 {
	if i == IntervalYear {
		return YearlyAmount(t.MonthlyAmountMinor, t.AnnualDiscountPercent)
	}
	return t.MonthlyAmountMinor
}

// Plan is a plan (supporter_plans): the role it carries and the currency never change.
type Plan struct {
	ID       string
	RoleID   string
	Currency string
	// CreatedByUserID is "" for the CLI.
	CreatedByUserID string
	CreatedAt       time.Time
}

// PlanRevision is a version of a plan's terms (supporter_plan_revisions); the newest one is the
// plan's current terms.
type PlanRevision struct {
	ID     string
	PlanID string
	PlanTerms
	// RevisedByUserID is "" for the CLI.
	RevisedByUserID string
	RevisedAt       time.Time
}
