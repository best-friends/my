// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/platform/config"
)

// retentionRepositories are the persistence of the purges of federated data; the database
// adapters' repositories implement them.
type retentionRepositories struct {
	statuses content.RetentionRepository
	purges   federation.PurgeRepository
	profiles account.ProfilePruneRepository
}

// setUpRetention registers the purges of federated data and their schedules (ADR-0034, FR-803):
// the remote statuses past remote_status_retention (or remote_status_max_retention), the inbox
// and delivery records, and the past profiles of unrelated remote accounts. It comes after the
// content, graph, and notification services, which tell what local data refers to.
func (a *App) setUpRetention(cfg *config.Config, repos retentionRepositories, statuses content.Repository) {
	content.NewRetention(content.RetentionDeps{
		UnitOfWork: a.UnitOfWork, Repository: statuses, Retention: repos.statuses, Notifications: a.Notifications,
		Jobs: a.Jobs, Clock: a.Clock, Period: cfg.RemoteStatusRetention, MaxPeriod: cfg.RemoteStatusMaxRetention,
	}).RegisterJobs(a.Handlers, a.Scheduler)
	federation.NewPurger(federation.PurgeDeps{
		UnitOfWork: a.UnitOfWork, Repository: repos.purges, Clock: a.Clock,
	}).RegisterJobs(a.Handlers, a.Scheduler)
	account.NewProfilePruner(account.PruneDeps{
		UnitOfWork: a.UnitOfWork, Repository: repos.profiles, Relations: remoteRelations{a}, Jobs: a.Jobs,
	}).RegisterJobs(a.Handlers, a.Scheduler)
}

// remoteRelations tells the account module whether something local relates to a remote account:
// a follow either way (the graph module) or a stored status of it (the content module), which
// both import the account module.
type remoteRelations struct{ a *App }

func (r remoteRelations) RelatedToLocal(ctx context.Context, accountID string) (bool, error) {
	c, err := r.a.Graph.Counters(ctx, accountID)
	if err != nil || c.FollowersCount > 0 || c.FollowingCount > 0 {
		return err == nil, err
	}
	n, err := r.a.Content.PostCount(ctx, accountID)
	return n > 0, err
}
