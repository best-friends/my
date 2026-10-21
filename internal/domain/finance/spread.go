// SPDX-License-Identifier: AGPL-3.0-only

package finance

import "math/big"

// Spread returns the part of amount that falls on the dates of part when amount is spread by
// day over span, with cumulative rounding (ADR-0067 §4): the first d of the D days of span carry
// floor(amount·d / D) together, and part carries the difference of that sum at its end and at
// its start. However span is cut into parts, the parts sum to amount exactly. Imported costs
// spread their billing periods with it (§5); amount may be negative (credits and refunds). It
// returns 0 when part and span share no date.
func Spread(amount int64, span, part DateRange) int64 {
	in, ok := span.Intersect(part)
	if !ok {
		return 0
	}
	days := int64(span.Days())
	before := int64(in.Start.Sub(span.Start))
	through := int64(in.End.Sub(span.Start)) + 1
	return cumulative(amount, through, days) - cumulative(amount, before, days)
}

// cumulative returns floor(amount·d / days) for 0 ≤ d ≤ days. The product can pass the range of
// int64, so it is computed in big integers; the quotient lies between 0 and amount and fits.
func cumulative(amount, d, days int64) int64 {
	if d <= 0 {
		return 0
	}
	if d >= days {
		return amount
	}
	n := new(big.Int).Mul(big.NewInt(amount), big.NewInt(d))
	// Euclidean division by a positive divisor rounds toward negative infinity: the floor.
	return n.Div(n, big.NewInt(days)).Int64()
}
