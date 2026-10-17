// SPDX-License-Identifier: AGPL-3.0-only

package platform

import "github.com/best-friends/my/internal/app/moderation"

// setUpModeration builds the moderation use cases and the policy that federation and the media
// proxy read the domain and account restrictions through (docs/architecture/federation.md §11).
// It needs the account module, and comes before everything that federates. It also registers
// the audit log projector.
func (a *App) setUpModeration(repo moderation.Repository) {
	a.Moderation = moderation.NewService(moderation.Deps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs,
	})
	a.ModerationPolicy = moderation.NewFederationPolicy(repo, a.Clock)
	// The administrative use cases of every module emit their audit log entries; the projector
	// writes them in the transaction of the operation (ADR-0037).
	moderation.RegisterAuditProjector(a.Projectors, repo)
}
