// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"sync"
	"time"

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
	// DefaultLocale returns the instance's default locale, which a new user gets unless the
	// registration names one. It is called once per registration, before the transaction, so a
	// change of the instance setting applies to the next registration. Nil or an empty result
	// means i18n.SourceLocale.
	DefaultLocale func(ctx context.Context) (string, error)

	// RateLimiter limits failed sign-ins (LoginIPLimit, LoginAccountLimit); SignIn needs it.
	RateLimiter *kernel.RateLimiter
	// SessionTTL is the lifetime of a session; zero means DefaultSessionTTL.
	SessionTTL time.Duration
}

// Service runs the identity module's use cases.
type Service struct {
	d Deps

	// dummy is the hash that SignIn verifies against for unknown users, made on first use.
	dummyMu sync.Mutex
	dummy   string
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
	// Locale is the new user's UI locale, a BCP 47 tag; empty means Deps.DefaultLocale.
	Locale string
}

// Registration is the result of RegisterLocalUser.
type Registration struct {
	User identity.User
	// AccountID is the ID of the user's local account.
	AccountID string
}

// RegisterLocalUser creates a user with a password, default preferences, and the local account it
// logs in as, in one transaction. It fails with account.ErrUsernameTaken when the username is
// taken, and with errors wrapping identity.ErrInvalidPassword, identity.ErrInvalidLanguageTag, or
// account.ErrInvalidUsername for invalid input.
//
// Hashing the password and generating the account's keys happen before the transaction.
func (s *Service) RegisterLocalUser(ctx context.Context, in RegisterInput) (Registration, error) {
	p, err := s.prepareUser(ctx, in)
	if err != nil {
		return Registration{}, err
	}
	if err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.writeUser(ctx, tx, p) }); err != nil {
		return Registration{}, err
	}
	return p.registration(), nil
}

// CreateAdminInput describes a new administrator.
type CreateAdminInput struct {
	Username string
	Email    string
	Password string
}

// CreateAdmin creates a user with a password, a primary email address, default preferences in the
// instance's default locale, the local account it logs in as, and a grant of the built-in admin role from the CLI (no granting user), in one
// transaction. It creates the built-in roles first if they are missing.
//
// It fails with account.ErrUsernameTaken or ErrEmailTaken when another user has the username or
// email address, and with errors wrapping identity.ErrInvalidPassword, identity.ErrInvalidEmail,
// or account.ErrInvalidUsername for invalid input. A failure writes nothing, so running it again
// is safe; it never changes an existing user.
func (s *Service) CreateAdmin(ctx context.Context, in CreateAdminInput) (Registration, error) {
	if err := identity.ValidateEmail(in.Email); err != nil {
		return Registration{}, err
	}
	p, err := s.prepareUser(ctx, RegisterInput{Username: in.Username, Password: in.Password})
	if err != nil {
		return Registration{}, err
	}
	email := identity.EmailAddress{
		ID:                s.d.IDs.NewID(),
		UserID:            p.user.ID,
		Address:           in.Email,
		AddressNormalized: identity.NormalizeEmail(in.Email),
		AddedAt:           p.user.CreatedAt,
	}
	roles := s.builtinRoles()
	grantID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		var admin identity.Role
		for _, r := range roles {
			stored, err := s.d.Repository.EnsureRole(ctx, tx, r.role, r.perms)
			if err != nil {
				return err
			}
			if stored.Key == identity.RoleAdmin {
				admin = stored
			}
		}
		if err := s.writeUser(ctx, tx, p); err != nil {
			return err
		}
		if err := s.d.Repository.InsertEmailAddress(ctx, tx, email); err != nil {
			return err
		}
		if err := s.d.Repository.InsertPrimaryEmail(ctx, tx, email); err != nil {
			return err
		}
		return s.d.Repository.InsertRoleGrant(ctx, tx, identity.RoleGrant{
			ID: grantID, UserID: p.user.ID, RoleID: admin.ID, GrantedAt: p.user.CreatedAt,
		})
	})
	if err != nil {
		return Registration{}, err
	}
	return p.registration(), nil
}

// preparedUser is a new user with everything slow (hashing, key generation) done, ready to be
// written by writeUser; a retried transaction writes the same values.
type preparedUser struct {
	user  identity.User
	cred  identity.PasswordCredential
	prefs identity.Preferences
	acct  *account.PreparedLocalAccount
}

func (p *preparedUser) registration() Registration {
	return Registration{User: p.user, AccountID: p.acct.Account.ID}
}

func (s *Service) prepareUser(ctx context.Context, in RegisterInput) (*preparedUser, error) {
	if err := identity.ValidatePassword(in.Password); err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	user := identity.User{ID: s.d.IDs.NewID(), CreatedAt: now}
	prefs, err := s.defaultPreferences(ctx, user.ID, in.Locale, now)
	if err != nil {
		return nil, err
	}
	acct, err := s.d.Accounts.PrepareLocalAccount(ctx, account.LocalAccountInput{
		UserID:      user.ID,
		Username:    in.Username,
		DisplayName: in.DisplayName,
	})
	if err != nil {
		return nil, err
	}
	hash, err := s.d.Passwords.Hash(ctx, in.Password)
	if err != nil {
		return nil, err
	}
	cred := identity.PasswordCredential{ID: s.d.IDs.NewID(), UserID: user.ID, Hash: hash, SetAt: now}
	return &preparedUser{user: user, cred: cred, prefs: prefs, acct: acct}, nil
}

func (s *Service) writeUser(ctx context.Context, tx uow.Tx, p *preparedUser) error {
	if err := s.d.Repository.InsertUser(ctx, tx, p.user); err != nil {
		return err
	}
	if err := s.d.Repository.InsertPasswordCredential(ctx, tx, p.cred); err != nil {
		return err
	}
	if err := s.d.Repository.InsertPreferences(ctx, tx, p.prefs); err != nil {
		return err
	}
	return s.d.Accounts.WriteLocalAccount(ctx, tx, p.acct)
}

type roleToEnsure struct {
	role  identity.Role
	perms []identity.Permission
}

// builtinRoles returns the built-in roles with fresh IDs, for EnsureRole.
func (s *Service) builtinRoles() []roleToEnsure {
	now := s.d.Clock.Now()
	builtins := identity.BuiltinRoles()
	out := make([]roleToEnsure, len(builtins))
	for i, b := range builtins {
		out[i] = roleToEnsure{
			role:  identity.Role{ID: s.d.IDs.NewID(), Key: b.Key, Position: b.Position, CreatedAt: now},
			perms: b.Permissions,
		}
	}
	return out
}

// EnsureBuiltinRoles creates the built-in roles (admin and moderator) with their permissions,
// or adds the permissions they lack. It is safe to run on every start.
func (s *Service) EnsureBuiltinRoles(ctx context.Context) error {
	roles := s.builtinRoles()
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		for _, r := range roles {
			if _, err := s.d.Repository.EnsureRole(ctx, tx, r.role, r.perms); err != nil {
				return err
			}
		}
		return nil
	})
}
