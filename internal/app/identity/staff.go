// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"errors"
	"time"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// What the moderator notifications read of the identity module (FR-712): who holds a permission,
// whether they want the notifications by email and where, and how many registrations await a
// decision.

// EventRegistrationPending follows a sign-up whose registration awaits a moderator's decision.
const EventRegistrationPending = "identity.registration_pending"

// RegistrationPending carries a new user whose registration awaits a moderator's approval: the
// sign-up of approval mode, and of open mode when no email verification stands in for it
// (FR-002). It is emitted in the sign-up's transaction.
type RegistrationPending struct {
	UserID string
	// AccountID is the user's local account.
	AccountID   string
	SubmittedAt time.Time
}

// EventType implements uow.Event.
func (RegistrationPending) EventType() string { return EventRegistrationPending }

// PermissionHolder is a user whose current roles grant a permission.
type PermissionHolder struct {
	UserID string
	// AccountID is the user's local account.
	AccountID string
	// EmailModeratorNotifications is the user's preference of that name.
	EmailModeratorNotifications bool
}

// PermissionHolders returns inside tx the users whose current roles grant p, with their local
// accounts and whether they want the moderator notifications by email. A user without a local
// account is left out.
func (s *Service) PermissionHolders(ctx context.Context, tx uow.Tx, p identity.Permission) ([]PermissionHolder, error) {
	ids, err := s.d.Repository.ListUserIDsWithPermission(ctx, tx, p)
	if err != nil {
		return nil, err
	}
	out := make([]PermissionHolder, 0, len(ids))
	for _, id := range ids {
		la, err := s.d.Accounts.LocalAccountOfUser(ctx, id)
		switch {
		case errors.Is(err, account.ErrNotFound):
			continue
		case err != nil:
			return nil, err
		}
		prefs, err := s.d.Repository.FindPreferences(ctx, id)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		out = append(out, PermissionHolder{UserID: id, AccountID: la.AccountID, EmailModeratorNotifications: prefs.EmailModeratorNotifications})
	}
	return out, nil
}

// ModeratorMailbox returns the address and locale that a user's moderator notifications are sent
// to by email. ok is false when the user does not want them by email, has no primary address,
// or, where an unverified address keeps users out, has not verified it.
func (s *Service) ModeratorMailbox(ctx context.Context, userID string) (address, locale string, ok bool, err error) {
	prefs, err := s.d.Repository.FindPreferences(ctx, userID)
	switch {
	case errors.Is(err, ErrNotFound):
		return "", "", false, nil
	case err != nil:
		return "", "", false, err
	case !prefs.EmailModeratorNotifications:
		return "", "", false, nil
	}
	email, verified, err := s.d.Repository.FindPrimaryEmail(ctx, userID)
	switch {
	case errors.Is(err, ErrNotFound):
		return "", "", false, nil
	case err != nil:
		return "", "", false, err
	case s.d.RequireEmailVerification && !verified:
		return "", "", false, nil
	}
	return email.Address, prefs.Locale, true, nil
}

// CountPendingRegistrations counts the registrations that await a moderator's decision
// (GET /api/v1/admin/counts).
func (s *Service) CountPendingRegistrations(ctx context.Context) (int64, error) {
	return s.d.Repository.CountRegistrations(ctx, identity.RegistrationPending)
}
