// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/uow"
	domaincontent "github.com/best-friends/my/internal/domain/content"
)

// remoteMediaRecorder records the attachments and custom emoji images of remote statuses for the
// content module. It reads a.Media per call, since the content module is built before setMedia.
type remoteMediaRecorder struct{ a *App }

func (r remoteMediaRecorder) RecordRemoteMedia(ctx context.Context, tx uow.Tx, in content.RemoteMediaInput) (domaincontent.RemoteMedia, error) {
	return r.a.Media.RecordRemoteMedia(ctx, tx, in)
}

// registerStatusHandlers registers the inbox handlers of remote posts (Create, Update, and
// Delete, docs/architecture/federation.md §3.4) on the inbox processor: the posts related to
// local accounts are ingested through the content module (ADR-0034).
func (a *App) registerStatusHandlers() {
	federation.NewStatusHandlers(federation.StatusHandlersDeps{
		UnitOfWork: a.UnitOfWork, Statuses: a.Content, Accounts: a.Accounts, Actors: a.RemoteActors,
		Fetcher: a.RemoteActors, Followers: a.Graph, Clock: a.Clock, Host: a.Config.Host, Logger: a.Logger,
	}).Register(a.InboxProcessor)
}
