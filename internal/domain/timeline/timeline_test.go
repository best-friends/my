// SPDX-License-Identifier: AGPL-3.0-only

package timeline_test

import (
	"slices"
	"testing"
	"time"

	"github.com/mycroblog/my/internal/domain/timeline"
)

func TestHomeWindows(t *testing.T) {
	day := 24 * time.Hour
	want := []time.Duration{7 * day, 28 * day, 112 * day, 448 * day}
	if got := timeline.HomeWindows(); !slices.Equal(got, want) {
		t.Errorf("HomeWindows() = %v, want %v", got, want)
	}
}

func TestDomainAndParents(t *testing.T) {
	cases := []struct {
		domain string
		want   []string
	}{
		{"", nil},
		{"example", []string{"example"}},
		{"a.b.example", []string{"a.b.example", "b.example", "example"}},
	}
	for _, tc := range cases {
		if got := timeline.DomainAndParents(tc.domain); !slices.Equal(got, tc.want) {
			t.Errorf("DomainAndParents(%q) = %v, want %v", tc.domain, got, tc.want)
		}
	}
}
