// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"regexp"
	"strings"
	"testing"
)

var addedBlock = regexp.MustCompile(`^added reject block on email_domain xn--r8jz45g\.jp \(block (\S+), never expires\)\n$`)

func TestAdminSignupBlockCommands(t *testing.T) {
	newAdminEnv(t)
	code, stdout, stderr := runCmd(t, adminCmd, "signup-block", "add", "-kind", "email_domain", "-comment", "spam\nwave", "例え.JP")
	m := addedBlock.FindStringSubmatch(stdout)
	if code != 0 || m == nil {
		t.Fatalf("signup-block add = %d %q %q", code, stdout, stderr)
	}
	id := m[1]
	if code, stdout, _ = runCmd(t, adminCmd, "signup-block", "add", "-kind", "ip_range", "-severity", "require_approval", "-duration", "24h", "192.0.2.9/24"); code != 0 ||
		!strings.Contains(stdout, "added require_approval block on ip_range 192.0.2.0/24") || !strings.Contains(stdout, "expires 20") {
		t.Errorf("signup-block add ip_range = %d %q", code, stdout)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "signup-block", "list"); code != 0 ||
		!regexp.MustCompile(id+` +email_domain +xn--r8jz45g\.jp +reject +active +\S+ +- +cli +- +spam wave`).MatchString(stdout) ||
		!strings.Contains(stdout, "192.0.2.0/24") {
		t.Errorf("signup-block list = %d\n%s", code, stdout)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "signup-block", "lift", id); code != 0 || stdout != "lifted sign-up block "+id+"\n" {
		t.Errorf("signup-block lift = %d %q", code, stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "signup-block", "list", "-active"); strings.Contains(stdout, id) || !strings.Contains(stdout, "192.0.2.0/24") {
		t.Errorf("signup-block list -active:\n%s", stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "signup-block", "list", "-kind", "email_domain"); !regexp.MustCompile(id+` +email_domain +\S+ +reject +lifted`).MatchString(stdout) || strings.Contains(stdout, "192.0.2.0/24") {
		t.Errorf("signup-block list -kind:\n%s", stdout)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"signup-block", "add", "-kind", "ip_range", "spam.example"}, 1, "invalid sign-up block"},
		{[]string{"signup-block", "add", "-kind", "ip_range", "-severity", "maybe", "192.0.2.0/24"}, 1, `severity "maybe"`},
		{[]string{"signup-block", "add", "spam.example"}, 2, "-kind is required"},
		{[]string{"signup-block", "lift", "01890a5d-ac96-774b-bcce-b302099a0000"}, 1, "no sign-up block has the ID"},
		{[]string{"signup-block", "lift"}, 2, "want 1 argument(s)"},
		{[]string{"signup-block", "list", "-kind", "email"}, 1, "invalid query"},
		{[]string{"signup-block", "burn"}, 2, `unknown subcommand "burn"`},
		{[]string{"signup-block", "list", "-limit", "0"}, 2, "-limit must be at least 1"},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
