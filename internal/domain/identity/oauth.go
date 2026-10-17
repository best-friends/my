// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// OAuthClient is an application registered with the authorization server (data-model §4.1,
// oauth_clients). Other records refer to it by ID; ClientID is the public identifier that the
// client sends.
type OAuthClient struct {
	ID       string
	ClientID string
	// SecretHash is the SHA-256 digest of the client secret of a confidential client, and nil
	// for a public client (ADR-0019 §4).
	SecretHash []byte
	Name       string
	// Website is the client's home page; "" when it has none.
	Website string
	// Scope is the most that the client may be granted.
	Scope Scope
	// FirstParty clients are this server's own clients; users are not asked to consent to them.
	FirstParty bool
	// OwnerUserID is the user who registered the client; "" for clients the server registers.
	OwnerUserID string
	CreatedAt   time.Time
}

// Public reports whether the client has no secret and so cannot authenticate itself.
func (c OAuthClient) Public() bool { return c.SecretHash == nil }

// AuthorizationCode is a one-time code that a client exchanges for tokens (RFC 6749 §4.1).
// Only the SHA-256 digest of the code is stored.
type AuthorizationCode struct {
	CodeHash []byte
	// ClientID is the ID of the OAuthClient (not its public client_id).
	ClientID    string
	UserID      string
	RedirectURI string
	Scope       Scope
	// CodeChallenge is the S256 PKCE challenge (RFC 7636 §4.2).
	CodeChallenge string
	// SessionID is the browser session in which the user authorized the client; "" when the
	// code was issued outside one.
	SessionID string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Expired reports whether the code has expired at now.
func (c AuthorizationCode) Expired(now time.Time) bool { return !now.Before(c.ExpiresAt) }

// AuthorizationCodeRedemption records that a code was exchanged (data-model §4.1). TokenID is
// the first token of the family that the exchange started.
type AuthorizationCodeRedemption struct {
	CodeHash   []byte
	TokenID    string
	RedeemedAt time.Time
}

// AccessToken is one issuance of an access token and, optionally, a refresh token
// (data-model §4.1, oauth_access_tokens). Refreshing issues a new AccessToken in the same
// family and revokes the old one; FamilyID is the ID of the family's first token.
type AccessToken struct {
	ID       string
	FamilyID string
	// ClientID is the ID of the OAuthClient (not its public client_id).
	ClientID        string
	UserID          string
	AccessTokenHash []byte
	// RefreshTokenHash is nil when no refresh token was issued.
	RefreshTokenHash []byte
	Scope            Scope
	// RotatedFromTokenID is the token whose refresh token this one replaced; "" for the first.
	RotatedFromTokenID string
	IssuedAt           time.Time
	// ExpiresAt is when the access token expires.
	ExpiresAt time.Time
	// RefreshExpiresAt is when the refresh token expires; zero when there is none.
	RefreshExpiresAt time.Time
}

// AccessExpired reports whether the access token has expired at now.
func (t AccessToken) AccessExpired(now time.Time) bool { return !now.Before(t.ExpiresAt) }

// RefreshExpired reports whether the refresh token has expired at now, or there is none.
func (t AccessToken) RefreshExpired(now time.Time) bool {
	return t.RefreshTokenHash == nil || !now.Before(t.RefreshExpiresAt)
}

// TokenRevocationReason says why a token stopped working.
type TokenRevocationReason string

// Token revocation reasons (data-model §4.1, oauth_token_revocations.reason).
const (
	// TokenRevokedUser: the client or the user revoked the token (RFC 7009).
	TokenRevokedUser TokenRevocationReason = "user"
	// TokenRevokedRotation: the refresh token was used and replaced by a new token.
	TokenRevokedRotation TokenRevocationReason = "rotation"
	// TokenRevokedRefreshReuse: a used refresh token of the family was presented again, so the
	// whole family was revoked (RFC 9700 §4.14.2).
	TokenRevokedRefreshReuse TokenRevocationReason = "refresh_reuse"
	// TokenRevokedCodeReuse: the authorization code that started the family was presented
	// again, so the whole family was revoked (RFC 6749 §4.1.2).
	TokenRevokedCodeReuse TokenRevocationReason = "code_reuse"
	// TokenRevokedPasswordChanged: the user's password changed.
	TokenRevokedPasswordChanged TokenRevocationReason = "password_changed"
	// TokenRevokedModeration: a moderator suspended the user's login.
	TokenRevokedModeration TokenRevocationReason = "moderation"
)

// TokenRevocation records that a token stopped working, both its access token and its refresh
// token (data-model §4.1, oauth_token_revocations).
type TokenRevocation struct {
	TokenID   string
	Reason    TokenRevocationReason
	RevokedAt time.Time
}

// Scopes the server grants. The client API checks them per operation (api/openapi.yaml lists
// the scopes of each operation). The broad scopes read, write, follow, and admin grant the
// fine-grained scopes under them; push grants Web Push only. read and write do not grant the
// admin scopes, whose operations also need a permission of the user's roles.
const (
	ScopeAdmin  = "admin"
	ScopeRead   = "read"
	ScopeWrite  = "write"
	ScopeFollow = "follow"
	ScopePush   = "push"
)

// Fine-grained scopes. "read:x" is granted by read and "write:x" by write; follow grants the
// scopes of follows, blocks, and mutes; admin grants "admin:read" and "admin:write".
const (
	ScopeAdminRead          = "admin:read"
	ScopeAdminWrite         = "admin:write"
	ScopeReadAccounts       = "read:accounts"
	ScopeReadBlocks         = "read:blocks"
	ScopeReadBookmarks      = "read:bookmarks"
	ScopeReadFavourites     = "read:favourites"
	ScopeReadFollows        = "read:follows"
	ScopeReadMutes          = "read:mutes"
	ScopeReadNotifications  = "read:notifications"
	ScopeReadStatuses       = "read:statuses"
	ScopeWriteAccounts      = "write:accounts"
	ScopeWriteBlocks        = "write:blocks"
	ScopeWriteBookmarks     = "write:bookmarks"
	ScopeWriteFavourites    = "write:favourites"
	ScopeWriteFollows       = "write:follows"
	ScopeWriteMedia         = "write:media"
	ScopeWriteMutes         = "write:mutes"
	ScopeWriteNotifications = "write:notifications"
	ScopeWriteReports       = "write:reports"
	ScopeWriteStatuses      = "write:statuses"
)

// broadScopes are the scopes that AllScopes holds; together they grant every scope.
var broadScopes = []string{ScopeAdmin, ScopeFollow, ScopePush, ScopeRead, ScopeWrite}

// knownScopes lists every scope, sorted.
var knownScopes = []string{
	ScopeAdmin, ScopeAdminRead, ScopeAdminWrite,
	ScopeFollow, ScopePush, ScopeRead,
	ScopeReadAccounts, ScopeReadBlocks, ScopeReadBookmarks, ScopeReadFavourites, ScopeReadFollows,
	ScopeReadMutes, ScopeReadNotifications, ScopeReadStatuses,
	ScopeWrite,
	ScopeWriteAccounts, ScopeWriteBlocks, ScopeWriteBookmarks, ScopeWriteFavourites, ScopeWriteFollows,
	ScopeWriteMedia, ScopeWriteMutes, ScopeWriteNotifications, ScopeWriteReports, ScopeWriteStatuses,
}

// followScopes are the fine-grained scopes that follow grants.
var followScopes = []string{ScopeReadBlocks, ScopeReadFollows, ScopeReadMutes, ScopeWriteBlocks, ScopeWriteFollows, ScopeWriteMutes}

// KnownScopes returns every scope the server knows, sorted.
func KnownScopes() []string { return slices.Clone(knownScopes) }

// ErrInvalidScope reports a scope string that ParseScope rejects.
var ErrInvalidScope = errors.New("invalid scope")

// Scope is a set of scopes in its normal form: the scope tokens sorted, without duplicates,
// separated by one space (RFC 6749 §3.3). It is stored and compared as a whole.
type Scope string

// AllScopes returns the scope that grants every scope the server knows: the broad scopes.
func AllScopes() Scope { return Scope(strings.Join(broadScopes, " ")) }

// ParseScope parses a space-separated list of scopes into its normal form. It rejects unknown
// scopes; an empty list is the empty Scope.
func ParseScope(s string) (Scope, error) {
	if s == "" {
		return "", nil
	}
	tokens := strings.Split(s, " ")
	for _, t := range tokens {
		if t == "" {
			return "", fmt.Errorf("%w: empty scope token", ErrInvalidScope)
		}
		if !slices.Contains(knownScopes, t) {
			return "", fmt.Errorf("%w: unknown scope %q", ErrInvalidScope, t)
		}
	}
	slices.Sort(tokens)
	return Scope(strings.Join(slices.Compact(tokens), " ")), nil
}

// Tokens returns the scope tokens, sorted.
func (s Scope) Tokens() []string {
	if s == "" {
		return nil
	}
	return strings.Split(string(s), " ")
}

// Has reports whether s holds the scope token t itself; Grants also follows the broad scopes.
func (s Scope) Has(t string) bool { return slices.Contains(s.Tokens(), t) }

// Grants reports whether s allows what the scope t allows: s holds t, or a broad scope over it
// (read over "read:x", write over "write:x", admin over "admin:x", follow over the follow,
// block, and mute scopes).
func (s Scope) Grants(t string) bool {
	if s.Has(t) {
		return true
	}
	if broad, _, ok := strings.Cut(t, ":"); ok && s.Has(broad) {
		return true
	}
	return slices.Contains(followScopes, t) && s.Has(ScopeFollow)
}

// Within reports whether outer grants every scope of s.
func (s Scope) Within(outer Scope) bool {
	for _, t := range s.Tokens() {
		if !outer.Grants(t) {
			return false
		}
	}
	return true
}

// PKCE (RFC 7636). Only the S256 method is accepted; plain is refused (ADR-0019 §4).
const (
	CodeChallengeMethodS256 = "S256"
	// Verifier lengths of RFC 7636 §4.1.
	MinCodeVerifierLength = 43
	MaxCodeVerifierLength = 128
	// codeChallengeLength is the length of an unpadded base64url SHA-256 digest.
	codeChallengeLength = 43
)

// ValidCodeChallenge reports whether c has the form of an S256 code challenge: the unpadded
// base64url encoding of 32 bytes.
func ValidCodeChallenge(c string) bool {
	if len(c) != codeChallengeLength {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	return err == nil && len(b) == sha256.Size
}

// ValidCodeVerifier reports whether v is a code verifier of RFC 7636 §4.1: 43 to 128 characters
// of [A-Za-z0-9-._~].
func ValidCodeVerifier(v string) bool {
	if len(v) < MinCodeVerifierLength || len(v) > MaxCodeVerifierLength {
		return false
	}
	for _, c := range []byte(v) {
		ok := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
		if !ok {
			return false
		}
	}
	return true
}

// S256Challenge returns the S256 code challenge of a verifier: BASE64URL(SHA256(verifier)).
func S256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// VerifyCodeVerifier reports whether verifier is valid and matches the S256 challenge.
func VerifyCodeVerifier(verifier, challenge string) bool {
	if !ValidCodeVerifier(verifier) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(S256Challenge(verifier)), []byte(challenge)) == 1
}
