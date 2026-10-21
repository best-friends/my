// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidRecurrence reports an unknown schedule or an interval outside its range.
var ErrInvalidRecurrence = errors.New("finance: invalid recurrence")

// Schedule is the unit of a cost item's payment schedule (cost_items.schedule, ADR-0067 §3).
type Schedule string

// Schedules.
const (
	// ScheduleOnce is one payment on the start date.
	ScheduleOnce Schedule = "once"
	// ScheduleWeeks pays every N weeks; there is no schedule in days.
	ScheduleWeeks  Schedule = "weeks"
	ScheduleMonths Schedule = "months"
	ScheduleYears  Schedule = "years"
)

// MaxInterval is the largest N of an every-N schedule.
const MaxInterval = 99

// Recurrence is the payment schedule of a cost item: once, or every Interval weeks, months, or
// years. It is a fact of the item that never changes (ADR-0067 §2).
type Recurrence struct {
	Schedule Schedule
	// Interval is N of every N weeks, months, or years; 1 for once.
	Interval int
}

// Validate fails with ErrInvalidRecurrence for an unknown schedule, an interval of once other
// than 1, or an interval of the other schedules outside 1 to MaxInterval.
func (r Recurrence) Validate() error {
	switch r.Schedule {
	case ScheduleOnce:
		if r.Interval == 1 {
			return nil
		}
	case ScheduleWeeks, ScheduleMonths, ScheduleYears:
		if r.Interval >= 1 && r.Interval <= MaxInterval {
			return nil
		}
	default:
		return fmt.Errorf("%w: schedule %q", ErrInvalidRecurrence, r.Schedule)
	}
	return fmt.Errorf("%w: %s every %d", ErrInvalidRecurrence, r.Schedule, r.Interval)
}

// Amortizes reports whether the amortized basis spreads each payment over the days it covers:
// only for schedules longer than a month (months with N ≥ 2, years, weeks with N ≥ 5; ADR-0067
// §4). Once and the schedules of a month or shorter stay on their payment dates.
func (r Recurrence) Amortizes() bool {
	switch r.Schedule {
	case ScheduleMonths:
		return r.Interval >= 2
	case ScheduleYears:
		return true
	case ScheduleWeeks:
		return r.Interval >= 5
	}
	return false
}

// Occurrence returns the date of payment k (k = 0, 1, …) of a schedule that starts on start,
// counted from the start date and not from the payment before it (ADR-0067 §3): weeks add 7·N·k
// days; months take the start's day N·k months later, or the last day of a month without it; years
// take the same month and day N·k years later, February 28 for February 29 in a common year.
// It returns false past MaxDate, for k > 0 of once, and for a negative k.
func (r Recurrence) Occurrence(start Date, k int) (Date, bool) {
	if k < 0 {
		return Date{}, false
	}
	if k == 0 {
		return start, true
	}
	y, m, d := start.Date()
	switch r.Schedule {
	case ScheduleWeeks:
		out := start.AddDays(7 * r.Interval * k)
		return out, !out.After(MaxDate)
	case ScheduleMonths:
		months := int(m-time.January) + r.Interval*k
		return dayOrLast(y+months/12, time.January+time.Month(months%12), d)
	case ScheduleYears:
		return dayOrLast(y+r.Interval*k, m, d)
	}
	return Date{}, false
}

// dayOrLast returns y-m-d, or the last day of the month when it has fewer days, and false past
// year 9999.
func dayOrLast(y int, m time.Month, d int) (Date, bool) {
	if y > 9999 {
		return Date{}, false
	}
	date, err := NewDate(y, m, min(d, DaysIn(y, m)))
	return date, err == nil
}

// firstCovering returns a k at or before the first payment whose covered dates can reach from: a
// lower bound that skips the payments far before it.
func (r Recurrence) firstCovering(start, from Date) int {
	if !from.After(start) {
		return 0
	}
	var k int
	switch r.Schedule {
	case ScheduleWeeks:
		k = from.Sub(start)/(7*r.Interval) - 1
	case ScheduleMonths:
		sy, sm, _ := start.Date()
		fy, fm, _ := from.Date()
		k = ((fy-sy)*12+int(fm-sm))/r.Interval - 1
	case ScheduleYears:
		sy, _, _ := start.Date()
		fy, _, _ := from.Date()
		k = (fy-sy)/r.Interval - 1
	}
	return max(k, 0)
}
