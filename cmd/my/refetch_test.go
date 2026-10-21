// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"strings"
	"testing"
)

func TestAdminRefetchCommands(t *testing.T) {
	newAdminEnv(t)
	if code, _, stderr := runAdminCreate(t, adminPassword+"\n", "-username", "root", "-email", "root@example.com"); code != 0 {
		t.Fatalf("admin create = %d %q", code, stderr)
	}
	code, stdout, stderr := runCmd(t, adminCmd, "refetch", "domain", "Remote.Example")
	if code != 0 || stdout != "queued the refetch of the statuses of the accounts of remote.example (20 per account, 1000 in all)\n" {
		t.Fatalf("refetch domain = %d %q %q", code, stdout, stderr)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"refetch", "domain", "social.example"}, 1, "is not remote"},
		{[]string{"refetch", "domain", "bad host"}, 1, "is not a domain"},
		{[]string{"refetch", "account", "@root"}, 1, "is not remote"},
		{[]string{"refetch", "account", "@nobody@remote.example"}, 1, "no account is"},
		{[]string{"refetch", "status", "01890a5d-ac96-774b-bcce-b302099a0000"}, 1, "no remote status or account"},
		{[]string{"refetch", "status", "https://remote.example/notes/1"}, 1, "no remote status or account"},
		{[]string{"refetch", "status"}, 2, "want 1 argument(s)"},
		{[]string{"refetch", "everything"}, 2, `unknown subcommand "everything"`},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
	if _, stdout, _ = runCmd(t, adminCmd, "audit", "list", "-target-kind", "domain"); !strings.Contains(stdout, "domain_statuses_refetch_queued") ||
		!strings.Contains(stdout, "remote.example") {
		t.Errorf("audit list of the domains:\n%s", stdout)
	}
}
