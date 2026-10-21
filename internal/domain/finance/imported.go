// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"
)

// The imported costs (ADR-0068, FR-904): the usage charges of a cloud provider by billing month,
// service, and charge type, recorded as revisions whose newest one the totals use.

var (
	// ErrInvalidYearMonth reports a billing month that is not YYYY-MM between 0001-01 and 9999-12.
	ErrInvalidYearMonth = errors.New("finance: invalid billing month")
	// ErrInvalidDecimal reports an amount that is not a decimal number such as -12.345, or one
	// whose minor units do not fit in 64 bits.
	ErrInvalidDecimal = errors.New("finance: invalid decimal amount")
)

// YearMonth is a calendar month: a provider's billing month in the provider's time zone
// (imported_cost_revisions.billing_month, YYYY-MM).
type YearMonth struct {
	Year  int
	Month time.Month
}

// NewYearMonth returns the month m of year y; it fails with ErrInvalidYearMonth outside 0001-01
// to 9999-12.
func NewYearMonth(y int, m time.Month) (YearMonth, error) {
	if y < 1 || y > 9999 || m < time.January || m > time.December {
		return YearMonth{}, fmt.Errorf("%w: %04d-%02d", ErrInvalidYearMonth, y, int(m))
	}
	return YearMonth{Year: y, Month: m}, nil
}

// ParseYearMonth parses YYYY-MM; it fails with ErrInvalidYearMonth for anything else.
func ParseYearMonth(s string) (YearMonth, error) {
	if len(s) != len("2006-01") || s[4] != '-' {
		return YearMonth{}, fmt.Errorf("%w: %q", ErrInvalidYearMonth, s)
	}
	y, ok1 := digits(s[0:4])
	m, ok2 := digits(s[5:7])
	if !ok1 || !ok2 {
		return YearMonth{}, fmt.Errorf("%w: %q", ErrInvalidYearMonth, s)
	}
	return NewYearMonth(y, time.Month(m))
}

// YearMonthOf returns the month that holds d.
func YearMonthOf(d Date) YearMonth {
	y, m, _ := d.Date()
	return YearMonth{Year: y, Month: m}
}

// String returns m as YYYY-MM.
func (m YearMonth) String() string { return fmt.Sprintf("%04d-%02d", m.Year, int(m.Month)) }

// AddMonths returns the month n months after m (before it for a negative n).
func (m YearMonth) AddMonths(n int) YearMonth {
	i := m.Year*12 + int(m.Month) - 1 + n
	return YearMonth{Year: i / 12, Month: time.Month(i%12 + 1)}
}

