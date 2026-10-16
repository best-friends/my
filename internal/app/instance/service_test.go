// SPDX-License-Identifier: AGPL-3.0-only

package instance_test

import (
	"reflect"
	"testing"

	"github.com/best-friends/my/internal/app/instance"
)

func TestTextFallbackChain(t *testing.T) {
	for _, tc := range []struct {
		locale, def string
		want        []string
	}{
		{"ja-JP", "en", []string{"ja-JP", "ja", "en"}},
		{"ja-JP", "fr", []string{"ja-JP", "ja", "fr", "en"}},
		{"en-GB", "ja", []string{"en-GB", "en", "ja"}},
		{"en", "ja", []string{"en", "ja"}},
		{"zh-TW", "ja", []string{"zh-TW", "zh-Hant", "ja", "en"}},
		{"ja", "ja-JP", []string{"ja", "ja-JP", "en"}},
		{"", "ja", []string{"ja", "en"}},
		{"bad tag", "en", []string{"en"}},
	} {
		if got := instance.TextFallbackChain(tc.locale, tc.def); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("TextFallbackChain(%q, %q) = %q, want %q", tc.locale, tc.def, got, tc.want)
		}
	}
}
