// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestAdminAuditCommands(t *testing.T) {
	newAdminEnv(t)
	code, stdout, stderr := runCmd(t, adminCmd, "domain", "issue", "-kind", "suspend", "bad.example")
	m := regexp.MustCompile(`\(action (\S+),`).FindStringSubmatch(stdout)
	if code != 0 || m == nil {
		t.Fatalf("domain issue = %d %q %q", code, stdout, stderr)
	}
	if code, _, stderr = runCmd(t, adminCmd, "domain", "lift", m[1]); code != 0 {
		t.Fatalf("domain lift = %d %q", code, stderr)
	}
	// The CLI's operations are recorded with actor cli, newest first.
	code, stdout, _ = runCmd(t, adminCmd, "audit", "list")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if code != 0 || len(lines) != 3 || !strings.HasPrefix(lines[0], "ID ") ||
		!regexp.MustCompile(` cli +domain_action_lifted +domain bad\.example +bad\.example +`+m[1]+`$`).MatchString(lines[1]) ||
		!regexp.MustCompile(` cli +domain_action_issued +domain bad\.example +bad\.example +`+m[1]+`$`).MatchString(lines[2]) {
		t.Errorf("audit list = %d\n%s", code, stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "audit", "list", "-action", "domain_action_issued"); strings.Count(stdout, "\n") != 2 ||
		!strings.Contains(stdout, "domain_action_issued") {
		t.Errorf("audit list -action:\n%s", stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "audit", "list", "-target-kind", "domain", "-target-id", "good.example"); strings.Count(stdout, "\n") != 1 {
		t.Errorf("audit list -target-id good.example:\n%s", stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "audit", "list", "-limit", "1"); strings.Count(stdout, "\n") != 2 || !strings.Contains(stdout, "lifted") {
		t.Errorf("audit list -limit 1:\n%s", stdout)
	}

	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"audit", "list", "-action", "domain_burned"}, 1, `invalid query: action "domain_burned"`},
		{[]string{"audit", "list", "-target-id", "x"}, 1, "a target ID needs its target kind"},
		{[]string{"audit", "list", "-actor", "nobody"}, 1, `no local account has the username "nobody"`},
		{[]string{"audit", "list", "-limit", "0"}, 2, "-limit must be at least 1"},
		// The audit log cannot be changed or deleted.
		{[]string{"audit", "delete"}, 2, `unknown subcommand "delete"`},
		{[]string{"audit", "clear"}, 2, `unknown subcommand "clear"`},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
