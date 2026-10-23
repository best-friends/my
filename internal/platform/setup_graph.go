// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/platform/config"
)

// setUpGraph registers the graph module's projectors, builds its service, and registers its jobs.
func (a *App) setUpGraph(cfg *config.Config, repo graph.Repository) {
	graph.RegisterProjectors(a.Projectors, repo)
	a.Graph = graph.NewService(graph.Deps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs, Host: cfg.Host,
		Interactions: a.AccountPolicy,
	})
	a.Graph.RegisterJobs(a.Handlers, a.Scheduler)
}
