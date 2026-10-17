// SPDX-License-Identifier: AGPL-3.0-only

package platform

import "github.com/best-friends/my/internal/app/moderation"

// setUpModeration builds the moderation use cases, the policy that federation and the media
// proxy read the domain and account restrictions through (docs/architecture/federation.md §11),
// and the policy of the accounts' restrictions (data-model §5.3). It needs the account module,
// and comes before the identity module and everything that federates.
func (a *App) setUpModeration(repo moderation.Repository) {
	a.Moderation = moderation.NewService(moderation.Deps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs,
	})
	a.ModerationPolicy = moderation.NewFederationPolicy(repo, a.Clock)
	a.AccountPolicy = moderation.NewAccountPolicy(repo, a.Accounts, a.Clock)
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
