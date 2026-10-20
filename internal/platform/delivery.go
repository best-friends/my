// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"github.com/best-friends/my/internal/adapter/httpsig"
	"github.com/best-friends/my/internal/app/federation"
)

// setUpDelivery registers the delivery of outbound activities, the instance_health projector,
// and the daily probe of unavailable servers (docs/architecture/federation.md §7.3). It needs
// the HTTP client, so it runs after newHTTPClient.
func (a *App) setUpDelivery(repo federation.Repository) {
	federation.RegisterHealthProjectors(a.Projectors, federation.HealthProjectorDeps{Repository: repo, IDs: a.IDs})
	a.InstanceHealth = federation.NewHealth(federation.HealthDeps{UnitOfWork: a.UnitOfWork, Repository: repo, Clock: a.Clock})
	a.Deliverer = federation.NewDeliverer(federation.DeliverDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo,
		Client:  federation.NewSignedHTTPClient(a.HTTPClient, httpsig.New(), a.KV, a.Clock),
		Keys:    federation.AccountSigningKeys{Accounts: a.Accounts},
		Proofs:  federation.AccountProofKeys{Accounts: a.Accounts},
		Domains: a.ModerationPolicy,
		Clock:   a.Clock, IDs: a.IDs, Logger: a.Logger, Meter: a.Meter,
	})
	a.Deliverer.RegisterJobs(a.Handlers)
	federation.NewProber(federation.ProbeDeps{
		Repository: repo, Health: a.InstanceHealth, HTTP: a.HTTPClient, Jobs: a.Jobs, Logger: a.Logger,
	}).RegisterJobs(a.Handlers, a.Scheduler)
}
