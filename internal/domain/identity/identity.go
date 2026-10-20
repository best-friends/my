// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// User is a login principal. It holds only its identity; credentials, email addresses, roles,
// and the local account it logs in as are separate facts that refer to it.
type User struct {
	ID        string
	CreatedAt time.Time
}

// PasswordCredential is one password a user set. The newest one is the current password.
type PasswordCredential struct {
	ID     string
	UserID string
	Hash   string // Argon2id PHC string
	SetAt  time.Time
}

// PasswordCredentialRemoval ends a password credential: its user removed the password, which then
// no longer signs in (FR-004). A user removes only the newest credential, and only while a passkey
// signs in instead; a newer credential sets a password again.
type PasswordCredentialRemoval struct {
	PasswordCredentialID string
	RemovedAt            time.Time
}

// Password length limits in characters (ADR-0019 §3, after the OWASP Authentication Cheat Sheet).
const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

// ErrInvalidPassword reports a password that ValidatePassword rejects.
var ErrInvalidPassword = errors.New("invalid password")

// ValidatePassword checks the length of a new password. It imposes no composition rules.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return fmt.Errorf("%w: not valid UTF-8", ErrInvalidPassword)
	}
	if n := utf8.RuneCountInString(password); n < MinPasswordLength || n > MaxPasswordLength {
		return fmt.Errorf("%w: must be %d to %d characters", ErrInvalidPassword, MinPasswordLength, MaxPasswordLength)
	}
	return nil
}
