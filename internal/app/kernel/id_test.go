// SPDX-License-Identifier: AGPL-3.0-only

package kernel

import (
	"testing"
	"time"
)

func TestValidID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical", "01a148a2-b24c-73b5-b159-fdfd429294e4", true},
		{"floor", UUIDv7Floor(time.UnixMilli(0x0123_4567_89ab)), true},
		{"uppercase", "01A148A2-B24C-73B5-B159-FDFD429294E4", false},
		{"no hyphens", "01a148a2b24c73b5b159fdfd429294e4", false},
		{"braced", "{01a148a2-b24c-73b5-b159-fdfd429294e4}", false},
		{"urn", "urn:uuid:01a148a2-b24c-73b5-b159-fdfd429294e4", false},
		{"uuidv4", "3f2b8c1e-9d4a-4e6b-8a3c-1f2e3d4c5b6a", false},
		{"bad variant", "01a148a2-b24c-73b5-c159-fdfd429294e4", false},
		{"trailing newline", "01a148a2-b24c-73b5-b159-fdfd429294e4\n", false},
		{"empty", "", false},
		{"garbage", "not-an-id", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidID(tt.in); got != tt.want {
				t.Errorf("ValidID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
