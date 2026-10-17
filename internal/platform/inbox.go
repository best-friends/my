// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"github.com/best-friends/my/internal/adapter/httpsig"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/queue"
)

// setUpInbox builds the synchronous part of the inbox (docs/architecture/federation.md §3.1):
// signatures verified with the HTTP signature adapter and the keys that RemoteActors resolves,
// the IP and domain rate limits, and the record and job written once per activity. It needs
// RemoteActors and InstanceHealth.
func (a *App) setUpInbox(repo federation.InboxRepository) {
	outbox := a.Outbox
	if outbox == nil {
		outbox = queue.AfterCommit(a.Projectors, a.Jobs, a.Logger)
	}
	a.Inbox = federation.NewInbox(federation.InboxDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Outbox: outbox,
		Signatures: federation.NewSignatureVerifier(httpsig.New(), a.RemoteActors, a.KV, a.Clock),
		Accounts:   a.Accounts,
		// No domain blocks exist yet; signers still go through the policy port.
		Domains: federation.NoDomainRestrictions{},
		Health:  a.InstanceHealth, RateLimiter: a.RateLimiter, Clock: a.Clock, Logger: a.Logger,
	})
	a.setUpInboxProcessor(repo)
}

// setUpInboxProcessor builds the asynchronous part of the inbox (docs/architecture/federation.md
// §3.2): the federation.inbox job, which resolves actors and fetches documents through
// RemoteActors and dispatches each activity to the handler of its type.
func (a *App) setUpInboxProcessor(repo federation.InboxRepository) {
	a.InboxProcessor = federation.NewInboxProcessor(federation.InboxProcessorDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Actors: a.RemoteActors, Fetcher: a.RemoteActors,
		// No domain or account restrictions exist yet; actors still go through the policy ports.
		Domains: federation.NoDomainRestrictions{}, Accounts: federation.NoAccountRestrictions{},
		Clock: a.Clock, Logger: a.Logger,
	})
	a.RemoteActors.RegisterInboxHandlers(a.InboxProcessor)
	a.InboxProcessor.RegisterJobs(a.Handlers)
	a.registerStatusHandlers(repo)
}
