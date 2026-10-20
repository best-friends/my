// SPDX-License-Identifier: AGPL-3.0-only

package instance

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// The server-wide announcements (FR-726, data-model §4.9): administrators write one in Markdown,
// publish it for every local user within an optional window, and withdraw it; each user marks
// the ones they have read.

// MaxAnnouncementBodyLength bounds the Markdown of an announcement, in characters.
const MaxAnnouncementBodyLength = 5000

var (
	// ErrAnnouncementBody reports a body that is blank or not valid text. It wraps
	// ErrInvalidValue.
	ErrAnnouncementBody = fmt.Errorf("instance: invalid announcement body: %w", ErrInvalidValue)
	// ErrAnnouncementTooLong reports a body longer than MaxAnnouncementBodyLength. It wraps
	// ErrInvalidValue.
	ErrAnnouncementTooLong = fmt.Errorf("instance: announcement body too long: %w", ErrInvalidValue)
	// ErrAnnouncementWindow reports a window that ends before, or when, it starts. It wraps
	// ErrInvalidValue.
	ErrAnnouncementWindow = fmt.Errorf("instance: the announcement ends before it starts: %w", ErrInvalidValue)
	// ErrAnnouncementWithdrawn reports a change of a withdrawn announcement, which can no longer
	// change.
	ErrAnnouncementWithdrawn = errors.New("instance: announcement is withdrawn")
)

// AnnouncementState is where an announcement is in its life (announcement_heads.state): a draft
// that no user sees, published, or withdrawn for good.
type AnnouncementState string

// States of an announcement.
const (
	AnnouncementDraft     AnnouncementState = "draft"
	AnnouncementPublished AnnouncementState = "published"
	AnnouncementWithdrawn AnnouncementState = "withdrawn"
)

// AnnouncementRevision is one version of an announcement (announcement_revisions): its body and
// its window. The revision with the greatest ID is the current one.
type AnnouncementRevision struct {
	ID             string
	AnnouncementID string
	// Body is the announcement in Markdown, rendered to sanitized HTML for the readers.
	Body string
	// StartsAt and EndsAt bound when a published announcement shows; the zero time leaves that
	// side open (from its publication, until it is withdrawn).
	StartsAt time.Time
	EndsAt   time.Time
	// RevisedByUserID is the user who wrote the revision, or "" for the CLI.
	RevisedByUserID string
	RevisedAt       time.Time
}

// ValidateAnnouncement checks a body and a window: a non-blank body of valid UTF-8 without NUL
// and at most MaxAnnouncementBodyLength characters, and a window that, when it has both ends,
// ends after it starts.
func ValidateAnnouncement(body string, startsAt, endsAt time.Time) error {
	switch {
	case !utf8.ValidString(body), strings.ContainsRune(body, 0), strings.TrimSpace(body) == "":
		return ErrAnnouncementBody
	case utf8.RuneCountInString(body) > MaxAnnouncementBodyLength:
		return ErrAnnouncementTooLong
	case !startsAt.IsZero() && !endsAt.IsZero() && !endsAt.After(startsAt):
		return ErrAnnouncementWindow
	}
	return nil
}

// AnnouncementStateChange records a publication or a withdrawal (announcement_state_changes).
type AnnouncementStateChange struct {
	ID             string
	AnnouncementID string
	// State is AnnouncementPublished or AnnouncementWithdrawn.
	State AnnouncementState
	// ChangedByUserID is the user who made the change, or "" for the CLI.
	ChangedByUserID string
	ChangedAt       time.Time
}

// AnnouncementHead is the current revision and state of an announcement (the announcement_heads
// projection). PublishedAt is the time of its latest publication, the zero time for a draft.
type AnnouncementHead struct {
	AnnouncementID string
	RevisionID     string
	State          AnnouncementState
	PublishedAt    time.Time
}

// Apply returns the head after the revision or state change with the greatest ID, given h, which
// is the zero value for an announcement without a head yet. Revisions and state changes come in
// any order; the result is the same.
func (h AnnouncementHead) Apply(revisionID string, change *AnnouncementStateChange) AnnouncementHead {
	if h.State == "" {
		h.State = AnnouncementDraft
	}
	if revisionID != "" && revisionID > h.RevisionID {
		h.RevisionID = revisionID
	}
	if change != nil {
		switch {
		case h.State == AnnouncementWithdrawn:
			// A withdrawal is for good.
		case change.State == AnnouncementPublished:
			h.State, h.PublishedAt = AnnouncementPublished, change.ChangedAt
		case change.State == AnnouncementWithdrawn:
			h.State = AnnouncementWithdrawn
			if h.PublishedAt.IsZero() {
				h.PublishedAt = change.ChangedAt
			}
		}
	}
	return h
}

// Announcement is an announcement with its current revision and state.
type Announcement struct {
	ID        string
	CreatedAt time.Time
	Revision  AnnouncementRevision
	State     AnnouncementState
	// PublishedAt is the time of its latest publication, the zero time for a draft.
	PublishedAt time.Time
}

// ShowsFrom returns when a published announcement starts to show: its publication, or the start
// of its window when that comes later.
func (a Announcement) ShowsFrom() time.Time {
	if a.Revision.StartsAt.After(a.PublishedAt) {
		return a.Revision.StartsAt
	}
	return a.PublishedAt
}

// ShowsAt reports whether the users see the announcement at t: it is published, and t is within
// its window.
func (a Announcement) ShowsAt(t time.Time) bool {
	if a.State != AnnouncementPublished {
		return false
	}
	if !a.Revision.StartsAt.IsZero() && t.Before(a.Revision.StartsAt) {
		return false
	}
	return a.Revision.EndsAt.IsZero() || t.Before(a.Revision.EndsAt)
}

// AnnouncementLabel returns the display name of an announcement for the audit log: the first
// non-blank line of its body, at most 80 characters.
func AnnouncementLabel(body string) string {
	for line := range strings.Lines(body) {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#>*-+ "))
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > 80 {
			line = string([]rune(line)[:79]) + "…"
		}
		return line
	}
	return ""
}
