// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate "+ReferencePath)

// TestReferenceUpToDate fails when the committed reference differs from the schema.
// Regenerate it with `make config-docs`.
func TestReferenceUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "..", filepath.FromSlash(ReferencePath))
	want := Reference()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path) //nolint:gosec // fixed path inside the repository
	if err != nil {
		t.Fatalf("%v; run `make config-docs`", err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run `make config-docs`", ReferencePath)
	}
}
