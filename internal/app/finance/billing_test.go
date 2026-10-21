// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/best-friends/my/internal/domain/finance"
)

func date(t *testing.T, s string) finance.Date {
	t.Helper()
	d, err := finance.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// The previous month is fetched through the 15th, and after it while it is not final.
func TestRefetchMonths(t *testing.T) {
	for _, tc := range []struct {
		today         string
		previousFinal bool
		want          []string
	}{
		{"2026-11-01", true, []string{"2026-10", "2026-11"}},
		{"2026-11-15", true, []string{"2026-10", "2026-11"}},
		{"2026-11-16", true, []string{"2026-11"}},
		{"2026-11-16", false, []string{"2026-10", "2026-11"}},
		{"2026-11-30", false, []string{"2026-10", "2026-11"}},
		{"2027-01-10", true, []string{"2026-12", "2027-01"}},
	} {
		var got []string
		for _, m := range RefetchMonths(date(t, tc.today), tc.previousFinal) {
			got = append(got, m.String())
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("RefetchMonths(%s, %v) = %v, want %v", tc.today, tc.previousFinal, got, tc.want)
		}
	}
}

// A month without a mark of its own closes on the 5th of the next month.
func TestClosedByRule(t *testing.T) {
	october := finance.YearMonthOf(date(t, "2026-10-01"))
	for today, want := range map[string]bool{"2026-10-31": false, "2026-11-04": false, "2026-11-05": true, "2027-01-01": true} {
		if got := ClosedByRule(october, date(t, today)); got != want {
			t.Errorf("ClosedByRule(2026-10, %s) = %v, want %v", today, got, want)
		}
	}
}

func TestImportErrorKindOf(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want ImportErrorKind
	}{
		{nil, ImportErrorNone},
		{fmt.Errorf("x: %w", ErrRequestCap), ImportErrorRequestCap},
		{fmt.Errorf("x: %w", ErrBytesCap), ImportErrorBytesCap},
		{fmt.Errorf("x: %w", ErrBillingThrottled), ImportErrorThrottled},
		{errors.New("network"), ImportErrorUnavailable},
	} {
		if got := ImportErrorKindOf(tc.err); got != tc.want {
			t.Errorf("ImportErrorKindOf(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
	if !ImportErrorThrottled.Retried() || !ImportErrorUnavailable.Retried() || ImportErrorRequestCap.Retried() || ImportErrorPermission.Retried() {
		t.Error("Retried does not pass exactly throttled and unavailable")
	}
}
