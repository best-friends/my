// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"io"

	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/queue"
)

// setUpExports builds the exports of the users' data (FR-008) and registers their jobs. The
// archive holds the actor and the outbox as the federation module writes them, and the user's
// own moderation records (ADR-0038). It comes after the blob store; the entries read the status
// reader, which is built later, only when a job runs.
func (a *App) setUpExports(repo identity.Repository) {
	outbox := a.Outbox
	if outbox == nil {
		// The inline queue has no transactional enqueue; its jobs are enqueued after the commit.
		outbox = queue.AfterCommit(a.Projectors, a.Jobs, a.Logger)
	}
	a.Exports = identity.NewExports(identity.ExportDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Blobs: a.Blobs, Outbox: outbox, Accounts: a.Accounts,
		Entries: []identity.ArchiveEntry{
			{Name: "actor.json", Write: func(ctx context.Context, accountID string, w io.Writer) error {
				return a.archiveObjects().ArchiveActor(ctx, accountID, w)
			}},
			{Name: "outbox.json", Write: func(ctx context.Context, accountID string, w io.Writer) error {
				return a.archiveObjects().ArchiveOutbox(ctx, accountID, w)
			}},
			{Name: "moderation.json", Write: a.Moderation.WriteOwnRecords},
		},
		Clock: a.Clock, IDs: a.IDs, Logger: a.Logger,
	})
	a.Exports.RegisterJobs(a.Handlers, a.Scheduler)
}

// archiveObjects returns the federation use case that writes the documents of an archive; it
// verifies no signatures.
func (a *App) archiveObjects() *federation.Objects {
	return federation.NewObjects(federation.ObjectsDeps{
		Statuses: a.Content, Accounts: a.Accounts, Graph: a.Graph, Timeline: a.Timeline, Reader: a.Statuses,
		Host: a.Config.Host, AccountDomain: a.Config.AccountDomain, Logger: a.Logger,
	})
}
