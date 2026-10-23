// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"testing"
	"time"
)

func TestSuspensionChange(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Minute), now.Add(time.Hour)
	row := func(r Restriction, action string, until *time.Time) RestrictionRow {
		return RestrictionRow{Target: testID(11), Restriction: r, SourceActionID: action, EffectiveUntil: until}
	}
	suspendA, suspendB := row(RestrictFederation, testID(1), nil), row(RestrictFederation, testID(2), &future)
	expiredA := row(RestrictFederation, testID(1), &past)
	login := row(RestrictLogin, testID(3), nil)
	for _, tc := range []struct {
		name               string
		before, after      []RestrictionRow
		changed, suspended bool
	}{
		{"a first suspension", []RestrictionRow{login}, []RestrictionRow{login, suspendA}, true, true},
		{"a second suspension", []RestrictionRow{suspendA}, []RestrictionRow{suspendA, suspendB}, false, true},
		{"a suspension over an expired one", []RestrictionRow{expiredA}, []RestrictionRow{expiredA, suspendB}, true, true},
		{"another restriction", nil, []RestrictionRow{login}, false, false},
		{"the lift of the only suspension", []RestrictionRow{suspendA, login}, []RestrictionRow{login}, true, false},
		{"the lift of one of two", []RestrictionRow{suspendA, suspendB}, []RestrictionRow{suspendB}, false, true},
		{"the removal of an expired suspension", []RestrictionRow{expiredA}, nil, true, false},
		{"the lift of another restriction", []RestrictionRow{login}, nil, false, false},
		{"nothing removed", []RestrictionRow{expiredA}, []RestrictionRow{expiredA}, false, false},
	} {
		changed, suspended := SuspensionChange(tc.before, tc.after, now)
		if changed != tc.changed || suspended != tc.suspended {
			t.Errorf("%s: SuspensionChange = %v, %v; want %v, %v", tc.name, changed, suspended, tc.changed, tc.suspended)
		}
	}
}
