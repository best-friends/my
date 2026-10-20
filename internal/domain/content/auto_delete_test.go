// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/best-friends/my/internal/domain/content"
)

func TestAutoDeletePolicyNormalize(t *testing.T) {
	p := content.DefaultAutoDeletePolicy("a")
	p.KeepVisibilities = []content.Visibility{content.VisibilityDirect, content.VisibilityPublic, content.VisibilityDirect}
	got, err := p.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if want := []content.Visibility{content.VisibilityPublic, content.VisibilityDirect}; !slices.Equal(got.KeepVisibilities, want) {
		t.Errorf("KeepVisibilities = %v, want %v", got.KeepVisibilities, want)
	}
	for _, tt := range []struct {
		change   func(*content.AutoDeletePolicy)
		property string
	}{
		{func(p *content.AutoDeletePolicy) { p.MinAgeDays = 0 }, "min_age_days"},
		{func(p *content.AutoDeletePolicy) { p.MinAgeDays = content.MaxAutoDeleteAgeDays + 1 }, "min_age_days"},
		{func(p *content.AutoDeletePolicy) { p.KeepMinBoosts = -1 }, "keep_min_boosts"},
		{func(p *content.AutoDeletePolicy) { p.KeepMinFavourites = content.MaxAutoDeleteThreshold + 1 }, "keep_min_favourites"},
		{func(p *content.AutoDeletePolicy) { p.KeepVisibilities = []content.Visibility{"secret"} }, "keep_visibilities"},
	} {
		p := content.DefaultAutoDeletePolicy("a")
		tt.change(&p)
		_, err := p.Normalize()
		var perr *content.AutoDeletePolicyError
		if !errors.As(err, &perr) || perr.Property != tt.property {
			t.Errorf("Normalize(%+v) = %v, want an error on %s", p, err, tt.property)
		}
	}
}

func TestAutoDeletePolicyCutoff(t *testing.T) {
	p := content.AutoDeletePolicy{MinAgeDays: 7}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if got, want := p.Cutoff(now), time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Cutoff = %v, want %v", got, want)
	}
}
