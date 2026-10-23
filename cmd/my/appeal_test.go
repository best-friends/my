// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/platform"
	"github.com/mycroblog/my/internal/platform/config"
)

func TestAdminAppealCommands(t *testing.T) {
	newAdminEnv(t)
	if code, _, stderr := runAdminCreate(t, adminPassword+"\n", "-username", "root", "-email", "root@example.com"); code != 0 {
		t.Fatalf("admin create: %d %q", code, stderr)
	}
	issue := func(kind string) (accountID, actionID string) {
		t.Helper()
		code, stdout, stderr := runCmd(t, adminCmd, "account", "action", "issue", "-kind", kind, "@root")
		m := regexp.MustCompile(`on account (\S+) \(action (\S+),`).FindStringSubmatch(stdout)
		if code != 0 || m == nil {
			t.Fatalf("account action issue %s = %d %q %q", kind, code, stdout, stderr)
		}
		return m[1], m[2]
	}
	accountID, restrict := issue("restrict_posting")
	_, warn := issue("warn")
	cfg, _, err := config.Load(config.Source{})
	if err != nil {
		t.Fatal(err)
	}
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = app.Close() }()
	first, err := app.Moderation.FileAppeal(t.Context(), accountID, restrict, "I did not\nspam.")
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.Moderation.FileAppeal(t.Context(), accountID, warn, "Why?")
	if err != nil {
		t.Fatal(err)
	}

	if code, stdout, _ := runCmd(t, adminCmd, "appeal", "list", "-state", "pending"); code != 0 ||
		!regexp.MustCompile(first.Appeal.ID+` +pending +\S+ +@root +`+restrict+` +restrict_posting +active +I did not spam\.`).MatchString(stdout) ||
		!strings.Contains(stdout, second.Appeal.ID) {
		t.Errorf("appeal list = %d\n%s", code, stdout)
	}
	if code, stdout, stderr := runCmd(t, adminCmd, "appeal", "overturn", "-reason", "Sorry.", first.Appeal.ID); code != 0 ||
		stdout != "appeal "+first.Appeal.ID+" is overturned; action "+restrict+" is lifted\n" {
		t.Errorf("appeal overturn = %d %q %q", code, stdout, stderr)
	}
	if rs, err := app.AccountPolicy.Restrictions(t.Context(), accountID); err != nil || len(rs) != 0 {
		t.Errorf("restrictions after overturning = %v, %v; want none", rs, err)
	}
	if code, stdout, _ := runCmd(t, adminCmd, "appeal", "uphold", second.Appeal.ID); code != 0 || !strings.Contains(stdout, "is upheld; action "+warn+" is active") {
		t.Errorf("appeal uphold = %d %q", code, stdout)
	}
	if code, stdout, _ := runCmd(t, adminCmd, "appeal", "list", "-state", "pending"); code != 0 || strings.Contains(stdout, first.Appeal.ID) || strings.Contains(stdout, second.Appeal.ID) {
		t.Errorf("appeal list -state pending after the decisions:\n%s", stdout)
	}
	if code, stdout, _ := runCmd(t, adminCmd, "audit", "list", "-target-kind", "appeal"); code != 0 ||
		!strings.Contains(stdout, "appeal_overturned") || !strings.Contains(stdout, "appeal_upheld") {
		t.Errorf("audit list of the appeals = %d\n%s", code, stdout)
	}

	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"appeal", "uphold", first.Appeal.ID}, 1, "already decided"},
		{[]string{"appeal", "uphold", "01890a5d-ac96-774b-bcce-b302099a0000"}, 1, "no appeal has the ID"},
		{[]string{"appeal", "list", "-state", "closed"}, 1, "invalid query"},
		{[]string{"appeal", "list", "-limit", "0"}, 2, "-limit must be at least 1"},
		{[]string{"appeal", "overturn"}, 2, "want 1 argument(s)"},
		{[]string{"appeal", "burn"}, 2, `unknown subcommand "burn"`},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
