// SPDX-License-Identifier: AGPL-3.0-only

package ctxmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		pat, p string
		want   bool
	}{
		{"internal/**", "internal/app/content/x.go", true},
		{"internal/*/content/**", "internal/app/content/x.go", true},
		{"internal/*/content/**", "internal/app/contents/x.go", false},
		{"**/*.gen.go", "internal/interface/clientapi/api.gen.go", true},
		{"**/*.gen.go", "x.gen.go", true},
		{"*.md", "docs/a.md", false},
		{"docs/**/*.md", "docs/a.md", true},
		{"app/web/src/features/*/**", "app/web/src/features/admin/a/b.tsx", true},
	}
	for _, c := range cases {
		if got := Match(c.pat, c.p); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pat, c.p, got, c.want)
		}
	}
}

func TestCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte("# Title\n\n## 1. Rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &Map{
		Ignore: []string{"**/*.gen.go"},
		Strict: []string{"internal/app/*"},
		Entries: []Entry{
			{Name: "go", Paths: []string{"internal/**"}, Docs: []string{"doc.md#1-rules"}, Checks: []string{"go"}},
			{Name: "content", Paths: []string{"internal/*/content/**"}, Docs: []string{"doc.md#missing"}},
			{Name: "docs", Paths: []string{"*.md"}},
		},
	}
	files := []string{"doc.md", "internal/app/content/a.go", "internal/app/graph/b.go", "internal/app/graph/c.gen.go", "README.txt"}
	var got []string
	for _, p := range m.Check(root, files, []string{"go"}) {
		got = append(got, p.Path+" "+p.Msg)
	}
	want := []string{
		"./ no entry covers README.txt",
		"internal/app/graph/ matches strict pattern",
		"entry \"content\": doc.md has no heading #missing",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d problems:\n%s", len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if !strings.Contains(got[i], want[i]) {
			t.Errorf("problem %d = %q, want it to contain %q", i, got[i], want[i])
		}
	}
}
