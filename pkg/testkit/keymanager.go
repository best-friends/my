// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"

	"github.com/best-friends/my/internal/app/kernel"
)

// KeyManager is an in-memory kernel.KeyManager for tests: it wraps with AES-256-GCM under a key
// derived from its key ID. It passes KeyManagerContract but protects nothing; never use it outside tests.
type KeyManager struct {
	id   string
	aead cipher.AEAD
}

var _ kernel.KeyManager = (*KeyManager)(nil)

// NewKeyManager returns a KeyManager whose only KEK has the given ID.
func NewKeyManager(id string) *KeyManager {
	key := sha256.Sum256([]byte("testkit/kek/" + id))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		panic(err)
	}
	return &KeyManager{id: id, aead: aead}
}

// CurrentKeyID implements kernel.KeyManager.
func (k *KeyManager) CurrentKeyID() string { return k.id }

// WrapKey implements kernel.KeyManager.
func (k *KeyManager) WrapKey(_ context.Context, dek, aad []byte) (string, []byte, error) {
	return k.id, k.aead.Seal(nil, nil, dek, aad), nil //nolint:gosec // G407 false positive: NewGCMWithRandomNonce generates the nonce and requires a nil one
}

// UnwrapKey implements kernel.KeyManager.
func (k *KeyManager) UnwrapKey(_ context.Context, id string, wrapped, aad []byte) ([]byte, error) {
	if id != k.id {
		return nil, fmt.Errorf("key %q: %w", id, kernel.ErrUnknownKey)
	}
	dek, err := k.aead.Open(nil, nil, wrapped, aad)
	if err != nil {
		return nil, kernel.ErrDecrypt
	}
	return dek, nil
}
