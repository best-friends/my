// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// ErrInvalidCredentials reports a failed sign-in. It is the same error whether the email
// address is unknown or the password is wrong, so that callers cannot reveal which.
var ErrInvalidCredentials = errors.New("identity: invalid email address or password")

// ErrInvalidSession reports a session token that is unknown, revoked, or expired.
var ErrInvalidSession = errors.New("identity: invalid session")

// Session defaults (ADR-0019 §4 leaves the lifetime open; it matches the refresh token's).
const (
	DefaultSessionTTL = 30 * 24 * time.Hour
	// SessionActivityInterval is how stale session_activity may get before Authenticate
	// updates it, so that reads do not turn into a write per request (data-model §4.1).
	SessionActivityInterval = time.Hour
	// SessionTokenPrefix marks session tokens for secret scanners (ADR-0019 §4).
	SessionTokenPrefix = "my_ses_"
)

// Login rate limits (ADR-0019 §5). Only failed attempts count.
var (
	LoginIPLimit      = kernel.Limit{Name: "login.ip", Max: 5, Window: time.Minute}
	LoginAccountLimit = kernel.Limit{Name: "login.account", Max: 10, Window: time.Hour}
)

// NewToken returns a new random token with prefix (256 bits, base64url) and its SHA-256
// digest, which is all that is stored (ADR-0019 §4).
func NewToken(prefix string) (token string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	token = prefix + base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

// HashToken returns the SHA-256 digest of a token, the key it is looked up by.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SignInInput is a sign-in attempt from a browser.
type SignInInput struct {
	Email    string
	Password string
	// IP is the client address ("" when unknown); it keys a rate limit and is stored with
	// the session.
	IP        string
	UserAgent string
	// CurrentToken is the session token the browser already holds, if any. Signing in revokes
	// it, so that the browser holds exactly the new session.
	CurrentToken string
}

// SignedIn is the result of SignIn.
type SignedIn struct {
	// Token is the new session token for the cookie; it is not stored anywhere.
	Token   string
	Session identity.Session
}

// SignIn checks an email address and password and starts a new session with a new token, so
// that a token planted before the sign-in is never promoted (session fixation, ADR-0019 §4).
//
// It fails with ErrInvalidCredentials for an unknown address or a wrong password, after the
// same amount of password hashing in both cases, and with a *kernel.RateLimitedError when too
// many attempts failed from the client's IP address or for the email address (ADR-0019 §5).
func (s *Service) SignIn(ctx context.Context, in SignInInput) (SignedIn, error) {
	if s.d.RateLimiter == nil {
		return SignedIn{}, errors.New("identity: SignIn needs a RateLimiter")
	}
	ipKey := kernel.ClientKey(in.IP)
	// No stored address is longer, so the cut only bounds the rate-limit key.
	accountKey := truncate(identity.NormalizeEmail(in.Email), identity.MaxEmailLength)
	if err := s.checkLoginLimits(ctx, ipKey, accountKey); err != nil {
		return SignedIn{}, err
	}
	cred, err := s.verifyPassword(ctx, accountKey, in.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		if rerr := s.recordLoginFailure(ctx, ipKey, accountKey); rerr != nil {
			return SignedIn{}, rerr
		}
		return SignedIn{}, err
	}
	if err != nil {
		return SignedIn{}, err
	}

	now := s.d.Clock.Now()
	token, hash := NewToken(SessionTokenPrefix)
	session := identity.Session{
		ID:        s.d.IDs.NewID(),
		UserID:    cred.cred.UserID,
		TokenHash: hash,
		UserAgent: truncate(in.UserAgent, identity.MaxUserAgentLength),
		IP:        in.IP,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL()),
	}
	var rehashed *identity.PasswordCredential
	if cred.needsRehash {
		h, err := s.d.Passwords.Hash(ctx, in.Password)
		if err != nil {
			return SignedIn{}, err
		}
		rehashed = &identity.PasswordCredential{ID: s.d.IDs.NewID(), UserID: session.UserID, Hash: h, SetAt: now}
	}
	var previous string
	if in.CurrentToken != "" {
		switch old, _, err := s.d.Repository.FindUnrevokedSession(ctx, HashToken(in.CurrentToken)); {
		case err == nil:
			previous = old.ID
		case !errors.Is(err, ErrNotFound):
			return SignedIn{}, err
		}
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if previous != "" {
			rev := identity.SessionRevocation{SessionID: previous, Reason: identity.RevokedReplaced, RevokedAt: now}
			if err := s.d.Repository.InsertSessionRevocation(ctx, tx, rev); err != nil {
				return err
			}
		}
		if rehashed != nil {
			if err := s.d.Repository.InsertPasswordCredential(ctx, tx, *rehashed); err != nil {
				return err
			}
		}
		if err := s.d.Repository.InsertSession(ctx, tx, session); err != nil {
			return err
		}
		return s.d.Repository.UpsertSessionActivity(ctx, tx, identity.SessionActivity{SessionID: session.ID, LastSeenAt: now})
	})
	if err != nil {
		return SignedIn{}, err
	}
	return SignedIn{Token: token, Session: session}, nil
}

