// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// CompareGolden compares got with the golden file at path. With update set (usually from a
// -update flag of the calling test package), it writes got to path first, creating the
// directories, so that the comparison passes and the change shows in the diff.
func CompareGolden(t testing.TB, path string, got []byte, update bool) {
	t.Helper()
	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) //nolint:gosec // path is a golden file chosen by the test
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s is missing; run go test -update to create it", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file (run go test -update to accept):\n got: %s\nwant: %s", path, got, want)
	}
}
