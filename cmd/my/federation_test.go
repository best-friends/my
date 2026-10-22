// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"strings"
	"testing"
)

func TestAdminFederationCommands(t *testing.T) {
	newAdminEnv(t)
	for _, dir := range []string{"in", "out", "both"} {
		code, stdout, stderr := runCmd(t, adminCmd, "federation", "sever", "--direction", dir)
		if want := "severed 0 follows of local accounts by remote ones and 0 follows of remote accounts by local ones\n"; code != 0 || stdout != want {
			t.Errorf("federation sever --direction %s = %d %q %q", dir, code, stdout, stderr)
		}
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"federation", "sever"}, 2, "-direction must be in, out, or both"},
		{[]string{"federation", "sever", "-direction", "sideways"}, 2, "-direction must be in, out, or both"},
		{[]string{"federation", "sever", "extra"}, 2, "want 0 argument(s)"},
		{[]string{"federation", "burn"}, 2, `unknown subcommand "burn"`},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
