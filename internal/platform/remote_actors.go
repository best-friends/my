// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/adapter/htmlsanitizer"
	"github.com/best-friends/my/internal/adapter/httpsig"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	domaincontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/platform/config"
)

// federationRepository is what the database adapters provide to the federation module: the
// outbound side, the remote servers seen, and the received activities.
type federationRepository interface {
	federation.Repository
	federation.InstanceRepository
	federation.InboxRepository
}

// newHTMLSanitizer returns the sanitizer of remote HTML (ADR-0019 §2).
func newHTMLSanitizer() kernel.HTMLSanitizer { return htmlsanitizer.New() }

// recordRemoteImage records the avatar or header of a remote actor as a remote media file of the
// actor, which the media proxy serves (federation.md §9).
func (a *App) recordRemoteImage(ctx context.Context, tx uow.Tx, accountID, url string) (string, error) {
	m, err := a.Media.RecordRemoteMedia(ctx, tx, content.RemoteMediaInput{
		OwnerAccountID: accountID, RemoteURL: url, DeclaredMediaType: domaincontent.MediaTypeImage,
	})
	return m.MediaID, err
}

// setUpRemoteActors builds the fetching of remote actors (federation.md §6): GETs signed with
// the instance actor's key, WebFinger checks of handles, key rotation, the public key resolver
// of inbound signatures, and the refresh and NodeInfo jobs. It needs the HTTPClient.
func (a *App) setUpRemoteActors(cfg *config.Config, repo federationRepository) {
	outbox := a.Outbox
	if outbox == nil {
		outbox = queue.AfterCommit(a.Projectors, a.Jobs, a.Logger)
	}
	fetcher := federation.NewFetcher(federation.FetcherDeps{
		Client: federation.NewSignedHTTPClient(a.HTTPClient, httpsig.New(), a.KV, a.Clock),
		KV:     a.KV, Clock: a.Clock,
		// No domain blocks exist yet; fetches still go through the policy port.
		Domains: federation.NoDomainRestrictions{}, Instances: repo,
		Key: federation.InstanceKey(a.Accounts), Host: cfg.Host,
	})
	a.RemoteActors = federation.NewRemoteActors(federation.RemoteActorsDeps{
		UnitOfWork: a.UnitOfWork, Accounts: a.Accounts, Instances: repo, Fetcher: fetcher, HTTP: a.HTTPClient,
		Jobs: a.Jobs, Outbox: outbox, Followers: a.Graph, Clock: a.Clock, IDs: a.IDs, Logger: a.Logger,
	})
	a.RemoteActors.RegisterJobs(a.Handlers, a.Scheduler)
}
