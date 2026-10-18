// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"time"

	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
)

// setUpModeration builds the moderation use cases, the policy that federation and the media
// proxy read the domain and account restrictions through (docs/architecture/federation.md §11),
// and the policy of the accounts' restrictions (data-model §5.3). It needs the account module,
// and comes before the identity module and everything that federates. It also registers the
// job that deletes the accounts whose deletion is due, which bans schedule, and
// the audit log projector.
func (a *App) setUpModeration(repo moderation.Repository) {
	a.Moderation = moderation.NewService(moderation.Deps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs,
		Statuses: moderationStatuses{a}, Lifecycles: a.Accounts,
		Emails: moderationEmails{a}, CanonicalDomain: federation.CanonicalHost,
	})
	a.ModerationPolicy = moderation.NewFederationPolicy(repo, a.Clock)
	a.AccountPolicy = moderation.NewAccountPolicy(repo, a.Accounts, a.Clock)
	// The removal of expired restrictions, which ends expired suspensions on other servers
	// (federation.md §11.1).
	a.Moderation.RegisterJobs(a.Handlers, a.Scheduler)
	a.Accounts.RegisterDeletionJobs(a.Handlers, a.Scheduler)
	// The administrative use cases of every module emit their audit log entries; the projector
	// writes them in the transaction of the operation (ADR-0037).
	moderation.RegisterAuditProjector(a.Projectors, repo)
}

// moderationStatuses gives the moderation module the content module, which is built after it
// because it reads the moderation restrictions.
type moderationStatuses struct{ a *App }

func (m moderationStatuses) Find(ctx context.Context, statusID string) (content.Post, error) {
	return m.a.Content.Find(ctx, statusID)
}

func (m moderationStatuses) DeleteByModeration(ctx context.Context, tx uow.Tx, st content.Status, actionID string, at time.Time) error {
	return m.a.Content.DeleteByModeration(ctx, tx, st, actionID, at)
}

func (m moderationStatuses) MarkSensitiveByModeration(ctx context.Context, tx uow.Tx, statusID, actionID string) error {
	return m.a.Content.MarkSensitiveByModeration(ctx, tx, statusID, actionID)
}

func (m moderationStatuses) ClearSensitivityOf(ctx context.Context, tx uow.Tx, actionID string) error {
	return m.a.Content.ClearSensitivityOf(ctx, tx, actionID)
}

// moderationEmails gives the moderation module the identity module, which is built after it
// because it reads the moderation restrictions, for the sign-up block that a ban makes of the
// banned user's email address.
type moderationEmails struct{ a *App }

func (m moderationEmails) PrimaryEmailAddress(ctx context.Context, userID string) (string, error) {
	return m.a.Identity.PrimaryEmailAddress(ctx, userID)
}

// setUpLocalReports builds the filing of local users' reports, which checks the reported
// statuses through the status reader and the cited rules through the instance module, so it
// comes after both.
func (a *App) setUpLocalReports(repo moderation.Repository) {
	a.LocalReports = moderation.NewLocalReports(moderation.LocalReportDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Accounts: a.Accounts, Statuses: a.Statuses,
		Rules: a.Instance, Clock: a.Clock, IDs: a.IDs,
	})
}
