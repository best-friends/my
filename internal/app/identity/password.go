// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the cost parameters of Argon2id (ADR-0019 §3).
type Argon2Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  int
	KeyLength   uint32
}

// DefaultArgon2Params are the defaults of ADR-0019 §3: m = 19 MiB, t = 2, p = 1, a 16-byte salt,
// and a 32-byte hash (one of the OWASP minimum configurations).
var DefaultArgon2Params = Argon2Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// ErrMalformedHash reports a stored hash that is not an Argon2id PHC string.
var ErrMalformedHash = errors.New("identity: malformed password hash")

// PasswordHasher hashes and verifies passwords with Argon2id. Each hash costs MemoryKiB of
// memory, so the hasher runs at most a bounded number of hashes at once and makes the rest wait
// (ADR-0019 §3). It is safe for concurrent use.
type PasswordHasher struct {
	params Argon2Params
	slots  chan struct{}
}

// NewPasswordHasher returns a hasher with params that runs at most concurrency hashes at once;
// concurrency <= 0 means GOMAXPROCS.
func NewPasswordHasher(params Argon2Params, concurrency int) *PasswordHasher {
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0)
	}
	return &PasswordHasher{params: params, slots: make(chan struct{}, concurrency)}
}

func (h *PasswordHasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("identity: wait for a password hashing slot: %w", ctx.Err())
	}
}

func (h *PasswordHasher) release() { <-h.slots }

// Hash returns the PHC string of password:
// $argon2id$v=19$m=<KiB>,t=<iterations>,p=<parallelism>$<salt>$<hash> (unpadded standard base64).
func (h *PasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	_, _ = rand.Read(salt) // crypto/rand.Read never returns an error
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	p := h.params
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, p.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.MemoryKiB, p.Iterations, p.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify reports whether password matches the PHC string encoded, comparing in constant time.
// needsRehash is true when encoded uses weaker parameters than the hasher's, so that the caller
// stores a new hash after a successful login.
func (h *PasswordHasher) Verify(ctx context.Context, password, encoded string) (ok, needsRehash bool, err error) {
	p, salt, want, err := parsePHC(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(want))) //nolint:gosec // G115: parsePHC bounds the length
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	cur := h.params
	weaker := p.MemoryKiB < cur.MemoryKiB || p.Iterations < cur.Iterations || p.Parallelism < cur.Parallelism ||
		len(salt) < cur.SaltLength || uint32(len(want)) < cur.KeyLength //nolint:gosec // G115: parsePHC bounds the length
	return true, weaker, nil
}

var b64 = base64.RawStdEncoding

// maxHashLength bounds the decoded salt and hash, so that a corrupt row cannot make Verify
// allocate without limit.
const maxHashLength = 1024

// Upper bounds of the cost parameters Verify accepts, for the same reason.
const (
	maxMemoryKiB  = 1 << 20 // 1 GiB
	maxIterations = 64
)

func parsePHC(s string) (p Argon2Params, salt, key []byte, err error) {
	parts := strings.Split(s, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, ErrMalformedHash
	}
	var parallelism uint32
	if n, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &parallelism); err != nil || n != 3 {
		return p, nil, nil, ErrMalformedHash
	}
	if p.MemoryKiB == 0 || p.MemoryKiB > maxMemoryKiB || p.Iterations == 0 || p.Iterations > maxIterations ||
		parallelism == 0 || parallelism > 255 {
		return p, nil, nil, ErrMalformedHash
	}
	p.Parallelism = uint8(parallelism)
	if salt, err = b64.DecodeString(parts[4]); err != nil || len(salt) == 0 || len(salt) > maxHashLength {
		return p, nil, nil, ErrMalformedHash
	}
	if key, err = b64.DecodeString(parts[5]); err != nil || len(key) == 0 || len(key) > maxHashLength {
		return p, nil, nil, ErrMalformedHash
	}
	p.SaltLength, p.KeyLength = len(salt), uint32(len(key)) //nolint:gosec // G115: bounded by maxHashLength
	return p, salt, key, nil
}
