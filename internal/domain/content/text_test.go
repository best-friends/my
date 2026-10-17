// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/domain/content"
)

func TestValidText(t *testing.T) {
	for s, want := range map[string]bool{"": true, "あ\n\t": true, "a\x00b": false, "a\xffb": false} {
		if got := content.ValidText(s); got != want {
			t.Errorf("ValidText(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestValidateIdempotencyKey(t *testing.T) {
	for _, ok := range []string{"", "key-1", strings.Repeat("あ", content.MaxIdempotencyKeyLength)} {
		if err := content.ValidateIdempotencyKey(ok); err != nil {
			t.Errorf("ValidateIdempotencyKey(%d chars) = %v", len([]rune(ok)), err)
		}
	}
	for _, bad := range []string{"k\x00", "k\xff", strings.Repeat("k", content.MaxIdempotencyKeyLength+1)} {
		if err := content.ValidateIdempotencyKey(bad); !errors.Is(err, content.ErrInvalidIdempotencyKey) {
			t.Errorf("ValidateIdempotencyKey(%q...) = %v, want ErrInvalidIdempotencyKey", bad[:2], err)
		}
	}
}

func TestValidateMediaDescription(t *testing.T) {
	if err := content.ValidateMediaDescription(strings.Repeat("あ", content.MaxMediaDescriptionLength)); err != nil {
		t.Errorf("a description at the limit: %v", err)
	}
	if err := content.ValidateMediaDescription(strings.Repeat("あ", content.MaxMediaDescriptionLength+1)); !errors.Is(err, content.ErrDescriptionTooLong) {
		t.Errorf("a long description: %v, want ErrDescriptionTooLong", err)
	}
	for _, bad := range []string{"a\x00b", "a\xffb"} {
		if err := content.ValidateMediaDescription(bad); !errors.Is(err, content.ErrInvalidDescription) {
			t.Errorf("ValidateMediaDescription(%q) = %v, want ErrInvalidDescription", bad, err)
		}
	}
}
