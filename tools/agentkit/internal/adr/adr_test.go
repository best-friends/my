// SPDX-License-Identifier: AGPL-3.0-only

package adr

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# Architecture Decision Records\n")
	write(t, root, "template.md", "# ADR-NNNN: タイトル\n\n- Status: Proposed\n")
	write(t, root, "0002-b.md", "# ADR-0002: 二つ目\n\n- Status: Superseded by ADR-0003\n- Date: 2026-10-01\n")
	write(t, root, "0001-a.md", "# ADR-0001: 一つ目\n\n- Status: Accepted（§2 を ADR-0003 が置き換える）\n")
	write(t, root, "0003-c.md", "# ADR-0003: 三つ目\n\n- Status: Proposed <!-- Proposed | Accepted -->\n")

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []ADR{
		{Number: "0001", Title: "一つ目", Status: "Accepted", Path: "docs/adr/0001-a.md"},
		{Number: "0002", Title: "二つ目", Status: "Superseded", Path: "docs/adr/0002-b.md"},
		{Number: "0003", Title: "三つ目", Status: "Proposed", Path: "docs/adr/0003-c.md"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %+v, want %+v", got, want)
	}
	if f := Filter(got, "proposed"); len(f) != 1 || f[0].Number != "0003" {
		t.Errorf("Filter(proposed) = %+v", f)
	}
	if f := Filter(got, ""); len(f) != 3 {
		t.Errorf("Filter(\"\") = %+v", f)
	}
}

func TestListRejectsMalformed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "0001-a.md", "no heading\n")
	if _, err := List(root); err == nil {
		t.Fatal("List accepted an ADR without a title and status")
	}
}
