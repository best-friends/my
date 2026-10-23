// SPDX-License-Identifier: AGPL-3.0-only

package kernel_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"testing"

	"github.com/mycroblog/my/internal/app/kernel"
)

// fakeKeyManager wraps with AES-256-GCM under fixed keys. The kernel's tests cannot use the
// adapters (internal/adapter/keymanager), so this is a minimal stand-in.
type fakeKeyManager struct {
	current string
	keys    map[string]cipher.AEAD
}

func newFakeKeyManager(t *testing.T, current string, ids ...string) *fakeKeyManager {
	t.Helper()
	km := &fakeKeyManager{current: current, keys: map[string]cipher.AEAD{}}
	for _, id := range append([]string{current}, ids...) {
		key := bytes.Repeat([]byte(id[:1]), 32)
		block, err := aes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		aead, err := cipher.NewGCMWithRandomNonce(block)
		if err != nil {
			t.Fatal(err)
		}
		km.keys[id] = aead
	}
	return km
}

func (k *fakeKeyManager) CurrentKeyID() string { return k.current }

func (k *fakeKeyManager) WrapKey(_ context.Context, dek, aad []byte) (string, []byte, error) {
	return k.current, k.keys[k.current].Seal(nil, nil, dek, aad), nil //nolint:gosec // G407 false positive: NewGCMWithRandomNonce generates the nonce and requires a nil one
}

func (k *fakeKeyManager) UnwrapKey(_ context.Context, id string, wrapped, aad []byte) ([]byte, error) {
	aead, ok := k.keys[id]
	if !ok {
		return nil, kernel.ErrUnknownKey
	}
	dek, err := aead.Open(nil, nil, wrapped, aad)
	if err != nil {
		return nil, kernel.ErrDecrypt
	}
	return dek, nil
}

var (
	ctx    = context.Background()
	aad    = kernel.AAD("01890a5d-ac96-7000-8000-000000000001", "private_key")
	secret = []byte("-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----\n")
)

func TestSealOpen(t *testing.T) {
	s := kernel.NewSealer(newFakeKeyManager(t, "a"))
	for _, plaintext := range [][]byte{secret, {}, bytes.Repeat([]byte{0}, 4096)} {
		sealed, err := s.Seal(ctx, plaintext, aad)
		if err != nil {
			t.Fatalf("Seal() error = %v", err)
		}
		if len(plaintext) > 0 && bytes.Contains(sealed, plaintext) {
			t.Fatalf("Seal() output contains the plaintext")
		}
		got, err := s.Open(ctx, sealed, aad)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Fatalf("Open() = %q, want %q", got, plaintext)
		}
		if id, err := kernel.SealedKeyID(sealed); err != nil || id != "a" {
			t.Fatalf("SealedKeyID() = %q, %v; want \"a\"", id, err)
		}
	}
}

func TestSealIsRandomized(t *testing.T) {
	s := kernel.NewSealer(newFakeKeyManager(t, "a"))
	first, _ := s.Seal(ctx, secret, aad)
	second, _ := s.Seal(ctx, secret, aad)
	if bytes.Equal(first, second) {
		t.Error("Seal() returned the same output twice")
	}
}

