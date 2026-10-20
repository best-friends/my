// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Passkeys (WebAuthn discoverable credentials, FR-004).

// PasskeyCredential is a WebAuthn public key credential that a user registered to sign in
// with instead of the password. A credential with a revocation no longer signs in.
type PasskeyCredential struct {
	ID     string
	UserID string
	// CredentialID is the ID that the authenticator chose for the credential, unique among
	// every credential (WebAuthn §4, credential ID).
	CredentialID []byte
	// PublicKey is the credential public key as a COSE_Key (RFC 9052 §7).
	PublicKey []byte
	// BackupEligible is the BE flag of the registration: whether the credential can be synced
	// to other devices. It never changes for a credential (WebAuthn §6.1.3).
	BackupEligible bool
	// Name is what the user calls the passkey, such as the device it is on.
	Name      string
	CreatedAt time.Time
}

// PasskeyCredentialRevocation ends a passkey: its user removed it.
type PasskeyCredentialRevocation struct {
	PasskeyID string
	RevokedAt time.Time
}

// Passkey limits.
const (
	// MaxPasskeyNameLength is the most characters of a passkey's name.
	MaxPasskeyNameLength = 64
	// MaxPasskeysPerUser bounds the passkeys a user has at a time.
	MaxPasskeysPerUser = 20
	// MaxCredentialIDLength is the longest credential ID that WebAuthn allows (§4).
	MaxCredentialIDLength = 1023
)

// ErrInvalidPasskeyName reports a passkey name that is empty, too long, or has control
// characters.
var ErrInvalidPasskeyName = errors.New("identity: invalid passkey name")

// NormalizePasskeyName trims the spaces around a passkey's name and checks it: one to
// MaxPasskeyNameLength characters of valid UTF-8 without control characters.
func NormalizePasskeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > MaxPasskeyNameLength ||
		strings.ContainsFunc(name, unicode.IsControl) {
		return "", ErrInvalidPasskeyName
	}
	return name, nil
}
