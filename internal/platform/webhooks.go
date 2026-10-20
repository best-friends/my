// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/webhook"
	domainwebhook "github.com/best-friends/my/internal/domain/webhook"
)

// setUpWebhooks builds the instance's webhooks (ADR-0060): the projectors of the module's own
// projections, the sources that turn the other modules' events into messages, and the jobs that
// send and purge them. It comes after the identity module, whose roles decide who may subscribe
// to which events.
func (a *App) setUpWebhooks(repo webhook.Repository) {
	outbox := a.Outbox
	if outbox == nil {
		// The inline queue has no transactional enqueue; its jobs are enqueued after the commit.
		outbox = queue.AfterCommit(a.Projectors, a.Jobs, a.Logger)
	}
	webhook.RegisterProjectors(a.Projectors, repo)
	a.Webhooks = webhook.NewService(webhook.Deps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Outbox: outbox, Sealer: a.Sealer, HTTP: a.HTTPClient,
		Clock: a.Clock, IDs: a.IDs, Logger: a.Logger,
		Permissions: func(ctx context.Context, userID string) ([]domainwebhook.Permission, error) {
			granted, err := a.Identity.Permissions(ctx, userID)
			if err != nil {
				return nil, err
			}
			out := make([]domainwebhook.Permission, len(granted))
			for i, p := range granted {
				out[i] = domainwebhook.Permission(p)
			}
			return out, nil
		},
	})
	a.Webhooks.RegisterSources(a.Projectors)
	a.Webhooks.RegisterJobs(a.Handlers, a.Scheduler)
}
