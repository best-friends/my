// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidBasis reports an unknown basis.
var ErrInvalidBasis = errors.New("finance: invalid basis")

// Basis is how the totals place a payment (ADR-0067 §4).
type Basis string

// Bases.
const (
	// BasisAmortized spreads each payment of a schedule longer than a month by day over the dates
	// it covers, and keeps the others on their payment dates. It is the default.
	BasisAmortized Basis = "amortized"
	// BasisCash places each payment on its payment date.
	BasisCash Basis = "cash"
)

// Validate fails with ErrInvalidBasis for an unknown basis.
func (b Basis) Validate() error {
	if b != BasisAmortized && b != BasisCash {
		return fmt.Errorf("%w: %q", ErrInvalidBasis, b)
	}
	return nil
}

// Entry is an amount to spread by day over a range of dates (Spread): a payment of a cost item
// on its date or over the dates it covers, or an imported cost over its billing period (§5).
type Entry struct {
	Range  DateRange
	Amount Money
	// Provider is the normalized provider that groups the breakdown, and Category its category.
	Provider string
	Category Category
	// Estimated marks an imported cost of a month that its provider has not closed (ADR-0068 §3).
	Estimated bool
}

// Entries returns the entries of the item's payments that meet window under basis: on the
// payment date, or over the covered dates when basis is amortized and the schedule amortizes.
func (c CostItem) Entries(basis Basis, window DateRange) []Entry {
	amortize := basis == BasisAmortized && c.Recurrence.Amortizes()
	var out []Entry
	for _, p := range c.Payments(window) {
		r := DateRange{Start: p.On, End: p.On}
		if amortize {
			r = p.Covers
		}
		if _, ok := r.Intersect(window); !ok {
			continue
		}
		out = append(out, Entry{Range: r, Amount: p.Price, Provider: c.ProviderNormalized, Category: c.Label.Category})
	}
	return out
}

// BreakdownKey groups the breakdown of a period: a provider, a category, and a currency.
type BreakdownKey struct {
	Provider string
	Category Category
	Currency Currency
}

// Line is an amount of a breakdown key.
type Line struct {
	Key    BreakdownKey
	Amount int64
}

// PeriodTotal is what a period carries: the total of each currency, by currency code, and the
// breakdown by provider, category, and currency, sorted by provider, category, and currency.
// A currency appears when an entry meets the period, even with a zero amount. Estimated reports
// that an estimated entry meets the period (ADR-0068 §3).
type PeriodTotal struct {
	Period    Period
	Totals    []Money
	Breakdown []Line
	Estimated bool
}

// Summarize spreads entries over periods (Spread) and totals each period by currency, never
// converting between currencies (ADR-0067 §6).
func Summarize(entries []Entry, periods []Period) []PeriodTotal {
	out := make([]PeriodTotal, len(periods))
	for i, p := range periods {
		totals := map[Currency]int64{}
		lines := map[BreakdownKey]int64{}
		estimated := false
		for _, e := range entries {
			if _, ok := e.Range.Intersect(p.Range); !ok {
				continue
			}
			estimated = estimated || e.Estimated
			amount := Spread(e.Amount.Minor, e.Range, p.Range)
			totals[e.Amount.Currency] += amount
			lines[BreakdownKey{Provider: e.Provider, Category: e.Category, Currency: e.Amount.Currency}] += amount
		}
		t := PeriodTotal{Period: p, Totals: make([]Money, 0, len(totals)), Breakdown: make([]Line, 0, len(lines)), Estimated: estimated}
		for c, amount := range totals {
			t.Totals = append(t.Totals, Money{Minor: amount, Currency: c})
		}
		slices.SortFunc(t.Totals, func(a, b Money) int { return cmp.Compare(a.Currency, b.Currency) })
		for k, amount := range lines {
			t.Breakdown = append(t.Breakdown, Line{Key: k, Amount: amount})
		}
		slices.SortFunc(t.Breakdown, func(a, b Line) int {
			return cmp.Or(cmp.Compare(a.Key.Provider, b.Key.Provider), cmp.Compare(a.Key.Category, b.Key.Category),
				cmp.Compare(a.Key.Currency, b.Key.Currency))
		})
		out[i] = t
	}
	return out
}

// Totals returns the totals of items under basis over periods: the entries of the items' payments
// that meet the periods, summarized with the entries of the imported costs, which every basis
// spreads by day over their billing periods (ADR-0067 §5).
func Totals(items []CostItem, imported []Entry, basis Basis, periods []Period) []PeriodTotal {
	if len(periods) == 0 {
		return nil
	}
	window := DateRange{Start: periods[0].Range.Start, End: periods[len(periods)-1].Range.End}
	var entries []Entry
	for _, c := range items {
		entries = append(entries, c.Entries(basis, window)...)
	}
	for _, e := range imported {
		if _, ok := e.Range.Intersect(window); ok {
			entries = append(entries, e)
		}
	}
	return Summarize(entries, periods)
}
