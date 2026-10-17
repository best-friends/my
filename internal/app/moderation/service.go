// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

var (
	// ErrUnsupportedAction reports an action on an account that this module does not issue: a
	// kind that does not apply to the account (login on a remote account), one whose effects
	// this module does not carry out yet, or an action on the instance actor.
	ErrUnsupportedAction = errors.New("moderation: the action is not supported")
	// ErrInvalidReportReference reports an action that names a report that does not exist or
	// is of another account.
	ErrInvalidReportReference = errors.New("moderation: the report does not exist or is of another account")
	// ErrAccountNotFound reports an action or a report on an account that does not exist.
	ErrAccountNotFound = errors.New("moderation: the account does not exist")
)

// Report is moderation.Report, a report as other modules file it.
type Report = moderation.Report

// Effect is moderation.Effect, a restriction in effect as the events carry it.
type Effect = moderation.Effect

// Restrictions that other modules look for in the events.
const (
	// RestrictProfileView is moderation.RestrictProfileView.
	RestrictProfileView = moderation.RestrictProfileView
	// RestrictLogin is moderation.RestrictLogin.
	RestrictLogin = moderation.RestrictLogin
)

// Domain events of the moderation module.
const (
	// EventDomainRestrictionsChanged follows every action on a domain and every lift of one,
	// with the restrictions in effect on the domain afterwards.
	EventDomainRestrictionsChanged = "moderation.domain_restrictions_changed"
	// EventAccountRestrictionsChanged is EventDomainRestrictionsChanged for an account.
	EventAccountRestrictionsChanged = "moderation.account_restrictions_changed"
	// EventReportFiled follows a new report.
	EventReportFiled = "moderation.report_filed"
)

// DomainRestrictionsChanged carries the restrictions in effect on one domain (not its parents)
// after an action or a lift: subscribers replace what they keep of the domain with Effects.
type DomainRestrictionsChanged struct {
	Domain  string
	Effects []moderation.Effect
}

// EventType implements uow.Event.
func (DomainRestrictionsChanged) EventType() string { return EventDomainRestrictionsChanged }

// AccountRestrictionsChanged is DomainRestrictionsChanged for an account.
type AccountRestrictionsChanged struct {
	AccountID string
	Effects   []moderation.Effect
}

// EventType implements uow.Event.
func (AccountRestrictionsChanged) EventType() string { return EventAccountRestrictionsChanged }

// ReportFiled carries a new report.
type ReportFiled struct {
	Report moderation.Report
	// FirstOpen reports that no other report of the target was open when the report was filed:
	// the moderators are notified of the first open report of an account only (data-model §4.7).
	FirstOpen bool
}

// EventType implements uow.Event.
func (ReportFiled) EventType() string { return EventReportFiled }

// Accounts is the part of the account module's public API that the moderation module reads.
// *appaccount.Service implements it.
type Accounts interface {
	FindByID(ctx context.Context, id string) (appaccount.Actor, error)
}

// Deps are the ports a Service uses.
type Deps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	Accounts   Accounts
	Clock      kernel.Clock
	IDs        kernel.IDGenerator
}

// Service runs the moderation module's use cases: actions on domains and accounts, their lifts,
// and the reports received from other servers, and the moderators' lists of them and handling
// of the reports (admin.go).
type Service struct{ d Deps }

// NewService returns a Service on d.
func NewService(d Deps) *Service { return &Service{d: d} }

// DomainActionInput is a moderator's action on a domain.
type DomainActionInput struct {
	// Domain is a canonical host (federation.CanonicalHost); the action covers its subdomains.
	Domain        string
	Kind          moderation.DomainActionKind
	PublicComment string
	PrivateNote   string
	Obfuscate     bool
	// IssuedByUserID is the moderator; "" for the CLI.
	IssuedByUserID string
	// Duration is how long the action lasts; zero lasts until it is lifted.
	Duration time.Duration
}

