// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestYearMonth(t *testing.T) {
	m, err := ParseYearMonth("2026-12")
	if err != nil {
		t.Fatal(err)
	}
	if m.String() != "2026-12" || m.AddMonths(1).String() != "2027-01" || m.AddMonths(-12).String() != "2025-12" ||
		m.AddMonths(-11).String() != "2026-01" {
		t.Errorf("months around %s: %s %s %s", m, m.AddMonths(1), m.AddMonths(-12), m.AddMonths(-11))
	}
	if r := (YearMonth{Year: 2024, Month: time.February}).Range(); r.String() != "2024-02-01/2024-02-29" {
		t.Errorf("Range of 2024-02 = %s", r)
	}
	if YearMonthOf(date(t, "2026-03-31")) != (YearMonth{Year: 2026, Month: time.March}) {
		t.Error("YearMonthOf(2026-03-31) is not 2026-03")
	}
	if m.Compare(m.AddMonths(1)) != -1 || m.AddMonths(1).Compare(m) != 1 {
		t.Error("Compare does not order the months")
	}
	for _, s := range []string{"2026-13", "2026-00", "0000-01", "2026-1", "2026/01", "abcd-ef", ""} {
		if _, err := ParseYearMonth(s); !errors.Is(err, ErrInvalidYearMonth) {
			t.Errorf("ParseYearMonth(%q) = %v, want ErrInvalidYearMonth", s, err)
		}
	}
}

func TestNextRevisionKind(t *testing.T) {
	for _, tc := range []struct {
		prev  RevisionKind
		final bool
		want  RevisionKind
	}{
		{"", false, KindEstimate},
		{"", true, KindFinal},
		{KindEstimate, false, KindEstimate},
		{KindEstimate, true, KindFinal},
		{KindFinal, true, KindCorrection},
		{KindFinal, false, KindCorrection},
		{KindCorrection, true, KindCorrection},
	} {
		if got := NextRevisionKind(tc.prev, tc.final); got != tc.want {
			t.Errorf("NextRevisionKind(%q, %v) = %q, want %q", tc.prev, tc.final, got, tc.want)
		}
	}
}

// Amounts round half to even, once per recorded line, to the minor unit of their currency.
func TestMinorUnitsOf(t *testing.T) {
	for _, tc := range []struct {
		amount   string
		currency Currency
		want     int64
	}{
		{"39.1603300457", "USD", 3916},
		{"0.005", "USD", 0},
		{"0.015", "USD", 2},
		{"0.0150001", "USD", 2},
		{"-0.005", "USD", 0},
		{"-0.015", "USD", -2},
		{"-0.0151", "USD", -2},
		{"-0.0251", "USD", -3},
		{"12", "USD", 1200},
		{"+1.5", "JPY", 2},
		{"2.5", "JPY", 2},
		{"1234.5678", "BHD", 1234568},
		{"0", "USD", 0},
		{".5", "USD", 50},
	} {
		got, err := MinorUnitsOf(tc.amount, tc.currency)
		if err != nil || got != tc.want {
			t.Errorf("MinorUnitsOf(%q, %s) = %d, %v, want %d", tc.amount, tc.currency, got, err, tc.want)
		}
	}
	for _, s := range []string{"", "-", "1.", "1e3", "1,000", "0x10", "NaN", "１", "99999999999999999999999"} {
		if _, err := MinorUnitsOf(s, "USD"); !errors.Is(err, ErrInvalidDecimal) {
			t.Errorf("MinorUnitsOf(%q) = %v, want ErrInvalidDecimal", s, err)
		}
	}
	if _, err := MinorUnitsOf("1", "XXX"); !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("MinorUnitsOf in XXX = %v, want ErrInvalidCurrency", err)
	}
}

func TestMoneyDecimal(t *testing.T) {
	for _, tc := range []struct {
		m    Money
		want string
	}{
		{Money{Minor: 1234, Currency: "USD"}, "12.34"},
		{Money{Minor: 5, Currency: "USD"}, "0.05"},
		{Money{Minor: -5, Currency: "USD"}, "-0.05"},
		{Money{Minor: 0, Currency: "USD"}, "0.00"},
		{Money{Minor: 1650, Currency: "JPY"}, "1650"},
		{Money{Minor: -1234567, Currency: "BHD"}, "-1234.567"},
		{Money{Minor: math.MinInt64, Currency: "USD"}, "-92233720368547758.08"},
	} {
		if got := tc.m.Decimal(); got != tc.want {
			t.Errorf("%+v.Decimal() = %q, want %q", tc.m, got, tc.want)
		}
	}
}

// An imported cost spreads by day over its billing period under both bases, and marks the periods
// it meets as estimated while its month is open.
func TestTotalsWithImportedCosts(t *testing.T) {
	april := YearMonth{Year: 2026, Month: time.April}.Range()
	imported := []Entry{
		{Range: april, Amount: Money{Minor: 3000, Currency: "USD"}, Provider: "aws", Category: CategoryCompute},
		{
			Range: DateRange{Start: date(t, "2026-05-01"), End: date(t, "2026-05-10")}, Amount: Money{Minor: 1000, Currency: "USD"},
			Provider: "aws", Category: CategoryCompute, Estimated: true,
		},
	}
	for _, basis := range []Basis{BasisAmortized, BasisCash} {
		months, err := GranularityMonth.Periods(date(t, "2026-03-01"), date(t, "2026-06-30"))
		if err != nil {
			t.Fatal(err)
		}
		got := Totals(nil, imported, basis, months)
		want := []struct {
			usd       int64
			estimated bool
		}{{0, false}, {3000, false}, {1000, true}, {0, false}}
		for i, pt := range got {
			var usd int64
			for _, m := range pt.Totals {
				usd += m.Minor
			}
			if usd != want[i].usd || pt.Estimated != want[i].estimated {
				t.Errorf("%s %s: %d estimated %v, want %d estimated %v", basis, pt.Period.Label, usd, pt.Estimated, want[i].usd, want[i].estimated)
			}
		}
		weeks, err := GranularityWeek.Periods(april.Start, april.End)
		if err != nil {
			t.Fatal(err)
		}
		var sum int64
		for _, pt := range Totals(nil, imported, basis, weeks) {
			for _, l := range pt.Breakdown {
				if l.Key.Provider == "aws" && pt.Period.Range.End.Before(date(t, "2026-05-01")) {
					sum += l.Amount
				}
			}
		}
		// The weeks wholly in April carry 100 a day.
		if sum <= 0 || sum > 3000 {
			t.Errorf("%s: the April weeks carry %d of 3000", basis, sum)
		}
	}
}
