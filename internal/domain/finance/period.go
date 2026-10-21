// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var (
	// ErrInvalidGranularity reports an unknown granularity.
	ErrInvalidGranularity = errors.New("finance: invalid granularity")
	// ErrInvalidPeriod reports a range whose end is before its start.
	ErrInvalidPeriod = errors.New("finance: invalid period")
	// ErrTooManyPeriods reports a range of more periods than Granularity.MaxPeriods.
	ErrTooManyPeriods = errors.New("finance: too many periods")
)

// Granularity is the length of the periods of the totals (ADR-0067 §5).
type Granularity string

// Granularities.
const (
	// GranularityWeek is the ISO 8601 week, Monday to Sunday.
	GranularityWeek Granularity = "week"
	// GranularityMonth and GranularityYear are the calendar months and years.
	GranularityMonth Granularity = "month"
	GranularityYear  Granularity = "year"
)

// MaxPeriods returns how many periods one query of the totals can return (ADR-0067 §9: 104
// weeks, 60 months, 20 years), 0 for an unknown granularity.
func (g Granularity) MaxPeriods() int {
	switch g {
	case GranularityWeek:
		return 104
	case GranularityMonth:
		return 60
	case GranularityYear:
		return 20
	}
	return 0
}

// Period is one week, month, or year of the totals, with its label: 2026-W17, 2026-04, or 2026.
type Period struct {
	Label string
	Range DateRange
}

// PeriodOf returns the week, month, or year that holds d.
func (g Granularity) PeriodOf(d Date) (Period, error) {
	y, m, _ := d.Date()
	switch g {
	case GranularityWeek:
		start := d.AddDays(-((int(d.Weekday()) + 6) % 7)) // back to Monday
		wy, w := d.ISOWeek()
		return Period{Label: fmt.Sprintf("%04d-W%02d", wy, w), Range: DateRange{Start: start, End: start.AddDays(6)}}, nil
	case GranularityMonth:
		start := mustDate(y, m, 1)
		return Period{Label: fmt.Sprintf("%04d-%02d", y, int(m)), Range: DateRange{Start: start, End: mustDate(y, m, DaysIn(y, m))}}, nil
	case GranularityYear:
		return Period{
			Label: strconv.Itoa(y), Range: DateRange{Start: mustDate(y, time.January, 1), End: mustDate(y, time.December, 31)},
		}, nil
	}
	return Period{}, fmt.Errorf("%w: %q", ErrInvalidGranularity, g)
}

// Periods returns, in order, the weeks, months, or years from the one that holds from to the one
// that holds to. Each granularity cuts the dates on its own, so a week across a new year falls in
// two years. It fails with ErrInvalidPeriod when to is before from and with ErrTooManyPeriods past
// MaxPeriods.
func (g Granularity) Periods(from, to Date) ([]Period, error) {
	if to.Before(from) {
		return nil, fmt.Errorf("%w: %s to %s", ErrInvalidPeriod, from, to)
	}
	limit := g.MaxPeriods()
	var out []Period
	for d := from; !d.After(to); {
		p, err := g.PeriodOf(d)
		if err != nil {
			return nil, err
		}
		if len(out) == limit {
			return nil, fmt.Errorf("%w: more than %d %ss", ErrTooManyPeriods, limit, g)
		}
		out = append(out, p)
		if !p.Range.End.Before(MaxDate) {
			break
		}
		d = p.Range.End.AddDays(1)
	}
	return out, nil
}
