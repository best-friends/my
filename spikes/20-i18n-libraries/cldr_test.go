// SPDX-License-Identifier: AGPL-3.0-only

package i18nspike

import "testing"

// TestCLDRDrift replays testdata/cldr.json (generated from the JS runtime's
// Intl data by ts/cldr.ts) through every Go engine. Differences mean the Go
// library and the webview disagree on CLDR plural rules or number formatting.
// Mismatches are logged, not failed: the report documents them.
func TestCLDRDrift(t *testing.T) {
	var fixtures []struct {
		Locale  string `json:"locale"`
		Message string `json:"message"`
		Outputs []struct {
			N    float64 `json:"n"`
			Want string  `json:"want"`
		} `json:"outputs"`
	}
	readJSON(t, "testdata/cldr.json", &fixtures)
	for _, eng := range Engines {
		total, same := 0, 0
		for _, f := range fixtures {
			for _, o := range f.Outputs {
				total++
				got, err := eng.Format(f.Locale, f.Message, map[string]any{"n": o.N})
				if err == nil && got == o.Want {
					same++
					continue
				}
				if err != nil {
					got = "error: " + err.Error()
				}
				t.Logf("%s %s n=%v\n    got  %q\n    want %q", eng.Name(), f.Locale, o.N, got, o.Want)
			}
		}
		t.Logf("%s: %d/%d outputs identical to Intl", eng.Name(), same, total)
	}
}

// TestKaptinlinCatchesCatalogMistakes shows that the recommended engine's
// strict mode can double as a catalog linter on the Go side.
func TestKaptinlinCatchesCatalogMistakes(t *testing.T) {
	k := Kaptinlin{}
	t.Run("plural category not in locale", func(t *testing.T) {
		// Japanese has only `other`; `one` is a translator mistake.
		if _, err := k.Format("ja", "{n, plural, one {# 件} other {# 件}}", map[string]any{"n": 1}); err == nil {
			t.Fatal("want error for `one` in ja plural")
		}
	})
	t.Run("missing argument", func(t *testing.T) {
		if _, err := k.Format("en", "Hello, {name}!", map[string]any{}); err == nil {
			t.Fatal("want error for missing argument")
		}
	})
	t.Run("syntax error", func(t *testing.T) {
		if _, err := k.Format("en", "{n, plural, one {#}", map[string]any{"n": 1}); err == nil {
			t.Fatal("want parse error")
		}
	})
}