// Compare returns -1, 0, or 1 as m is before, equal to, or after o.
func (m YearMonth) Compare(o YearMonth) int {
	a, b := m.Year*12+int(m.Month), o.Year*12+int(o.Month)
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Range returns the dates of the month.
func (m YearMonth) Range() DateRange {
	return DateRange{Start: mustDate(m.Year, m.Month, 1), End: mustDate(m.Year, m.Month, DaysIn(m.Year, m.Month))}
}

// ChargeType is the kind of an imported charge (imported_cost_revisions.charge_type): the
// provider's record types and cost types mapped by the source's adapter (ADR-0068 §6, §7).
type ChargeType string

// Charge types, in the order of the schema's CHECK.
const (
	ChargeUsage      ChargeType = "usage"
	ChargeCredit     ChargeType = "credit"
	ChargeRefund     ChargeType = "refund"
	ChargeTax        ChargeType = "tax"
	ChargeSupport    ChargeType = "support"
	ChargeFee        ChargeType = "fee"
	ChargeAdjustment ChargeType = "adjustment"
	ChargeOther      ChargeType = "other"
)

var chargeTypes = []ChargeType{
	ChargeUsage, ChargeCredit, ChargeRefund, ChargeTax, ChargeSupport, ChargeFee, ChargeAdjustment, ChargeOther,
}

// ChargeTypes returns the charge types in a stable order.
func ChargeTypes() []ChargeType { return slices.Clone(chargeTypes) }

// Valid reports whether t is a known charge type.
func (t ChargeType) Valid() bool { return slices.Contains(chargeTypes, t) }

// RevisionKind is what a revision of an imported cost records (imported_cost_revisions.kind,
// ADR-0068 §3).
type RevisionKind string

// Revision kinds, in the order of the schema's CHECK.
const (
	// KindEstimate is a value of a month that the provider has not closed.
	KindEstimate RevisionKind = "estimate"
	// KindFinal is the first value of a closed month.
	KindFinal RevisionKind = "final"
	// KindCorrection is a change of the value after the month closed.
	KindCorrection RevisionKind = "correction"
)

// RevisionKinds returns the revision kinds in a stable order.
func RevisionKinds() []RevisionKind { return []RevisionKind{KindEstimate, KindFinal, KindCorrection} }

// Closed reports whether k records a closed month: final or correction.
func (k RevisionKind) Closed() bool { return k == KindFinal || k == KindCorrection }

// NextRevisionKind returns the kind of a new revision of a key whose newest revision is of kind
// prev ("" for none), when the provider reports the month as final or not: a correction once the
// key was closed, final when the month closes, an estimate before.
func NextRevisionKind(prev RevisionKind, final bool) RevisionKind {
	switch {
	case prev.Closed():
		return KindCorrection
	case final:
		return KindFinal
	}
	return KindEstimate
}

// MinorUnitsOf rounds a decimal amount in major units (such as "39.1603300457" or "-0.005") to
// the minor unit of c, half to even (ADR-0068 §5, ADR-0067 §6). It fails with ErrInvalidDecimal
// for anything that is not a plain decimal or does not fit, and with ErrInvalidCurrency for an
// unknown currency.
func MinorUnitsOf(amount string, c Currency) (int64, error) {
	digits, ok := c.MinorUnits()
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrInvalidCurrency, c)
	}
	r, err := ParseDecimal(amount)
	if err != nil {
		return 0, err
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	n := roundHalfEven(r)
	if !n.IsInt64() {
		return 0, fmt.Errorf("%w: %q does not fit", ErrInvalidDecimal, amount)
	}
	return n.Int64(), nil
}

// Decimal returns the amount in major units with the digits of its currency, such as "12.34" for
// 1234 USD cents, "-0.05", or "1650" for 1650 JPY (ADR-0067 §9); an unknown currency has none.
func (m Money) Decimal() string {
	digits, _ := m.Currency.MinorUnits()
	neg := m.Minor < 0
	abs := new(big.Int).Abs(big.NewInt(m.Minor)).String()
	if digits > 0 {
		if len(abs) <= digits {
			abs = strings.Repeat("0", digits-len(abs)+1) + abs
		}
		abs = abs[:len(abs)-digits] + "." + abs[len(abs)-digits:]
	}
	if neg {
		return "-" + abs
	}
	return abs
}

// maxDecimalLength bounds the decimals that ParseDecimal reads.
const maxDecimalLength = 64

// ParseDecimal parses a plain decimal number: an optional sign, digits, and an optional fraction
// (no exponent). It fails with ErrInvalidDecimal otherwise.
func ParseDecimal(s string) (*big.Rat, error) {
	body := strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	whole, frac, _ := strings.Cut(body, ".")
	if len(s) > maxDecimalLength || (whole == "" && frac == "") || !allDigits(whole) || !allDigits(frac) ||
		(strings.Contains(body, ".") && frac == "") {
		return nil, fmt.Errorf("%w: %q", ErrInvalidDecimal, s)
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidDecimal, s)
	}
	return r, nil
}

func allDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// roundHalfEven rounds r to the nearest integer, ties to the even one.
func roundHalfEven(r *big.Rat) *big.Int {
	num, den := r.Num(), r.Denom()
	q, m := new(big.Int).QuoRem(num, den, new(big.Int)) // truncated toward zero
	twice := new(big.Int).Mul(new(big.Int).Abs(m), big.NewInt(2))
	switch c := twice.Cmp(den); {
	case c > 0, c == 0 && q.Bit(0) == 1:
		if num.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}
