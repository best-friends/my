// SPDX-License-Identifier: AGPL-3.0-only

package identity

import "time"

// Session is a sign-in to the server-rendered screens (login, registration, consent), carried
// by a cookie (ADR-0019 §4). Only the SHA-256 digest of the cookie's token is stored.
type Session struct {
	ID        string
	UserID    string
	TokenHash []byte
	// UserAgent and IP describe the client that signed in, for the user's session list;
	// "" when unknown.
	UserAgent string
	IP        string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Expired reports whether the session has expired at now.
func (s Session) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt) }

// MaxUserAgentLength bounds the stored User-Agent in bytes; longer values are cut.
const MaxUserAgentLength = 512

// RevocationReason says why a session ended before it expired.
type RevocationReason string

// Revocation reasons (data-model §4.1, session_revocations.reason).
const (
	// RevokedLogout: the user signed out.
	RevokedLogout RevocationReason = "logout"
	// RevokedReplaced: the user signed in again from the same browser, which got a new session.
	RevokedReplaced RevocationReason = "replaced"
	// RevokedPasswordChanged: the user's password changed.
	RevokedPasswordChanged RevocationReason = "password_changed"
	// RevokedModeration: a moderator suspended the user's login.
	RevokedModeration RevocationReason = "moderation"
	// RevokedExpiredCleanup: the cleanup job ended an expired session.
	RevokedExpiredCleanup RevocationReason = "expired_cleanup"
)

// SessionRevocation records that a session ended (data-model §4.1, session_revocations).
type SessionRevocation struct {
	SessionID string
	Reason    RevocationReason
	RevokedAt time.Time
}

// SessionActivity is when a session was last used (data-model §4.1, session_activity).
type SessionActivity struct {
	SessionID  string
	LastSeenAt time.Time
}
