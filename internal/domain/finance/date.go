// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidDate reports a calendar date that is not YYYY-MM-DD between 0001-01-01 and
// 9999-12-31, or a day that its month does not have.
var ErrInvalidDate = errors.New("finance: invalid date")

// Date is a calendar date without a time or a time zone (the logical type date, data-model
// §1.2): a payment day on a card statement or an invoice (ADR-0067 §5). The zero value is
// 1970-01-01. Dates compare with == and Compare.
type Date struct {
	// days counts the days since 1970-01-01.
	days int64
}

// The dates that a Date can hold, the range of the four-digit years of YYYY-MM-DD.
var (
	MinDate = mustDate(1, time.January, 1)
	MaxDate = mustDate(9999, time.December, 31)
)

const secondsPerDay = 24 * 60 * 60

// NewDate returns the date y-m-d; it fails with ErrInvalidDate for a day that the month does not
// have or a year outside 1 to 9999.
func NewDate(y int, m time.Month, d int) (Date, error) {
	if y < 1 || y > 9999 || m < time.January || m > time.December || d < 1 || d > DaysIn(y, m) {
		return Date{}, fmt.Errorf("%w: %04d-%02d-%02d", ErrInvalidDate, y, int(m), d)
	}
	return Date{days: time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / secondsPerDay}, nil
}

func mustDate(y int, m time.Month, d int) Date {
	date, err := NewDate(y, m, d)
	if err != nil {
		panic(err)
	}
	return date
}

// ParseDate parses YYYY-MM-DD; it fails with ErrInvalidDate for anything else.
func ParseDate(s string) (Date, error) {
	if len(s) != len("2006-01-02") || s[4] != '-' || s[7] != '-' {
		return Date{}, fmt.Errorf("%w: %q", ErrInvalidDate, s)
	}
	y, ok1 := digits(s[0:4])
	m, ok2 := digits(s[5:7])
	d, ok3 := digits(s[8:10])
	if !ok1 || !ok2 || !ok3 {
		return Date{}, fmt.Errorf("%w: %q", ErrInvalidDate, s)
	}
	return NewDate(y, time.Month(m), d)
}

func digits(s string) (int, bool) {
	n := 0
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// DateOf returns the calendar date of t in t's location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{days: time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / secondsPerDay}
}

// DaysIn returns the number of days of month m of year y.
func DaysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// Time returns the start of the date in UTC.
func (d Date) Time() time.Time { return time.Unix(d.days*secondsPerDay, 0).UTC() }

// Date returns the year, month, and day of d.
func (d Date) Date() (year int, month time.Month, day int) { return d.Time().Date() }

// String returns d as YYYY-MM-DD.
func (d Date) String() string { return d.Time().Format(time.DateOnly) }

// AddDays returns the date n days after d (before it for a negative n).
func (d Date) AddDays(n int) Date { return Date{days: d.days + int64(n)} }

// Sub returns the number of days from o to d.
func (d Date) Sub(o Date) int { return int(d.days - o.days) }

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.days < o.days }

// After reports whether d is after o.
func (d Date) After(o Date) bool { return d.days > o.days }

// Compare returns -1, 0, or 1 as d is before, equal to, or after o.
func (d Date) Compare(o Date) int {
	switch {
	case d.days < o.days:
		return -1
	case d.days > o.days:
		return 1
	}
	return 0
}

// Weekday returns the day of the week of d.
func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

// ISOWeek returns the ISO 8601 year and week of d (weeks from Monday to Sunday).
func (d Date) ISOWeek() (year, week int) { return d.Time().ISOWeek() }

func minDate(a, b Date) Date {
	if a.days < b.days {
		return a
	}
	return b
}

func maxDate(a, b Date) Date {
	if a.days > b.days {
		return a
	}
	return b
}

// DateRange is the dates from Start to End, both included.
type DateRange struct {
	Start Date
	End   Date
}

// Valid reports whether the range holds at least one date.
func (r DateRange) Valid() bool { return !r.End.Before(r.Start) }

// Days returns the number of dates of the range, 0 for an invalid one.
func (r DateRange) Days() int {
	if !r.Valid() {
		return 0
	}
	return r.End.Sub(r.Start) + 1
}

// Contains reports whether d is in the range.
func (r DateRange) Contains(d Date) bool { return !d.Before(r.Start) && !d.After(r.End) }

// Intersect returns the dates in both r and o, and false when there are none.
func (r DateRange) Intersect(o DateRange) (DateRange, bool) {
	out := DateRange{Start: maxDate(r.Start, o.Start), End: minDate(r.End, o.End)}
	return out, out.Valid()
}

// String returns the range as START/END (ISO 8601 interval).
func (r DateRange) String() string { return r.Start.String() + "/" + r.End.String() }
