// SPDX-License-Identifier: AGPL-3.0-only

package fs_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/adapter/blob/fs"
	"github.com/best-friends/my/internal/app/blob"
	"github.com/best-friends/my/pkg/testkit"
)

func newStore(t *testing.T, dir string) *fs.Store {
	t.Helper()
	s, err := fs.New(fs.Options{Dir: dir, BaseURL: "https://example.com/"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBlobStoreContract(t *testing.T) {
	testkit.BlobStoreContract(t, func(t *testing.T) blob.Store { return newStore(t, t.TempDir()) })
}

func TestPublicURLJoinsTheBaseURL(t *testing.T) {
	s := newStore(t, t.TempDir())
	if got, want := s.PublicURL("media/x/full.jpg"), "https://example.com/media/x/full.jpg"; got != want {
		t.Errorf("PublicURL = %q, want %q", got, want)
	}
}

// A symbolic link inside the store that points outside it is never followed.
func TestDoesNotFollowLinksOutOfTheStore(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newStore(t, dir)
	if err := os.Symlink(outside, filepath.Join(dir, "objects", "escape")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if r, _, err := s.Get(t.Context(), "escape/secret"); err == nil {
		b, _ := io.ReadAll(r)
		_ = r.Close()
		t.Fatalf("Get through a link out of the store read %q", b)
	}
	if _, err := s.Put(t.Context(), "escape/new", strings.NewReader("x"), blob.PutOptions{}); err == nil {
		t.Fatal("Put through a link out of the store succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatalf("a file was written outside the store: %v", err)
	}
}

// Blobs are plain files under objects/, so that an operator can back them up or serve them
// with a web server.
func TestStoresPlainFiles(t *testing.T) {
	dir := t.TempDir()
	s := newStore(t, dir)
	if _, err := s.Put(t.Context(), "media/x/full.jpg", strings.NewReader("jpeg"), blob.PutOptions{ContentType: "image/jpeg"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "objects", "media", "x", "full.jpg")) //nolint:gosec // the test's own temporary directory
	if err != nil || string(b) != "jpeg" {
		t.Fatalf("object file = %q, %v", b, err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "tmp"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("tmp/ has %d leftovers, %v", len(entries), err)
	}
	if err := s.Delete(t.Context(), "media/x/full.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "objects", "media")); !os.IsNotExist(err) {
		t.Errorf("empty directories remain after Delete: %v", err)
	}
}