func (s *Service) sessionTTL() time.Duration {
	if s.d.SessionTTL > 0 {
		return s.d.SessionTTL
	}
	return DefaultSessionTTL
}

func (s *Service) checkLoginLimits(ctx context.Context, ipKey, accountKey string) error {
	var wait time.Duration
	for _, c := range []struct {
		lim kernel.Limit
		key string
	}{{LoginIPLimit, ipKey}, {LoginAccountLimit, accountKey}} {
		if c.key == "" {
			continue
		}
		d, err := s.d.RateLimiter.Check(ctx, c.lim, c.key)
		if err != nil {
			return err
		}
		if !d.Allowed {
			wait = max(wait, d.RetryAfter)
		}
	}
	if wait > 0 {
		return &kernel.RateLimitedError{RetryAfter: wait}
	}
	return nil
}

func (s *Service) recordLoginFailure(ctx context.Context, ipKey, accountKey string) error {
	if ipKey != "" {
		if err := s.d.RateLimiter.Record(ctx, LoginIPLimit, ipKey); err != nil {
			return err
		}
	}
	if accountKey != "" {
		return s.d.RateLimiter.Record(ctx, LoginAccountLimit, accountKey)
	}
	return nil
}

type verifiedPassword struct {
	cred        identity.PasswordCredential
	needsRehash bool
}

// verifyPassword returns the current credential of the user with the email address when
// password matches it. When the user or their password does not exist, it verifies against a
// dummy hash instead, so that the response time does not tell (ADR-0019 §3).
func (s *Service) verifyPassword(ctx context.Context, normalizedEmail, password string) (verifiedPassword, error) {
	var cred identity.PasswordCredential
	userID, err := s.d.Repository.FindUserIDByEmail(ctx, normalizedEmail)
	if err == nil {
		cred, err = s.d.Repository.FindCurrentPasswordCredential(ctx, userID)
	}
	if errors.Is(err, ErrNotFound) {
		dummy, err := s.dummyHash(ctx)
		if err != nil {
			return verifiedPassword{}, err
		}
		if _, _, err := s.d.Passwords.Verify(ctx, password, dummy); err != nil {
			return verifiedPassword{}, err
		}
		return verifiedPassword{}, ErrInvalidCredentials
	}
	if err != nil {
		return verifiedPassword{}, err
	}
	ok, rehash, err := s.d.Passwords.Verify(ctx, password, cred.Hash)
	if err != nil {
		return verifiedPassword{}, fmt.Errorf("identity: verify the password of user %s: %w", userID, err)
	}
	if !ok {
		return verifiedPassword{}, ErrInvalidCredentials
	}
	return verifiedPassword{cred: cred, needsRehash: rehash}, nil
}

// dummyHash returns a hash of a random password with the current parameters, made once.
func (s *Service) dummyHash(ctx context.Context) (string, error) {
	s.dummyMu.Lock()
	defer s.dummyMu.Unlock()
	if s.dummy != "" {
		return s.dummy, nil
	}
	pw, _ := NewToken("")
	h, err := s.d.Passwords.Hash(ctx, pw)
	if err != nil {
		return "", err
	}
	s.dummy = h
	return h, nil
}

// Authenticate returns the session of a token. It fails with ErrInvalidSession when the token
// is unknown, revoked, or expired. It records the session's activity at most once per
// SessionActivityInterval.
func (s *Service) Authenticate(ctx context.Context, token string) (identity.Session, error) {
	if token == "" {
		return identity.Session{}, ErrInvalidSession
	}
	session, activity, err := s.d.Repository.FindUnrevokedSession(ctx, HashToken(token))
	if errors.Is(err, ErrNotFound) {
		return identity.Session{}, ErrInvalidSession
	}
	if err != nil {
		return identity.Session{}, err
	}
	now := s.d.Clock.Now()
	if session.Expired(now) {
		return identity.Session{}, ErrInvalidSession
	}
	if now.Sub(activity.LastSeenAt) >= SessionActivityInterval {
		err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			return s.d.Repository.UpsertSessionActivity(ctx, tx, identity.SessionActivity{SessionID: session.ID, LastSeenAt: now})
		})
		if err != nil {
			return identity.Session{}, err
		}
	}
	return session, nil
}

// SignOut revokes the session of a token. Signing out with an unknown or already revoked token
// does nothing.
func (s *Service) SignOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	session, _, err := s.d.Repository.FindUnrevokedSession(ctx, HashToken(token))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	rev := identity.SessionRevocation{SessionID: session.ID, Reason: identity.RevokedLogout, RevokedAt: s.d.Clock.Now()}
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.d.Repository.InsertSessionRevocation(ctx, tx, rev) })
}

// truncate cuts s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xc0 == 0x80 {
		n--
	}
	return s[:n]
}
