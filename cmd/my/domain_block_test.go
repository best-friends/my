// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminDomainImportAndExport(t *testing.T) {
	newAdminEnv(t)
	file := filepath.Join(t.TempDir(), "domain_blocks.csv")
	csv := "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate\n" +
		"spam.example,suspend,false,true,\"Spam, mostly\",true\n" +
		"noisy.example,silence,false,false,,false\n" +
		"odd.example,ban,false,false,,false\n"
	if err := os.WriteFile(file, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	// Without -apply, the import only previews.
	code, stdout, stderr := runCmd(t, adminCmd, "domain", "import", file)
	want := "line 2: spam.example: issue suspend, reject_reports\nline 3: noisy.example: issue limit\nline 4: odd.example: invalid\n" +
		"2 row(s) to import; run again with -apply to issue their actions\n"
	if code != 0 || stdout != want {
		t.Fatalf("domain import = %d %q %q; want %q", code, stdout, stderr, want)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "domain", "list"); strings.Contains(stdout, "spam.example") {
		t.Errorf("the preview issued an action:\n%s", stdout)
	}
	code, stdout, stderr = runCmd(t, adminCmd, "domain", "import", "-apply", "-only", "Spam.example", "-private-note", "from mastodon.example", file)
	want = "line 2: spam.example: issue suspend, reject_reports\nline 3: noisy.example: issue limit (not chosen)\nline 4: odd.example: invalid\n" +
		"issued 2 action(s), skipped 0 row(s)\n"
	if code != 0 || stdout != want {
		t.Fatalf("domain import -apply -only = %d %q %q; want %q", code, stdout, stderr, want)
	}
	code, stdout, stderr = runCmd(t, adminCmd, "domain", "export")
	want = "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate\n" +
		"spam.example,suspend,false,true,\"Spam, mostly\",true\n"
	if code != 0 || stdout != want {
		t.Errorf("domain export = %d %q %q; want %q", code, stdout, stderr, want)
	}
	// stdin supplies the CSV given as "-"; what is in effect is skipped.
	var out, errOut bytes.Buffer
	code = runAdmin([]string{"domain", "import", "-apply", "-"}, strings.NewReader(stdout), &out, &errOut)
	if want := "line 2: spam.example: already_applied\nissued 0 action(s), skipped 0 row(s)\n"; code != 0 || out.String() != want {
		t.Errorf("domain import - = %d %q %q; want %q", code, out.String(), errOut.String(), want)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"domain", "import"}, 2, "want 1 argument(s)"},
		{[]string{"domain", "export", file}, 2, "want 0 argument(s)"},
		{[]string{"domain", "import", filepath.Join(t.TempDir(), "missing.csv")}, 1, "no such file"},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
