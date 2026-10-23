// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/app/identity"
)

var cheap = identity.Argon2Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func TestHashUsesTheADRDefaults(t *testing.T) {
	h := identity.NewPasswordHasher(identity.DefaultArgon2Params, 1)
	got, err := h.Hash(t.Context(), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	// ADR-0019 §3: m = 19456 KiB, t = 2, p = 1, 16-byte salt, 32-byte hash.
	want := regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	if !want.MatchString(got) {
		t.Fatalf("Hash() = %q, want a PHC string with the ADR-0019 parameters", got)
	}
	if ok, rehash, err := h.Verify(t.Context(), "correct horse", got); !ok || rehash || err != nil {
		t.Fatalf("Verify() = %v, %v, %v; want true, false, nil", ok, rehash, err)
	}
}

func TestVerify(t *testing.T) {
	h := identity.NewPasswordHasher(cheap, 0)
	hash, err := h.Hash(t.Context(), "pässwörd 🔑")
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := h.Hash(t.Context(), "pässwörd 🔑"); again == hash {
		t.Error("Hash() is not salted")
	}
	if ok, _, err := h.Verify(t.Context(), "pässwörd 🔑", hash); !ok || err != nil {
		t.Errorf("Verify(right) = %v, %v", ok, err)
	}
	if ok, _, err := h.Verify(t.Context(), "passwörd 🔑", hash); ok || err != nil {
		t.Errorf("Verify(wrong) = %v, %v; want false, nil", ok, err)
	}
}

func TestVerifyAsksForRehashOfWeakerHashes(t *testing.T) {
	weak, err := identity.NewPasswordHasher(cheap, 0).Hash(t.Context(), "password")
	if err != nil {
		t.Fatal(err)
	}
	stronger := cheap
	stronger.Iterations = 2
	ok, rehash, err := identity.NewPasswordHasher(stronger, 0).Verify(t.Context(), "password", weak)
	if !ok || !rehash || err != nil {
		t.Fatalf("Verify() = %v, %v, %v; want true, true, nil", ok, rehash, err)
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h := identity.NewPasswordHasher(cheap, 0)
	good, _ := h.Hash(t.Context(), "password")
	parts := strings.Split(good, "$")
	for _, bad := range []string{
		"",
		"plaintext",
		strings.Replace(good, "argon2id", "argon2i", 1),
		strings.Replace(good, "v=19", "v=16", 1),
		strings.Replace(good, parts[3], "m=0,t=1,p=1", 1),
		strings.Replace(good, parts[3], "m=4194304,t=1,p=1", 1), // 4 GiB
		strings.Replace(good, parts[3], "m=64,t=1000,p=1", 1),
		strings.Replace(good, parts[4], "!!", 1),
		good + "$extra",
	} {
		if _, _, err := h.Verify(t.Context(), "password", bad); !errors.Is(err, identity.ErrMalformedHash) {
			t.Errorf("Verify(%q) error = %v, want ErrMalformedHash", bad, err)
		}
	}
}
