// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"errors"
	"fmt"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

// The moderators' use cases: listing the domain actions (FR-702) and handling the reports in
// their queue (FR-701). The admin API and the CLI call them; they check the moderator's
// permissions before.

// ErrInvalidQuery reports a list query with an unknown filter value.
var ErrInvalidQuery = errors.New("moderation: invalid query")

// DomainActionView is a domain action with its lift and whether it is in effect now.
type DomainActionView struct {
	moderation.DomainActionRecord
	State moderation.DomainActionState
}

func (s *Service) view(r moderation.DomainActionRecord) DomainActionView {
	return DomainActionView{DomainActionRecord: r, State: r.State(s.d.Clock.Now())}
}

// DomainAction returns a domain action with its lift; it fails with ErrNotFound for an unknown
// ID.
func (s *Service) DomainAction(ctx context.Context, id string) (DomainActionView, error) {
	r, err := s.d.Repository.FindDomainActionRecord(ctx, id)
	if err != nil {
		return DomainActionView{}, err
	}
	return s.view(r), nil
}

// DomainActionFilter selects the domain actions that DomainActions lists.
type DomainActionFilter struct {
	// Domain keeps the actions on one canonical host (not its subdomains); "" keeps every
	// domain.
	Domain string
	// Active keeps the actions in effect now: neither lifted nor expired.
	Active bool
	// Before is the ID of the last action of the previous page; "" starts from the newest.
	Before string
}

// DomainActions lists up to limit domain actions, newest first.
func (s *Service) DomainActions(ctx context.Context, f DomainActionFilter, limit int) ([]DomainActionView, error) {
	q := DomainActionQuery{Domain: f.Domain, Before: f.Before}
	if f.Active {
		now := kernel.Normalize(s.d.Clock.Now())
		q.InEffectAt = &now
	}
	rs, err := s.d.Repository.ListDomainActions(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]DomainActionView, len(rs))
	for i, r := range rs {
		out[i] = s.view(r)
	}
	return out, nil
}

// Reports lists up to limit reports with their states, newest first. It fails with
// ErrInvalidQuery for an unknown state.
func (s *Service) Reports(ctx context.Context, q ReportQuery, limit int) ([]moderation.ReportRecord, error) {
	if q.State != "" && q.State != moderation.ReportOpen && q.State != moderation.ReportResolved {
		return nil, fmt.Errorf("%w: report state %q", ErrInvalidQuery, q.State)
	}
	return s.d.Repository.ListReports(ctx, q, limit)
}

// ReportRecord returns a report with its statuses and its state; it fails with ErrNotFound for
// an unknown ID.
func (s *Service) ReportRecord(ctx context.Context, id string) (moderation.ReportRecord, error) {
	r, err := s.d.Repository.FindReport(ctx, id)
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	st, err := s.d.Repository.FindReportStatus(ctx, id)
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	return moderation.ReportRecord{Report: r, Status: st}, nil
}

// AssignReport makes assigneeUserID the moderator who handles a report, or leaves it to nobody
// when assigneeUserID is "". It records the assignment and, in the same transaction, the
// report's new assignee in its state and the audit log entry; assigning the current assignee
// again records nothing.
// byUserID is the moderator who assigns it, "" for the CLI.
func (s *Service) AssignReport(ctx context.Context, reportID, assigneeUserID, byUserID string) (moderation.ReportRecord, error) {
	now := kernel.Normalize(s.d.Clock.Now())
	repo := s.d.Repository
	label, err := s.reportLabel(ctx, reportID)
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	assignmentID, auditID := s.d.IDs.NewID(), s.d.IDs.NewID()
	action := moderation.AuditReportAssigned
	if assigneeUserID == "" {
		action = moderation.AuditReportUnassigned
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		st, err := repo.ReportStatusForUpdate(ctx, tx, reportID)
		if err != nil || st.AssigneeUserID == assigneeUserID {
			return err
		}
		err = repo.InsertReportAssignment(ctx, tx, moderation.ReportAssignment{
			ID: assignmentID, ReportID: reportID, AssigneeUserID: assigneeUserID, AssignedByUserID: byUserID, AssignedAt: now,
		})
		if err != nil {
			return err
		}
		st.AssigneeUserID = assigneeUserID
		if err := repo.UpsertReportStatus(ctx, tx, st); err != nil {
			return err
		}
		return s.emitAudit(ctx, tx, moderation.AuditEntry{
			ID: auditID, Action: action, TargetKind: moderation.AuditTargetReport,
			TargetID: reportID, TargetLabel: label, SourceEventID: assignmentID, OccurredAt: now,
		}, byUserID)
	})
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	return s.ReportRecord(ctx, reportID)
}

// ResolveReport resolves an open report with outcome, recording the resolution and, in the same
// transaction, the report's resolved state and the audit log entry. Resolving a resolved report
// records nothing and keeps its outcome. It fails with moderation.ErrInvalidReport for an
// unknown outcome. byUserID is the moderator, "" for the CLI.
func (s *Service) ResolveReport(ctx context.Context, reportID string, outcome moderation.ReportOutcome, byUserID string) (moderation.ReportRecord, error) {
	if !outcome.Valid() {
		return moderation.ReportRecord{}, fmt.Errorf("%w: outcome %q", moderation.ErrInvalidReport, outcome)
	}
	now := kernel.Normalize(s.d.Clock.Now())
	repo := s.d.Repository
	label, err := s.reportLabel(ctx, reportID)
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	resolutionID, auditID := s.d.IDs.NewID(), s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		st, err := repo.ReportStatusForUpdate(ctx, tx, reportID)
		if err != nil || st.State == moderation.ReportResolved {
			return err
		}
		err = repo.InsertReportResolution(ctx, tx, moderation.ReportResolution{
			ID: resolutionID, ReportID: reportID, Outcome: outcome, ResolvedByUserID: byUserID, ResolvedAt: now,
		})
		if err != nil {
			return err
		}
		st.State, st.Outcome, st.Since = moderation.ReportResolved, outcome, now
		if err := repo.UpsertReportStatus(ctx, tx, st); err != nil {
			return err
		}
		return s.emitAudit(ctx, tx, moderation.AuditEntry{
			ID: auditID, Action: moderation.AuditReportResolved, TargetKind: moderation.AuditTargetReport,
			TargetID: reportID, TargetLabel: label, SourceEventID: resolutionID, OccurredAt: now,
		}, byUserID)
	})
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	return s.ReportRecord(ctx, reportID)
}

// reportLabel returns the display name of a report for the audit log: the handle of the account
// it reports. It fails with ErrNotFound for an unknown report.
func (s *Service) reportLabel(ctx context.Context, reportID string) (string, error) {
	r, err := s.d.Repository.FindReport(ctx, reportID)
	if err != nil {
		return "", err
	}
	return s.accountLabel(ctx, r.TargetAccountID)
}
