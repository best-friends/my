// SPDX-License-Identifier: AGPL-3.0-only

package i18nspike

import (
	"testing"
	"time"
)

// TestSharedSubsetLimits pins where the Go engine diverges from FormatJS
// (FormatJS outputs are in the comments, from `intl-messageformat` on Bun).
// These features are excluded from shared namespaces by ts/check.ts.
func TestSharedSubsetLimits(t *testing.T) {
	d := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		msg     string
		args    map[string]any
		want    string // Go output; "" = error
		formatJ string // FormatJS output for reference
	}{
		{"<b>{name}</b> joined", map[string]any{"name": "A"}, "<b>A</b> joined", "rich-text callback b(chunks)"},
		{"{n, number, ::percent}", map[string]any{"n": 0.25}, "", "25%"},
		{"{n, number, ::compact-short}", map[string]any{"n": 12345}, "", "12K"},
		{"{n, number, ::currency/JPY}", map[string]any{"n": 1234}, "", "¥1,234"},
		{"{d, date, medium}", map[string]any{"d": d}, "", "Oct 16, 2026"},
		{"{d, date, short}", map[string]any{"d": d}, "10/16/2026", "10/16/26"},
		// Within the shared subset both sides agree:
		{"{n, number, percent}", map[string]any{"n": 0.25}, "25%", "25%"},
		{"{n, number, integer}", map[string]any{"n": 3.7}, "4", "4"},
	}
	for _, tt := range tests {
		got, err := Kaptinlin{}.Format("en", tt.msg, tt.args)
		if err != nil {
			got = ""
		}
		if got != tt.want {
			t.Errorf("%q: got %q (err %v), want %q", tt.msg, got, err, tt.want)
		}
	}
}
