// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// Deps are the ports and collaborators a Service uses.
type Deps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	Clock      kernel.Clock
	IDs        kernel.IDGenerator
	Passwords  *PasswordHasher
	// Accounts creates the local account a new user logs in as.
	Accounts *account.Service
}

// Service runs the identity module's use cases.
type Service struct {
	d Deps
}

// NewService returns a Service on d.
func NewService(d Deps) *Service {
	return &Service{d: d}
}

// RegisterInput describes a new local user.
type RegisterInput struct {
	Username    string
	Password    string
	DisplayName string
}

// Registration is the result of RegisterLocalUser.
type Registration struct {
	User identity.User
	// AccountID is the ID of the user's local account.
	AccountID string
}

// RegisterLocalUser creates a user with a password and the local account it logs in as, in one
// transaction. It fails with account.ErrUsernameTaken when the username is taken, and with
// errors wrapping identity.ErrInvalidPassword or account.ErrInvalidUsername for invalid input.
//
// Hashing the password and generating the account's keys happen before the transaction.
func (s *Service) RegisterLocalUser(ctx context.Context, in RegisterInput) (Registration, error) {
	if err := identity.ValidatePassword(in.Password); err != nil {
		return Registration{}, err
	}
	now := s.d.Clock.Now()
	user := identity.User{ID: s.d.IDs.NewID(), CreatedAt: now}
	acct, err := s.d.Accounts.PrepareLocalAccount(ctx, account.LocalAccountInput{
		UserID:      user.ID,
		Username:    in.Username,
		DisplayName: in.DisplayName,
	})
	if err != nil {
		return Registration{}, err
	}
	hash, err := s.d.Passwords.Hash(ctx, in.Password)
	if err != nil {
		return Registration{}, err
	}
	cred := identity.PasswordCredential{ID: s.d.IDs.NewID(), UserID: user.ID, Hash: hash, SetAt: now}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertUser(ctx, tx, user); err != nil {
			return err
		}
		if err := s.d.Repository.InsertPasswordCredential(ctx, tx, cred); err != nil {
			return err
		}
		return s.d.Accounts.WriteLocalAccount(ctx, tx, acct)
	})
	if err != nil {
		return Registration{}, err
	}
	return Registration{User: user, AccountID: acct.Account.ID}, nil
}

// EnsureBuiltinRoles creates the built-in roles (admin and moderator) with their permissions,
// or adds the permissions they lack. It is safe to run on every start.
func (s *Service) EnsureBuiltinRoles(ctx context.Context) error {
	builtins := identity.BuiltinRoles()
	now := s.d.Clock.Now()
	roles := make([]identity.Role, len(builtins))
	for i, b := range builtins {
		roles[i] = identity.Role{ID: s.d.IDs.NewID(), Key: b.Key, Position: b.Position, CreatedAt: now}
	}
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		for i, b := range builtins {
			if _, err := s.d.Repository.EnsureRole(ctx, tx, roles[i], b.Permissions); err != nil {
				return err
			}
		}
		return nil
	})
}
