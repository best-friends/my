// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/app/identity"
	domain "github.com/best-friends/my/internal/domain/identity"
)

func TestCanonicalLanguage(t *testing.T) {
	for in, want := range map[string]string{
		"en":                 "en",
		"ja-jp":              "ja-JP",
		"zh_hant_tw":         "zh-Hant-TW",
		"EN-latn-us":         "en-Latn-US",
		"iw":                 "he",
		"und":                "und",
		"de-CH-1996":         "de-CH-1996",
		"en-US-u-ca-gregory": "en-US-u-ca-gregory",
	} {
		if got, err := identity.CanonicalLanguage(in); err != nil || got != want {
			t.Errorf("CanonicalLanguage(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{
		"", "xx", "zz-ZZ", "en-", "ja--JP", "x-private", "und-JP", "abcdefghi", "en US",
		strings.Repeat("a", domain.MaxLanguageTagLength+1),
	} {
		if got, err := identity.CanonicalLanguage(bad); !errors.Is(err, domain.ErrInvalidLanguageTag) {
			t.Errorf("CanonicalLanguage(%q) = %q, %v; want ErrInvalidLanguageTag", bad, got, err)
		}
	}
}

func TestCanonicalLocaleRejectsUndetermined(t *testing.T) {
	if got, err := identity.CanonicalLocale("und"); !errors.Is(err, domain.ErrInvalidLanguageTag) {
		t.Errorf("CanonicalLocale(und) = %q, %v; want ErrInvalidLanguageTag", got, err)
	}
	if got, err := identity.CanonicalLocale("ja-jp"); err != nil || got != "ja-JP" {
		t.Errorf("CanonicalLocale(ja-jp) = %q, %v; want ja-JP", got, err)
	}
}
