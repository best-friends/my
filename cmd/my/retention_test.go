// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	appmoderation "github.com/best-friends/my/internal/app/moderation"
)

func TestAdminRetentionAndExportCommands(t *testing.T) {
	newAdminEnv(t)
	if code, _, stderr := runAdminCreate(t, adminPassword+"\n", "-username", "root", "-email", "root@example.com"); code != 0 {
		t.Fatalf("admin create: %d %q", code, stderr)
	}
	code, stdout, stderr := runCmd(t, adminCmd, "account", "action", "issue", "-kind", "warn", "-private-note", "watch this one", "@root")
	if code != 0 {
		t.Fatalf("account action issue = %d %q %q", code, stdout, stderr)
	}

	code, stdout, stderr = runCmd(t, adminCmd, "retention", "hold", "-reason", "court order", "@root")
	m := regexp.MustCompile(`^placed retention hold (\S+) on account (\S+)\n$`).FindStringSubmatch(stdout)
	if code != 0 || m == nil {
		t.Fatalf("retention hold = %d %q %q", code, stdout, stderr)
	}
	holdID, accountID := m[1], m[2]
	if code, stdout, _ := runCmd(t, adminCmd, "retention", "list", "-account", "@root"); code != 0 ||
		!regexp.MustCompile(holdID+` +`+accountID+` +active +\S+ +- +cli +court order`).MatchString(stdout) {
		t.Errorf("retention list = %d\n%s", code, stdout)
	}
	if code, stdout, _ := runCmd(t, adminCmd, "retention", "release", holdID); code != 0 || stdout != "released retention hold "+holdID+"\n" {
		t.Errorf("retention release = %d %q", code, stdout)
	}
	if code, stdout, _ := runCmd(t, adminCmd, "retention", "release", holdID); code != 0 || !strings.Contains(stdout, "already released") {
		t.Errorf("retention release again = %d %q", code, stdout)
	}
	if code, stdout, _ := runCmd(t, adminCmd, "retention", "list"); code != 0 || !strings.Contains(stdout, " released ") {
		t.Errorf("retention list after the release = %d\n%s", code, stdout)
	}

	code, stdout, stderr = runCmd(t, adminCmd, "account", "export-moderation", "@root")
	if code != 0 {
		t.Fatalf("account export-moderation = %d %q", code, stderr)
	}
	var x appmoderation.RecordExport
	if err := json.Unmarshal([]byte(stdout), &x); err != nil {
		t.Fatalf("export is not JSON: %v\n%s", err, stdout)
	}
	if x.AccountID != accountID || len(x.Actions) != 1 || x.Actions[0].PrivateNote != "watch this one" || len(x.AuditLog) != 1 {
		t.Errorf("export = %+v; want the action with its private note and its audit log entry", x)
	}

	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"retention", "hold", "@root"}, 2, "-reason is required"},
		{[]string{"retention", "hold", "-reason", "x", "@nobody"}, 1, `no account is "@nobody"`},
		{[]string{"retention", "release", "01890a5d-ac96-774b-bcce-b302099a0000"}, 1, "no retention hold has the ID"},
		{[]string{"retention", "release"}, 2, "want 1 argument(s)"},
		{[]string{"retention", "erase"}, 2, "unknown subcommand"},
		{[]string{"account", "export-moderation", "@nobody"}, 1, `no account is "@nobody"`},
		{[]string{"account", "export-moderation"}, 2, "want 1 argument(s)"},
	} {
		code, _, stderr := runCmd(t, adminCmd, tc.args...)
		if code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %v = %d %q; want %d containing %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
