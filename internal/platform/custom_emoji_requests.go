// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"errors"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/identity"
)

// The ports of the custom emoji requests (ADR-0070 §2, §3, §4), which the content module reads
// of the identity, account, and supporters modules. They read the modules lazily, because the
// custom emoji use cases are built before the supporters module.

// emojiCreators tells the content module whether a user may create custom emoji now: their
// roles grant create_custom_emojis and they may use the app.
type emojiCreators struct{ a *App }

func (c emojiCreators) MayCreate(ctx context.Context, userID string) (bool, error) {
	if c.a.Identity == nil {
		return false, nil
	}
	ok, err := c.a.Identity.HasPermission(ctx, userID, identity.PermCreateCustomEmojis)
	if err != nil || !ok {
		return false, err
	}
	access, err := c.a.Identity.UserAccess(ctx, userID)
	if err != nil {
		return false, err
	}
	return access.Allowed(), nil
}

func (c emojiCreators) AccountOf(ctx context.Context, userID string) (string, error) {
	la, err := c.a.Accounts.LocalAccountOfUser(ctx, userID)
	if errors.Is(err, account.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return la.AccountID, nil
}

// emojiQuota is the limit of the custom emoji that a supporter plan sets
// (supporters.Service.EmojiLimit, ADR-0072 §11); without the supporters module, nobody is
// limited.
type emojiQuota struct{ a *App }

func (q emojiQuota) Limit(ctx context.Context, userID string) (int, bool, error) {
	if q.a.Supporters == nil {
		return 0, false, nil
	}
	return q.a.Supporters.EmojiLimit(ctx, userID)
}

// emojiImageOwner returns the instance actor, creating it on first use, whose uploads the
// images of the approved custom emoji requests become.
func (a *App) emojiImageOwner(ctx context.Context) (string, error) {
	acc, err := a.Accounts.EnsureInstanceActor(ctx)
	if err != nil {
		return "", err
	}
	return acc.ID, nil
}
