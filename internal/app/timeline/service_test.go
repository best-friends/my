// SPDX-License-Identifier: AGPL-3.0-only

package timeline_test

import (
	"context"
	"slices"
	"testing"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	apptimeline "github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/domain/timeline"
)

// leakyRepository returns whatever rows the test gives it, as a repository whose SQL forgot
// part of the visibility policy would, so that the tests see what CanView removes.
type leakyRepository struct {
	apptimeline.Repository
	rows  []apptimeline.Entry
	facts apptimeline.Facts
	homes []apptimeline.HomeQuery
}

func (r *leakyRepository) ListLocal(context.Context, apptimeline.LocalQuery) ([]apptimeline.Entry, error) {
	return r.rows, nil
}

func (r *leakyRepository) ListHome(_ context.Context, q apptimeline.HomeQuery) ([]apptimeline.Entry, error) {
	r.homes = append(r.homes, q)
	return r.rows, nil
}

func (r *leakyRepository) Facts(context.Context, apptimeline.FactsQuery) (apptimeline.Facts, error) {
	return r.facts, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func post(id, author string, v appcontent.Visibility) apptimeline.Entry {
	return apptimeline.Entry{
		ID: id, Kind: timeline.KindPost, StatusID: id, AuthorAccountID: author, OriginalAuthorAccountID: author,
		Visibility: v, IsLocal: true, Language: "en", RevisionSeq: 1,
	}
}

func ids(p apptimeline.Page) []string {
	var out []string
	for _, e := range p.Entries {
		out = append(out, e.ID)
	}
	return out
}

func TestServiceReappliesCanView(t *testing.T) {
	repo := &leakyRepository{
		rows: []apptimeline.Entry{
			post("06", "blocker", appcontent.VisibilityPublic),
			post("05", "author", appcontent.VisibilityDirect),
			post("04", "author", appcontent.VisibilityPrivate),
			post("03", "author", appcontent.VisibilityUnlisted),
			post("02", "author", appcontent.VisibilityPublic),
			post("01", "muted", appcontent.VisibilityPublic),
		},
		facts: apptimeline.Facts{Exclusions: map[string][]timeline.ExclusionReason{
			"blocker": {timeline.ExcludedBlockedBy}, "muted": {timeline.ExcludedMute},
		}},
	}
	svc := apptimeline.NewService(apptimeline.Deps{Repository: repo, Clock: fixedClock{time.Now()}})
	viewer := appcontent.Viewer{Kind: appcontent.ViewerLocal, AccountID: "viewer"}
	p, err := svc.Local(t.Context(), viewer, apptimeline.Query{Limit: 10})
	if err != nil {
		t.Fatalf("Local: %v", err)
	}
	if got, want := ids(p), []string{"02"}; !slices.Equal(got, want) {
		t.Errorf("Local = %v, want %v", got, want)
	}
	// The page is cut before CanView, so the cursor follows the rows read, not the rows kept.
	p, err = svc.Local(t.Context(), viewer, apptimeline.Query{Limit: 3})
	if err != nil {
		t.Fatalf("Local: %v", err)
	}
	if !slices.Equal(ids(p), nil) || p.Next != "04" {
		t.Errorf("Local(limit 3) = %v next %q, want no entries and next 04", ids(p), p.Next)
	}
}

func TestServiceChecksBoostedStatuses(t *testing.T) {
	boost := apptimeline.Entry{
		ID: "02", Kind: timeline.KindBoost, StatusID: "01", AuthorAccountID: "followee", OriginalAuthorAccountID: "blocker",
		Visibility: appcontent.VisibilityPublic, IsLocal: true, Language: "en", RevisionSeq: 1,
	}
	repo := &leakyRepository{
		rows: []apptimeline.Entry{boost},
		facts: apptimeline.Facts{
			Followed:   map[string]bool{"followee": true},
			Exclusions: map[string][]timeline.ExclusionReason{"blocker": {timeline.ExcludedBlockedBy}},
		},
	}
	svc := apptimeline.NewService(apptimeline.Deps{Repository: repo, Clock: fixedClock{time.Now()}})
	viewer := appcontent.Viewer{Kind: appcontent.ViewerLocal, AccountID: "viewer"}
	p, err := svc.Home(t.Context(), viewer, apptimeline.Query{})
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if len(p.Entries) != 0 {
		t.Errorf("Home = %v, want the boost of a blocker's status left out", ids(p))
	}
	// An unfilled page widens the window through every bound, each older than the last, and
	// then drops the bound.
	var floors []string
	for _, q := range repo.homes {
		floors = append(floors, q.Floor)
	}
	ok := len(floors) == len(timeline.HomeWindows())+1 && floors[len(floors)-1] == ""
	for i := 1; ok && i < len(floors)-1; i++ {
		ok = floors[i] < floors[i-1]
	}
	if !ok {
		t.Errorf("Home floors = %v, want %d widening bounds and then none", floors, len(timeline.HomeWindows()))
	}
}
