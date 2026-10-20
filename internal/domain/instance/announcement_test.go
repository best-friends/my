// SPDX-License-Identifier: AGPL-3.0-only

package instance

import (
	"strings"
	"testing"
	"time"
)

func TestAnnouncementLabel(t *testing.T) {
	for body, want := range map[string]string{
		"# Maintenance\n\nOn Sunday.": "Maintenance",
		"\n\n  - first item\n":        "first item",
		"> quoted":                    "quoted",
		strings.Repeat("a", 100):      strings.Repeat("a", 79) + "…",
		" \n ":                        "",
	} {
		if got := AnnouncementLabel(body); got != want {
			t.Errorf("AnnouncementLabel(%q) = %q, want %q", body, got, want)
		}
	}
}

func TestAnnouncementHeadApply(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	h := AnnouncementHead{AnnouncementID: "a"}.Apply("r1", nil)
	if h.State != AnnouncementDraft || h.RevisionID != "r1" || !h.PublishedAt.IsZero() {
		t.Fatalf("after a revision: %+v, want a draft", h)
	}
	h = h.Apply("", &AnnouncementStateChange{State: AnnouncementPublished, ChangedAt: at})
	h = h.Apply("r0", nil) // an older revision changes nothing
	if h.State != AnnouncementPublished || h.RevisionID != "r1" || !h.PublishedAt.Equal(at) {
		t.Fatalf("after publishing: %+v", h)
	}
	h = h.Apply("", &AnnouncementStateChange{State: AnnouncementWithdrawn, ChangedAt: at.Add(time.Hour)})
	h = h.Apply("", &AnnouncementStateChange{State: AnnouncementPublished, ChangedAt: at.Add(2 * time.Hour)})
	if h.State != AnnouncementWithdrawn || !h.PublishedAt.Equal(at) {
		t.Errorf("after withdrawing: %+v, want withdrawn for good, published at %v", h, at)
	}
	// A draft withdrawn before any publication still has a time, as announcement_heads requires.
	d := AnnouncementHead{}.Apply("r1", &AnnouncementStateChange{State: AnnouncementWithdrawn, ChangedAt: at})
	if d.State != AnnouncementWithdrawn || !d.PublishedAt.Equal(at) {
		t.Errorf("withdrawn draft: %+v", d)
	}
}

func TestAnnouncementShowsAt(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	a := Announcement{State: AnnouncementPublished, PublishedAt: at, Revision: AnnouncementRevision{StartsAt: at.Add(time.Hour), EndsAt: at.Add(2 * time.Hour)}}
	for offset, want := range map[time.Duration]bool{0: false, time.Hour: true, 90 * time.Minute: true, 2 * time.Hour: false} {
		if got := a.ShowsAt(at.Add(offset)); got != want {
			t.Errorf("ShowsAt(+%v) = %v, want %v", offset, got, want)
		}
	}
	if !a.ShowsFrom().Equal(at.Add(time.Hour)) {
		t.Errorf("ShowsFrom = %v, want the start of the window", a.ShowsFrom())
	}
	a.State = AnnouncementDraft
	if a.ShowsAt(at.Add(time.Hour)) {
		t.Error("a draft shows")
	}
}
