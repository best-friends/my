// SPDX-License-Identifier: AGPL-3.0-only

package locales

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFS checks that FS holds the catalogs of every namespace Go formats and none of a client* namespace.
func TestFS(t *testing.T) {
	onDisk, err := filepath.Glob("*/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range onDisk {
		namespace := strings.TrimSuffix(filepath.Base(path), ".json")
		_, err := fs.Stat(FS, filepath.ToSlash(path))
		switch client := strings.HasPrefix(namespace, "client"); {
		case client && err == nil:
			t.Errorf("FS embeds %s, which only the TypeScript clients read", path)
		case !client && err != nil:
			t.Errorf("FS does not embed %s: add its pattern to the go:embed directive (%v)", path, err)
		}
	}
	if _, err := os.Stat("en/server.json"); err != nil {
		t.Fatalf("the source catalog is missing: %v", err)
	}
}
