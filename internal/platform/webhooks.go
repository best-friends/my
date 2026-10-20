// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/app/webhook"
	domainaccount "github.com/best-friends/my/internal/domain/account"
	domainmoderation "github.com/best-friends/my/internal/domain/moderation"
	domainwebhook "github.com/best-friends/my/internal/domain/webhook"
	"github.com/best-friends/my/internal/interface/clientapi"
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

// webhookReads are the reads in a transaction behind the data of the webhook messages
// (clientapi.WebhookReads), over the modules' use cases.
type webhookReads struct{ a *App }

var _ clientapi.WebhookReads = webhookReads{}

func (r webhookReads) FindByIDTx(ctx context.Context, tx uow.Tx, id string) (account.Actor, error) {
	return r.a.Accounts.FindByIDTx(ctx, tx, id)
}

func (r webhookReads) LocalAccountOfUserTx(ctx context.Context, tx uow.Tx, userID string) (domainaccount.LocalAccount, error) {
	return r.a.Accounts.LocalAccountOfUserTx(ctx, tx, userID)
}

func (r webhookReads) ImagesTx(ctx context.Context, tx uow.Tx, refs []content.ImageRef) (map[content.ImageRef]content.Image, error) {
	return r.a.Media.ImagesTx(ctx, tx, refs)
}

func (r webhookReads) CustomEmojisTx(ctx context.Context, tx uow.Tx, ids []string) (map[string]content.CustomEmojiView, error) {
	return r.a.Media.CustomEmojisTx(ctx, tx, ids)
}

func (r webhookReads) RemoteProfileURLsTx(ctx context.Context, tx uow.Tx, accountIDs []string) (map[string]string, error) {
	return r.a.Accounts.RemoteProfileURLsTx(ctx, tx, accountIDs)
}

func (r webhookReads) ReportRecordTx(ctx context.Context, tx uow.Tx, id string) (domainmoderation.ReportRecord, error) {
	return r.a.Moderation.ReportRecordTx(ctx, tx, id)
}

func (r webhookReads) AccountActionTx(ctx context.Context, tx uow.Tx, id string) (moderation.AccountActionView, error) {
	return r.a.Moderation.AccountActionTx(ctx, tx, id)
}

func (r webhookReads) AppealTx(ctx context.Context, tx uow.Tx, id string) (moderation.AppealView, error) {
	return r.a.Moderation.AppealTx(ctx, tx, id)
}

func (r webhookReads) DomainActionTx(ctx context.Context, tx uow.Tx, id string) (moderation.DomainActionView, error) {
	return r.a.Moderation.DomainActionTx(ctx, tx, id)
}

func (r webhookReads) RegistrationTx(ctx context.Context, tx uow.Tx, userID string) (identity.RegistrationEntry, error) {
	return r.a.Identity.RegistrationTx(ctx, tx, userID)
}

func (r webhookReads) AdminAccountTx(ctx context.Context, tx uow.Tx, id string) (identity.AdminAccount, error) {
	return r.a.AccountAdmin.AccountTx(ctx, tx, id)
}

func (r webhookReads) CustomEmojiTx(ctx context.Context, tx uow.Tx, id string) (content.AdminCustomEmoji, error) {
	return r.a.CustomEmojis.GetTx(ctx, tx, id)
}