func TestOpenRejectsOtherAAD(t *testing.T) {
	s := kernel.NewSealer(newFakeKeyManager(t, "a"))
	sealed, _ := s.Seal(ctx, secret, aad)
	for _, other := range [][]byte{nil, kernel.AAD("01890a5d-ac96-7000-8000-000000000002", "private_key"), kernel.AAD("01890a5d-ac96-7000-8000-000000000001", "other_column")} {
		if _, err := s.Open(ctx, sealed, other); !errors.Is(err, kernel.ErrDecrypt) {
			t.Errorf("Open() with AAD %q: error = %v, want ErrDecrypt", other, err)
		}
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	s := kernel.NewSealer(newFakeKeyManager(t, "a", "b"))
	sealed, _ := s.Seal(ctx, secret, aad)
	for i := range sealed {
		tampered := bytes.Clone(sealed)
		tampered[i] ^= 0x01
		_, err := s.Open(ctx, tampered, aad)
		// Flipping a key ID byte names another key; every other change fails authentication.
		if !errors.Is(err, kernel.ErrDecrypt) && !errors.Is(err, kernel.ErrUnknownKey) {
			t.Fatalf("Open() with byte %d flipped: error = %v, want ErrDecrypt or ErrUnknownKey", i, err)
		}
	}
	for n := range len(sealed) {
		if _, err := s.Open(ctx, sealed[:n], aad); err == nil {
			t.Fatalf("Open() of the first %d bytes succeeded", n)
		}
	}
	if _, err := s.Open(ctx, append(bytes.Clone(sealed), 0), aad); !errors.Is(err, kernel.ErrDecrypt) {
		t.Fatalf("Open() with a trailing byte: error = %v, want ErrDecrypt", err)
	}
}

func TestOpenUnknownKey(t *testing.T) {
	sealed, _ := kernel.NewSealer(newFakeKeyManager(t, "a")).Seal(ctx, secret, aad)
	_, err := kernel.NewSealer(newFakeKeyManager(t, "b")).Open(ctx, sealed, aad)
	if !errors.Is(err, kernel.ErrUnknownKey) {
		t.Fatalf("Open() under another key: error = %v, want ErrUnknownKey", err)
	}
}

func TestRewrap(t *testing.T) {
	sealed, _ := kernel.NewSealer(newFakeKeyManager(t, "a")).Seal(ctx, secret, aad)
	rotated := kernel.NewSealer(newFakeKeyManager(t, "b", "a"))

	rewrapped, err := rotated.Rewrap(ctx, sealed, aad)
	if err != nil {
		t.Fatalf("Rewrap() error = %v", err)
	}
	if id, _ := kernel.SealedKeyID(rewrapped); id != "b" {
		t.Errorf("SealedKeyID(rewrapped) = %q, want \"b\"", id)
	}
	got, err := kernel.NewSealer(newFakeKeyManager(t, "b")).Open(ctx, rewrapped, aad)
	if err != nil || !bytes.Equal(got, secret) {
		t.Errorf("Open(rewrapped) without the old key = %q, %v; want %q", got, err, secret)
	}
	if same, err := rotated.Rewrap(ctx, rewrapped, aad); err != nil || !bytes.Equal(same, rewrapped) {
		t.Errorf("Rewrap() of a current value = %x, %v; want it unchanged", same, err)
	}

	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := rotated.Rewrap(ctx, tampered, aad); !errors.Is(err, kernel.ErrDecrypt) {
		t.Errorf("Rewrap() of a tampered body: error = %v, want ErrDecrypt", err)
	}
	if _, err := rotated.Rewrap(ctx, sealed, kernel.AAD("01890a5d-ac96-7000-8000-000000000002", "private_key")); !errors.Is(err, kernel.ErrDecrypt) {
		t.Errorf("Rewrap() with another AAD: error = %v, want ErrDecrypt", err)
	}
}

func TestSealedKeyIDMalformed(t *testing.T) {
	for _, b := range [][]byte{nil, {1}, {2, 1, 'a', 0, 0}, {1, 0, 0, 0}} {
		if _, err := kernel.SealedKeyID(b); !errors.Is(err, kernel.ErrDecrypt) {
			t.Errorf("SealedKeyID(%x): error = %v, want ErrDecrypt", b, err)
		}
	}
}

type failingKeyManager struct{ *fakeKeyManager }

func (failingKeyManager) WrapKey(context.Context, []byte, []byte) (string, []byte, error) {
	return "", nil, errors.New("kms unavailable")
}

func TestSealWrapError(t *testing.T) {
	if _, err := kernel.NewSealer(failingKeyManager{newFakeKeyManager(t, "a")}).Seal(ctx, secret, aad); err == nil {
		t.Fatal("Seal() succeeded although WrapKey failed")
	}
}

func TestAAD(t *testing.T) {
	// Length prefixes keep different part lists apart even when their concatenations agree.
	lists := [][]string{{"ab", "c"}, {"a", "bc"}, {"abc"}, {"abc", ""}, {}, {""}}
	seen := map[string]string{}
	for _, parts := range lists {
		key := string(kernel.AAD(parts...))
		if prev, dup := seen[key]; dup {
			t.Errorf("AAD(%q) = AAD(%s)", parts, prev)
		}
		seen[key] = fmt.Sprintf("%q", parts)
	}
	if got, want := kernel.AAD("ab", "c"), []byte{2, 'a', 'b', 1, 'c'}; !bytes.Equal(got, want) {
		t.Errorf("AAD(\"ab\", \"c\") = %x, want %x", got, want)
	}
}
