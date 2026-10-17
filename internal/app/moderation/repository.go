// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"errors"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

var (
	// ErrNotFound reports that no row matches a lookup.
	ErrNotFound = errors.New("moderation: not found")
	// ErrAlreadyRecorded reports a second lift of one action.
	ErrAlreadyRecorded = errors.New("moderation: the action is already lifted")
)

// Repository is the persistence port of the moderation module: the actions on accounts and
// domains, their lifts, the restrictions they impose, and reports (data-model §4.3). The
// database adapters implement it and pass testkit.ModerationContract.
//
// Methods that take a uow.Tx run inside that transaction; the others read committed data outside
// any transaction. Lookups that find nothing return an error wrapping ErrNotFound.
type Repository interface {
	// Events and resources, written by commands.

	InsertAction(ctx context.Context, tx uow.Tx, a moderation.Action) error
	// Action returns an action on an account by ID.
	Action(ctx context.Context, tx uow.Tx, id string) (moderation.Action, error)
	// InsertActionLift fails with ErrAlreadyRecorded when the action is already lifted.
	InsertActionLift(ctx context.Context, tx uow.Tx, l moderation.Lift) error
	InsertDomainAction(ctx context.Context, tx uow.Tx, a moderation.DomainAction) error
	// DomainAction returns an action on a domain by ID.
	DomainAction(ctx context.Context, tx uow.Tx, id string) (moderation.DomainAction, error)
	// InsertDomainActionLift fails with ErrAlreadyRecorded when the action is already lifted.
	InsertDomainActionLift(ctx context.Context, tx uow.Tx, l moderation.DomainLift) error
	// InsertReport inserts a report with its statuses and reports whether it did: a report
	// whose URI another report has inserts nothing.
	InsertReport(ctx context.Context, tx uow.Tx, r moderation.Report) (bool, error)

	// Projections, written only by this module's commands.

	InsertAccountRestriction(ctx context.Context, tx uow.Tx, r moderation.RestrictionRow) error
	// DeleteAccountRestrictionsOf deletes the rows that an action on an account imposes.
	DeleteAccountRestrictionsOf(ctx context.Context, tx uow.Tx, actionID string) error
	// AccountRestrictionRows returns every row of an account, expired ones included.
	AccountRestrictionRows(ctx context.Context, tx uow.Tx, accountID string) ([]moderation.RestrictionRow, error)
	InsertDomainRestriction(ctx context.Context, tx uow.Tx, r moderation.RestrictionRow) error
	// DeleteDomainRestrictionsOf deletes the rows that an action on a domain imposes.
	DeleteDomainRestrictionsOf(ctx context.Context, tx uow.Tx, actionID string) error
	// DomainRestrictionRows returns every row of one domain (not its parents), expired ones
	// included.
	DomainRestrictionRows(ctx context.Context, tx uow.Tx, domain string) ([]moderation.RestrictionRow, error)
	UpsertReportStatus(ctx context.Context, tx uow.Tx, s moderation.ReportStatus) error

	// Lookups.

	// AccountRestrictionsInEffect returns the distinct restrictions of an account in effect at
	// now, in restriction order.
	AccountRestrictionsInEffect(ctx context.Context, accountID string, now time.Time) ([]moderation.Restriction, error)
	// DomainRestrictionsInEffect returns the distinct restrictions in effect at now of any of
	// the domains, in restriction order.
	DomainRestrictionsInEffect(ctx context.Context, domains []string, now time.Time) ([]moderation.Restriction, error)
	// FindReport returns a report with its statuses.
	FindReport(ctx context.Context, id string) (moderation.Report, error)
	// FindReportByURI returns the report of a received Flag with its statuses.
	FindReportByURI(ctx context.Context, uri string) (moderation.Report, error)
	FindReportStatus(ctx context.Context, reportID string) (moderation.ReportStatus, error)
	// ListReportsOf returns the reports of a target account with their statuses, oldest first.
	ListReportsOf(ctx context.Context, targetAccountID string) ([]moderation.Report, error)
}
