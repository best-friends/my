// SPDX-License-Identifier: AGPL-3.0-only

package i18nspike

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type testCase struct {
	Key  string         `json:"key"`
	Args map[string]any `json:"args"`
}

var goldenLocales = []string{"en", "ja", "en-XA", "ar-XB"}

func loadFixtures(t *testing.T) (Catalog, []testCase, map[string][]string) {
	t.Helper()
	catalog, err := LoadCatalog(Locales)
	if err != nil {
		t.Fatal(err)
	}
	var cases []testCase
	readJSON(t, "testdata/cases.json", &cases)
	var golden map[string][]string // produced by `bun run ts/format.ts --write` (FormatJS)
	readJSON(t, "testdata/golden.json", &golden)
	return catalog, cases, golden
}

func readJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

// TestParityWithFormatJS formats the shared catalog with each Go engine and
// compares the result with FormatJS output. Only the recommended engine is
// required to match; the others are reported so the comparison table in
// docs/spikes/20-i18n-libraries.md can be reproduced with `go test -v`.
func TestParityWithFormatJS(t *testing.T) {
	catalog, cases, golden := loadFixtures(t)
	required := Kaptinlin{}.Name()
	for _, eng := range Engines {
		t.Run(strings.ReplaceAll(eng.Name(), "/", "_"), func(t *testing.T) {
			var mismatches []string
			for _, locale := range goldenLocales {
				for i, c := range cases {
					got, err := eng.Format(FormattingLocale(locale), catalog[locale]["demo"][c.Key].DefaultMessage, c.Args)
					want := golden[locale][i]
					if err != nil {
						mismatches = append(mismatches, fmt.Sprintf("%s %s %v: error: %v", locale, c.Key, c.Args, err))
					} else if got != want {
						mismatches = append(mismatches, fmt.Sprintf("%s %s %v:\n    got  %q\n    want %q", locale, c.Key, c.Args, got, want))
					}
				}
			}
			total := len(goldenLocales) * len(cases)
			t.Logf("%s: %d/%d outputs identical to FormatJS", eng.Name(), total-len(mismatches), total)
			for _, m := range mismatches {
				if eng.Name() == required {
					t.Error(m)
				} else {
					t.Log(m)
				}
			}
		})
	}
}
