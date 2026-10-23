// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/account"
	domainaccount "github.com/best-friends/my/internal/domain/account"
	domaincontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/platform/config"
)

// setUpAccount registers the account module's projectors and builds its service. It comes first
// of the modules: the others read the accounts.
func (a *App) setUpAccount(cfg *config.Config, repo account.Repository) {
	account.RegisterProjectors(a.Projectors, repo)
	a.Accounts = account.NewService(account.Deps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Clock: a.Clock, IDs: a.IDs, Sealer: a.Sealer, Host: cfg.Host,
		// Profiles are rendered and their images checked by the content module, built below; the
		// account module does not import it, since the content module imports the account module.
		RenderProfileText: func(ctx context.Context, text string) (string, error) {
			return a.Content.RenderProfileText(ctx, text)
		},
		CheckProfileImage: func(ctx context.Context, accountID, mediaID string, prop domainaccount.ProfileProperty) error {
			purpose := domaincontent.PurposeAvatar
			if prop == domainaccount.PropertyHeader {
				purpose = domaincontent.PurposeHeader
			}
			return a.Media.CheckProfileImage(ctx, accountID, mediaID, purpose)
		},
		Sanitizer:          newHTMLSanitizer(),
		RecordRemoteImage:  a.recordRemoteImage,
		RecordRemoteEmojis: a.recordRemoteEmojis,
		// The local custom emoji of profiles, which the content module keeps (ADR-0051 §3).
		LocalEmojis: func(ctx context.Context, texts ...string) ([]string, error) {
			return a.CustomEmojis.LocalEmojiIDs(ctx, texts...)
		},
	})
}
