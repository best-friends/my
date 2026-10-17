// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// MaxReportComment is the number of characters of a report's comment that are kept; a received
// Flag's longer comment is cut (federation.md §11).
const MaxReportComment = 5000

// Report is a report of an account to the moderators (reports, a resource; FR-701).
type Report struct {
	ID string
	// URI is the id of the received Flag, or the URI synthesized for a Flag without one; "" for
	// a local report.
	URI               string
	ReporterAccountID string
	TargetAccountID   string
	Category          Category
	Comment           string
	// Forwarded reports that the report was forwarded to the target's server.
	Forwarded bool
	FiledAt   time.Time
	// StatusIDs are the statuses of the target that the report points at (report_statuses).
	StatusIDs []string
}

// ErrInvalidReport reports a report with a missing or invalid field.
var ErrInvalidReport = errors.New("moderation: invalid report")

// Validate checks the invariants of a new report.
func (r Report) Validate() error {
	switch {
	case r.ID == "" || r.ReporterAccountID == "" || r.TargetAccountID == "" || r.FiledAt.IsZero():
		return fmt.Errorf("%w: a field is empty", ErrInvalidReport)
	case !r.Category.Valid():
		return fmt.Errorf("%w: category %q", ErrInvalidReport, r.Category)
	case utf8.RuneCountInString(r.Comment) > MaxReportComment:
		return fmt.Errorf("%w: the comment is longer than %d characters", ErrInvalidReport, MaxReportComment)
	}
	return nil
}

// TruncateComment returns the first MaxReportComment characters of comment.
func TruncateComment(comment string) string {
	if utf8.RuneCountInString(comment) <= MaxReportComment {
		return comment
	}
	n := 0
	for i := range comment {
		if n == MaxReportComment {
			return comment[:i]
		}
		n++
	}
	return comment
}

// ReportState is whether a report awaits the moderators (report_states.state).
type ReportState string

// Report states.
const (
	ReportOpen     ReportState = "open"
	ReportResolved ReportState = "resolved"
)

// ReportStatus is the current state of a report (the report_states projection).
type ReportStatus struct {
	ReportID       string
	State          ReportState
	AssigneeUserID string // "" for none
	Since          time.Time
}