// IssueDomainAction records an action on a domain and, in the same transaction, the
// restrictions it imposes on the domain and its subdomains (data-model §5.2), and emits
// DomainRestrictionsChanged.
func (s *Service) IssueDomainAction(ctx context.Context, in DomainActionInput) (moderation.DomainAction, error) {
	now := kernel.Normalize(s.d.Clock.Now())
	a := moderation.DomainAction{
		ID: s.d.IDs.NewID(), Domain: in.Domain, Kind: in.Kind, PublicComment: in.PublicComment,
		PrivateNote: in.PrivateNote, Obfuscate: in.Obfuscate, IssuedByUserID: in.IssuedByUserID,
		ExpiresAt: expiry(now, in.Duration), IssuedAt: now,
	}
	if err := a.Validate(); err != nil {
		return moderation.DomainAction{}, err
	}
	repo := s.d.Repository
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := repo.InsertDomainAction(ctx, tx, a); err != nil {
			return err
		}
		for _, r := range a.Rows() {
			if err := repo.InsertDomainRestriction(ctx, tx, r); err != nil {
				return err
			}
		}
		return s.emitDomain(ctx, tx, a.Domain, now)
	})
	if err != nil {
		return moderation.DomainAction{}, err
	}
	return a, nil
}

// LiftDomainAction lifts an action on a domain: its restrictions end, while those of other
// actions on the domain stay. Lifting an action that is already lifted changes nothing.
func (s *Service) LiftDomainAction(ctx context.Context, actionID, liftedByUserID string) error {
	now := kernel.Normalize(s.d.Clock.Now())
	repo := s.d.Repository
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		a, err := repo.DomainAction(ctx, tx, actionID)
		if err != nil {
			return err
		}
		err = repo.InsertDomainActionLift(ctx, tx, moderation.DomainLift{ActionID: a.ID, LiftedByUserID: liftedByUserID, LiftedAt: now})
		if errors.Is(err, ErrAlreadyRecorded) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := repo.DeleteDomainRestrictionsOf(ctx, tx, a.ID); err != nil {
			return err
		}
		return s.emitDomain(ctx, tx, a.Domain, now)
	})
}

func (s *Service) emitDomain(ctx context.Context, tx uow.Tx, domain string, now time.Time) error {
	rows, err := s.d.Repository.DomainRestrictionRows(ctx, tx, domain)
	if err != nil {
		return err
	}
	return tx.Emit(ctx, DomainRestrictionsChanged{Domain: domain, Effects: moderation.Merge(rows, now)})
}

// remoteActionKinds are the kinds of actions on a remote account whose only effect is the
// restrictions they impose (data-model §5.2: login and post mean nothing for a remote account).
var remoteActionKinds = []moderation.ActionKind{moderation.ActionLimit, moderation.ActionSuspend, moderation.ActionForceSensitive}

// localActionKinds are the kinds of actions on a local account that this module issues: those
// whose effects are their restrictions and the notice to the account (data-model §5.2). The
// other kinds also change statuses or the account's lifecycle.
var localActionKinds = []moderation.ActionKind{
	moderation.ActionWarn, moderation.ActionForceSensitive, moderation.ActionLimit, moderation.ActionRestrictPosting,
	moderation.ActionRestrictInteraction, moderation.ActionDisableLogin, moderation.ActionSuspend,
}

// AccountActionInput is a moderator's action on an account.
type AccountActionInput struct {
	TargetAccountID string
	Kind            moderation.ActionKind
	ReasonCategory  moderation.Category
	PublicReason    string
	PrivateNote     string
	// ReportID is the report the action follows from; "" for none. The report must be of the
	// target account.
	ReportID string
	// NotifyTarget tells a local target of the action (moderation_notice); it is ignored for a
	// remote account.
	NotifyTarget bool
	// IssuedByUserID is the moderator; "" for the CLI.
	IssuedByUserID string
	// Duration is how long the action lasts; zero lasts until it is lifted.
	Duration time.Duration
}

