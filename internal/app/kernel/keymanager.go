// SPDX-License-Identifier: AGPL-3.0-only

package kernel

import (
	"context"
	"errors"
)

// KeyManager wraps and unwraps data-encryption keys (DEKs) with a key-encryption key (KEK)
// that is kept outside the database (ADR-0019 §6). Use cases do not call it directly;
// they encrypt secrets such as actor private keys with a Sealer, which generates a DEK per secret
// and stores it wrapped next to the ciphertext (envelope encryption).
//
// Every KEK has an ID that is stored with each wrapped DEK. A KeyManager wraps with its current KEK
// and unwraps with any KEK it still holds, so that KEKs can be rotated: configure the new KEK as
// current and keep the old one until every secret is rewrapped (Sealer.Rewrap).
type KeyManager interface {
	// CurrentKeyID returns the ID of the KEK that WrapKey uses. It is 1 to 255 bytes long.
	CurrentKeyID() string

	// WrapKey encrypts dek under the current KEK, authenticating aad, and returns the KEK's ID
	// together with the wrapped key. Wrapping the same dek twice yields different wrapped keys.
	WrapKey(ctx context.Context, dek, aad []byte) (keyID string, wrapped []byte, err error)

	// UnwrapKey decrypts a key wrapped by WrapKey under the KEK named keyID with the same aad.
	// It returns an error wrapping ErrUnknownKey when the KeyManager does not hold that KEK,
	// and one wrapping ErrDecrypt when wrapped or aad does not authenticate.
	UnwrapKey(ctx context.Context, keyID string, wrapped, aad []byte) ([]byte, error)
}

var (
	// ErrUnknownKey reports ciphertext wrapped under a KEK the KeyManager does not hold,
	// for example after the KEK was replaced without keeping the previous one.
	ErrUnknownKey = errors.New("unknown key-encryption key")

	// ErrDecrypt reports ciphertext that is malformed, was tampered with,
	// or was sealed with different additional authenticated data.
	ErrDecrypt = errors.New("decryption failed")
)
