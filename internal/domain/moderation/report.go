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
	ReportID string
	State    ReportState
	// Outcome is that of the latest resolution while the report is resolved, "" while open.
	Outcome        ReportOutcome
	AssigneeUserID string // "" for none
	// Since is when the report entered its state: when it was filed or resolved.
	Since time.Time
}

// ReportOutcome is how a moderator resolved a report (report_resolutions.outcome).
type ReportOutcome string

// Report outcomes.
const (
	// OutcomeActioned: the moderator took an action on the target.
	OutcomeActioned ReportOutcome = "actioned"
	// OutcomeDismissed: the moderator found nothing to act on.
	OutcomeDismissed ReportOutcome = "dismissed"
)

// Valid reports whether o is one of the report outcomes.
func (o ReportOutcome) Valid() bool { return o == OutcomeActioned || o == OutcomeDismissed }

// ReportAssignment is the event that a moderator took a report, or gave it up
// (report_assignments).
type ReportAssignment struct {
	ID       string
	ReportID string
	// AssigneeUserID is the moderator who handles the report; "" gives it up.
	AssigneeUserID   string
	AssignedByUserID string // "" for the CLI
	AssignedAt       time.Time
}

// ReportResolution is the event that a moderator resolved a report (report_resolutions).
type ReportResolution struct {
	ID               string
	ReportID         string
	Outcome          ReportOutcome
	ResolvedByUserID string // "" for the CLI
	ResolvedAt       time.Time
}

// ReportRecord is a report with its current state, as the moderators list it.
type ReportRecord struct {
	Report Report
	Status ReportStatus
}