// IssueAccountAction records an action on an account and, in the same transaction, the
// restrictions it imposes (data-model §5.2), and emits AccountRestrictionsChanged. A local
// account takes the kinds of localActionKinds, a remote one limit, suspend, and
// force_sensitive; another kind, or the instance actor, fails with ErrUnsupportedAction, and an
// unknown kind or an invalid field with moderation.ErrInvalidAction. An unknown account fails
// with ErrAccountNotFound, and a report that does not exist or is of
// another account with ErrInvalidReportReference.
func (s *Service) IssueAccountAction(ctx context.Context, in AccountActionInput) (moderation.Action, error) {
	if !in.Kind.Valid() {
		return moderation.Action{}, fmt.Errorf("%w: kind %q", moderation.ErrInvalidAction, in.Kind)
	}
	target, err := s.d.Accounts.FindByID(ctx, in.TargetAccountID)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		return moderation.Action{}, fmt.Errorf("%w: %s", ErrAccountNotFound, in.TargetAccountID)
	case err != nil:
		return moderation.Action{}, err
	case target.IsInstanceActor():
		return moderation.Action{}, fmt.Errorf("%w: %s is the instance actor", ErrUnsupportedAction, in.TargetAccountID)
	}
	kinds, where := remoteActionKinds, "remote"
	if target.Account.IsLocal() {
		kinds, where = localActionKinds, "local"
	} else {
		in.NotifyTarget = false
	}
	if !slices.Contains(kinds, in.Kind) {
		return moderation.Action{}, fmt.Errorf("%w: %s on a %s account", ErrUnsupportedAction, in.Kind, where)
	}
	if in.ReportID != "" {
		r, err := s.d.Repository.FindReport(ctx, in.ReportID)
		switch {
		case errors.Is(err, ErrNotFound):
			return moderation.Action{}, fmt.Errorf("%w: no report %s", ErrInvalidReportReference, in.ReportID)
		case err != nil:
			return moderation.Action{}, err
		case r.TargetAccountID != in.TargetAccountID:
			return moderation.Action{}, fmt.Errorf("%w: report %s is of another account", ErrInvalidReportReference, in.ReportID)
		}
	}
	now := kernel.Normalize(s.d.Clock.Now())
	a := moderation.Action{
		ID: s.d.IDs.NewID(), TargetAccountID: in.TargetAccountID, Kind: in.Kind, ReasonCategory: in.ReasonCategory,
		PublicReason: in.PublicReason, PrivateNote: in.PrivateNote, ReportID: in.ReportID, Issuer: moderation.IssuerModerator,
		IssuedByUserID: in.IssuedByUserID, NotifyTarget: in.NotifyTarget, ExpiresAt: expiry(now, in.Duration), IssuedAt: now,
	}
	if err := a.Validate(); err != nil {
		return moderation.Action{}, err
	}
	repo := s.d.Repository
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := repo.InsertAction(ctx, tx, a); err != nil {
			return err
		}
		for _, r := range a.Rows() {
			if err := repo.InsertAccountRestriction(ctx, tx, r); err != nil {
				return err
			}
		}
		return s.emitAccount(ctx, tx, a.TargetAccountID, now)
	})
	if err != nil {
		return moderation.Action{}, err
	}
	return a, nil
}

// LiftAccountAction lifts an action on an account: its restrictions end, while those of other
// actions on the account stay. Lifting an action that is already lifted changes nothing. An
// invalid reason fails with an error wrapping moderation.ErrInvalidAction and a
// *moderation.TextError.
func (s *Service) LiftAccountAction(ctx context.Context, actionID, liftedByUserID, reason string) error {
	now := kernel.Normalize(s.d.Clock.Now())
	lift := moderation.Lift{ActionID: actionID, LiftedByUserID: liftedByUserID, Reason: reason, LiftedAt: now}
	if err := lift.Validate(); err != nil {
		return err
	}
	repo := s.d.Repository
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		a, err := repo.Action(ctx, tx, actionID)
		if err != nil {
			return err
		}
		err = repo.InsertActionLift(ctx, tx, lift)
		if errors.Is(err, ErrAlreadyRecorded) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := repo.DeleteAccountRestrictionsOf(ctx, tx, a.ID); err != nil {
			return err
		}
		return s.emitAccount(ctx, tx, a.TargetAccountID, now)
	})
}

