// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"errors"
	"slices"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// LoginPolicy reads whether moderation forbids a local account to log in and use the API (the
// login restriction, data-model §5.3). *moderation.AccountPolicy implements it.
type LoginPolicy interface {
	LoginRestricted(ctx context.Context, accountID string) (bool, error)
}

// loginRestricted reports whether the policy forbids the account to log in; without a policy,
// nobody is.
func (s *Service) loginRestricted(ctx context.Context, accountID string) (bool, error) {
	if s.d.Logins == nil {
		return false, nil
	}
	return s.d.Logins.LoginRestricted(ctx, accountID)
}

// Accounts finds the local account of a restricted account. *account.Service implements it.
type Accounts interface {
	FindByID(ctx context.Context, id string) (account.Actor, error)
}

// RegisterModerationProjectors subscribes the end of a user's sessions and tokens to the
// moderation module's events: when the restrictions in effect on a local account include login
// (disable_login, suspend), every session and token of its user is revoked with the reason
// moderation in the transaction of the action (data-model §5.2). Lifting the action revokes
// nothing and restores nothing; the user signs in again.
func RegisterModerationProjectors(r *uow.Projectors, repo Repository, accounts Accounts, clock kernel.Clock) {
	r.Register(appmoderation.EventAccountRestrictionsChanged, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appmoderation.AccountRestrictionsChanged)
		if !slices.ContainsFunc(ev.Effects, func(e appmoderation.Effect) bool { return e.Restriction == appmoderation.RestrictLogin }) {
			return nil
		}
		a, err := accounts.FindByID(ctx, ev.AccountID)
		switch {
		case errors.Is(err, account.ErrNotFound):
			return nil
		case err != nil:
			return err
		case a.Local == nil || a.Local.UserID == "":
			return nil
		}
		now := kernel.Normalize(clock.Now())
		if err := repo.RevokeSessionsOfUser(ctx, tx, a.Local.UserID, identity.RevokedModeration, now); err != nil {
			return err
		}
		return repo.RevokeTokensOfUser(ctx, tx, a.Local.UserID, identity.TokenRevokedModeration, now)
	})
}
