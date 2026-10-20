// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/i18n"
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
	// Logins keeps the accounts that moderation forbids to log in from signing in and from
	// using their tokens; nil forbids nobody.
	Logins LoginPolicy
	// DefaultLocale returns the instance's default locale, which a new user gets unless the
	// registration names one. It is called once per registration, before the transaction, so a
	// change of the instance setting applies to the next registration. Nil or an empty result
	// means i18n.SourceLocale.
	DefaultLocale func(ctx context.Context) (string, error)

	// RateLimiter limits failed sign-ins (LoginIPLimit, LoginAccountLimit) and failed codes of
	// two-factor authentication (SecondFactorLimit); SignIn needs it.
	RateLimiter *kernel.RateLimiter
	// Sealer seals the secrets of TOTP enrollments (ADR-0019 §6), and KV keeps the sign-in
	// challenges and used TOTP steps; two-factor authentication needs both.
	Sealer *kernel.Sealer
	KV     kernel.KeyValueStore
	// HumanVerifier checks the CAPTCHA token of every sign-in and sign-up (ADR-0042 §3); nil,
	// as with the captcha setting none, checks nothing.
	HumanVerifier kernel.HumanVerifier
	// Logger records the sign-ins let through while the CAPTCHA service cannot check their
	// token; nil means slog.Default().
	Logger *slog.Logger
	// SessionTTL is the lifetime of a session; zero means DefaultSessionTTL.
	SessionTTL time.Duration
	// WebClient is the first-party web client of the authorization server; a zero value
	// registers none.
	WebClient WebClient

	// RegistrationMode returns the instance's registration mode (the registration_mode
	// setting). SignUp calls it once per sign-up, so a change applies to the next one. Nil
	// means identity.RegistrationClosed.
	RegistrationMode func(ctx context.Context) (identity.RegistrationMode, error)
	// SignupBlocks screens the sign-ups from the registration page against the sign-up block
	// list (FR-719); nil screens nothing.
	SignupBlocks SignupScreen
	// Mailer sends verification emails; nil sends none and issues no verification tokens.
	Mailer kernel.Mailer
	// RequireEmailVerification keeps users whose primary address is unverified from using
	// the app (Access.Allowed). It is false when the mailer delivers no email (mail = none);
	// then a moderator's approval stands in for verification and open mode waits for one
	// (FR-002).
	RequireEmailVerification bool
	// Messages words the emails; nil means the embedded catalogs (i18n.Load).
	Messages *i18n.Bundle
	// BaseURL is the origin that links in emails point to, such as https://social.example.
	BaseURL string
	// InstanceName returns the instance's display name for readers of locale, which emails name
	// the instance by; nil or "" names it by the host of BaseURL.
	InstanceName func(ctx context.Context, locale string) string
}

// Service runs the identity module's use cases.
type Service struct {
	d Deps

	// dummy is the hash that SignIn verifies against for unknown users, made on first use.
	dummyMu sync.Mutex
	dummy   string

	// webClientReady is set once EnsureWebClient has succeeded.
	webClientReady atomic.Bool
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

// CreateAdmin creates a user with a password, a primary email address (verified, since the
// operator gave it), default preferences in the instance's default locale, the local account it
// logs in as, and a grant of the built-in admin role from the CLI (no granting user), in one
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
		// The operator gave the address on the command line; there is nobody to send a link to.
		verified := identity.EmailVerification{EmailAddressID: email.ID, VerifiedAt: email.AddedAt}
		if err := s.d.Repository.InsertEmailVerification(ctx, tx, verified); err != nil {
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

// HasPermission reports whether one of the roles that the user userID holds grants p.
func (s *Service) HasPermission(ctx context.Context, userID string, p identity.Permission) (bool, error) {
	perms, err := s.Permissions(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.Contains(perms, p), nil
}

// Permissions returns the permissions that the roles of the user userID grant, in the order of
// identity.AllPermissions; none for a user without roles.
func (s *Service) Permissions(ctx context.Context, userID string) ([]identity.Permission, error) {
	roles, err := s.d.Repository.ListUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	granted := map[identity.Permission]bool{}
	for _, r := range roles {
		_, perms, err := s.d.Repository.FindRole(ctx, r.Key)
		if err != nil {
			return nil, err
		}
		for _, p := range perms {
			granted[p] = true
		}
	}
	out := []identity.Permission{}
	for _, p := range identity.AllPermissions() {
		if granted[p] {
			out = append(out, p)
		}
	}
	return out, nil
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