func (s *Service) emitAccount(ctx context.Context, tx uow.Tx, accountID string, now time.Time) error {
	rows, err := s.d.Repository.AccountRestrictionRows(ctx, tx, accountID)
	if err != nil {
		return err
	}
	return tx.Emit(ctx, AccountRestrictionsChanged{AccountID: accountID, Effects: moderation.Merge(rows, now)})
}

func expiry(now time.Time, d time.Duration) *time.Time {
	if d <= 0 {
		return nil
	}
	t := now.Add(d)
	return &t
}

// RemoteReportInput is a report received from another server in a Flag (federation.md §11).
type RemoteReportInput struct {
	// URI is the id of the Flag (or the URI synthesized for a Flag without one), the key that
	// makes filing the same Flag again do nothing.
	URI string
	// ReporterAccountID is the remote actor of the Flag, often the instance actor of its server.
	ReporterAccountID string
	// TargetAccountID is the local account reported.
	TargetAccountID string
	// StatusIDs are the target's statuses the Flag points at.
	StatusIDs []string
	// Comment is the Flag's content as plain text; it is cut to MaxReportComment characters.
	Comment string
}

// FileRemoteReport records the report of a received Flag, open, in the moderators' queue
// (FR-701), and emits ReportFiled. Filing a Flag whose URI already has a report changes nothing
// and returns that report; created reports whether this call filed it.
func (s *Service) FileRemoteReport(ctx context.Context, in RemoteReportInput) (r moderation.Report, created bool, err error) {
	if in.URI == "" {
		return moderation.Report{}, false, fmt.Errorf("%w: a received report needs a URI", moderation.ErrInvalidReport)
	}
	now := kernel.Normalize(s.d.Clock.Now())
	r = moderation.Report{
		ID: s.d.IDs.NewID(), URI: in.URI, ReporterAccountID: in.ReporterAccountID, TargetAccountID: in.TargetAccountID,
		Category: moderation.CategoryOther, Comment: moderation.TruncateComment(in.Comment), FiledAt: now,
		StatusIDs: uniqueSorted(in.StatusIDs), RuleIDs: []string{},
	}
	if err := r.Validate(); err != nil {
		return moderation.Report{}, false, err
	}
	repo := s.d.Repository
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		ok, err := repo.InsertReport(ctx, tx, r)
		if err != nil || !ok {
			created = false
			return err
		}
		created = true
		if err := repo.UpsertReportStatus(ctx, tx, moderation.ReportStatus{ReportID: r.ID, State: moderation.ReportOpen, Since: now}); err != nil {
			return err
		}
		return emitReportFiled(ctx, tx, repo, r)
	})
	if err != nil {
		return moderation.Report{}, false, err
	}
	if !created {
		r, err = repo.FindReportByURI(ctx, in.URI)
		return r, false, err
	}
	return r, true, nil
}

// emitReportFiled emits ReportFiled for the report r just filed in tx.
func emitReportFiled(ctx context.Context, tx uow.Tx, repo Repository, r moderation.Report) error {
	other, err := repo.HasOtherOpenReportOf(ctx, tx, r.TargetAccountID, r.ID)
	if err != nil {
		return err
	}
	return tx.Emit(ctx, ReportFiled{Report: r, FirstOpen: !other})
}

// CountOpenReports counts the reports that await the moderators (GET /api/v1/admin/counts).
func (s *Service) CountOpenReports(ctx context.Context) (int64, error) {
	return s.d.Repository.CountReports(ctx, moderation.ReportOpen)
}

func uniqueSorted(ids []string) []string {
	out := append([]string{}, ids...)
	slices.Sort(out)
	return slices.Compact(out)
}

// Report returns a report with its statuses.
func (s *Service) Report(ctx context.Context, id string) (moderation.Report, error) {
	return s.d.Repository.FindReport(ctx, id)
}

// ReportStatus returns the state of a report.
func (s *Service) ReportStatus(ctx context.Context, id string) (moderation.ReportStatus, error) {
	return s.d.Repository.FindReportStatus(ctx, id)
}

// ReportsOf returns the reports of an account, oldest first.
func (s *Service) ReportsOf(ctx context.Context, targetAccountID string) ([]moderation.Report, error) {
	return s.d.Repository.ListReportsOf(ctx, targetAccountID)
}
