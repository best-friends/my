// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"errors"

	"github.com/mycroblog/my/internal/app/account"
	"github.com/mycroblog/my/internal/app/uow"
)

// The reads in a transaction: each answers as the method of the same name without Tx does, read
// in tx with what tx has written, for the use cases that present what an event's transaction
// wrote (the data of the webhook messages, ADR-0060).

// RegistrationTx returns the registration of a user as Registration does, read in tx.
func (s *Service) RegistrationTx(ctx context.Context, tx uow.Tx, userID string) (RegistrationEntry, error) {
	repo := s.d.Repository
	app, err := repo.FindRegistrationApplicationTx(ctx, tx, userID)
	if err != nil {
		return RegistrationEntry{}, err
	}
	st, err := repo.FindRegistrationStateTx(ctx, tx, userID)
	if err != nil {
		return RegistrationEntry{}, err
	}
	e := RegistrationEntry{Application: app, State: st}
	email, verified, err := repo.FindPrimaryEmailTx(ctx, tx, userID)
	switch {
	case err == nil:
		e.Email, e.EmailVerified = email.Address, verified
	case !errors.Is(err, ErrNotFound):
		return RegistrationEntry{}, err
	}
	la, err := s.d.Accounts.LocalAccountOfUserTx(ctx, tx, userID)
	switch {
	case err == nil:
		e.AccountID, e.Username = la.AccountID, la.Username
	case !errors.Is(err, account.ErrNotFound):
		return RegistrationEntry{}, err
	}
	return e, nil
}

// AccountTx returns an account as moderators see it as Account does, read in tx.
func (s *AccountAdmin) AccountTx(ctx context.Context, tx uow.Tx, accountID string) (AdminAccount, error) {
	actor, err := s.d.Accounts.FindByIDTx(ctx, tx, accountID)
	if err != nil {
		return AdminAccount{}, err
	}
	out := AdminAccount{Actor: actor}
	if out.Restrictions, err = s.d.Restrictions.OwnRestrictionsTx(ctx, tx, accountID); err != nil {
		return AdminAccount{}, err
	}
	if actor.Local != nil && actor.Local.UserID != "" {
		out.UserID = actor.Local.UserID
		st, err := s.d.Repository.FindRegistrationStateTx(ctx, tx, out.UserID)
		switch {
		case err == nil:
			out.Registration = &st
		case !errors.Is(err, ErrNotFound):
			return AdminAccount{}, err
		}
	}
	out.State = adminState(out)
	return out, nil
}
