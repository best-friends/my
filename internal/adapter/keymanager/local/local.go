// SPDX-License-Identifier: AGPL-3.0-only

// Package local implements the KeyManager port of internal/app/kernel with key-encryption keys
// (KEKs) given in the configuration (`key_manager = local`, `kek`, `kek_previous`; ADR-0019 §6).
//
// Each KEK is a 32-byte AES-256 key. Wrapped keys are AES-256-GCM with a random 96-bit nonce
// prepended. A KEK's ID is derived from the key itself, so operators never assign IDs and
// the same KEK always has the same ID on every process and after a restore.
package local

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/mycroblog/my/internal/app/kernel"
)

// KEKSize is the length in bytes of a key-encryption key.
const KEKSize = 32

// keyIDPrefix names the key ID derivation; it separates the ID hash from any other use of the KEK.
const keyIDPrefix = "my/local-kek-id/v1\x00"

// KeyManager holds one current KEK for wrapping and any number of previous KEKs for unwrapping.
// It never prints its keys: String and LogValue show only the current key ID.
type KeyManager struct {
	current string
	keys    map[string]cipher.AEAD
}

var _ kernel.KeyManager = (*KeyManager)(nil)

// New returns a KeyManager that wraps with current and unwraps with current or any of previous.
// Every KEK must be KEKSize bytes, and no KEK may be given twice.
func New(current []byte, previous ...[]byte) (*KeyManager, error) {
	km := &KeyManager{keys: make(map[string]cipher.AEAD, 1+len(previous))}
	for i, kek := range append([][]byte{current}, previous...) {
		if len(kek) != KEKSize {
			return nil, fmt.Errorf("local key manager: KEK #%d must be %d bytes, got %d", i, KEKSize, len(kek))
		}
		id := KeyID(kek)
		if _, dup := km.keys[id]; dup {
			return nil, fmt.Errorf("local key manager: KEK %s is given more than once", id)
		}
		block, err := aes.NewCipher(kek)
		if err != nil {
			return nil, fmt.Errorf("local key manager: %w", err)
		}
		aead, err := cipher.NewGCMWithRandomNonce(block)
		if err != nil {
			return nil, fmt.Errorf("local key manager: %w", err)
		}
		km.keys[id] = aead
		if i == 0 {
			km.current = id
		}
	}
	return km, nil
}

// KeyID returns the ID of kek: "local:" followed by the first 8 bytes of
// SHA-256(prefix || kek) in hex. The ID reveals nothing usable about a random 256-bit key.
func KeyID(kek []byte) string {
	h := sha256.New()
	h.Write([]byte(keyIDPrefix))
	h.Write(kek)
	return "local:" + hex.EncodeToString(h.Sum(nil)[:8])
}

// CurrentKeyID returns the ID of the KEK that WrapKey uses.
func (k *KeyManager) CurrentKeyID() string {
	return k.current
}

// WrapKey encrypts dek under the current KEK.
func (k *KeyManager) WrapKey(_ context.Context, dek, aad []byte) (string, []byte, error) {
	return k.current, k.keys[k.current].Seal(nil, nil, dek, aad), nil //nolint:gosec // G407 false positive: NewGCMWithRandomNonce generates the nonce and requires a nil one
}

// UnwrapKey decrypts a key wrapped under the KEK named keyID.
func (k *KeyManager) UnwrapKey(_ context.Context, keyID string, wrapped, aad []byte) ([]byte, error) {
	aead, ok := k.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("local key manager: key %q is not configured: %w", keyID, kernel.ErrUnknownKey)
	}
	dek, err := aead.Open(nil, nil, wrapped, aad)
	if err != nil {
		return nil, fmt.Errorf("local key manager: key %q: %w", keyID, kernel.ErrDecrypt)
	}
	return dek, nil
}

// String describes the KeyManager without its keys.
func (k *KeyManager) String() string {
	return fmt.Sprintf("local.KeyManager{current: %s, keys: %d}", k.current, len(k.keys))
}

// LogValue keeps the keys out of logs (ADR-0019 §6).
func (k *KeyManager) LogValue() slog.Value {
	return slog.GroupValue(slog.String("current", k.current), slog.Int("keys", len(k.keys)))
}
