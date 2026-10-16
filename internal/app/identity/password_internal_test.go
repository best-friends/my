// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHashWaitsForAFreeSlot(t *testing.T) {
	h := NewPasswordHasher(Argon2Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}, 1)
	h.slots <- struct{}{} // another hash holds the only slot

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := h.Hash(ctx, "password"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Hash() with no free slot = %v, want context.DeadlineExceeded", err)
	}

	h.release()
	if _, err := h.Hash(t.Context(), "password"); err != nil {
		t.Fatalf("Hash() after the slot is freed = %v", err)
	}
}
