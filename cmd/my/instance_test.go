// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"strings"
	"testing"
)

func runInstanceCmd(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = runInstance(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func mustInstance(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	code, stdout, stderr := runInstanceCmd(t, stdin, args...)
	if code != 0 {
		t.Fatalf("my instance %q: exit code %d, stderr %q", args, code, stderr)
	}
	return stdout
}

func TestInstanceSetGetList(t *testing.T) {
	newAdminEnv(t)

	if got := mustInstance(t, "", "get", "registration_mode"); got != "approval\n" {
		t.Errorf("get registration_mode before set = %q, want the default", got)
	}
	if got := mustInstance(t, "", "set", "registration_mode", "open"); !strings.HasPrefix(got, "set registration_mode = open (revision ") {
		t.Errorf("set stdout = %q", got)
	}
	mustInstance(t, "", "set", "registration_mode", "closed")
	if got := mustInstance(t, "", "get", "registration_mode"); got != "closed\n" {
		t.Errorf("get registration_mode = %q, want closed", got)
	}
	history := mustInstance(t, "", "history", "registration_mode")
	if lines := strings.Split(strings.TrimSpace(history), "\n"); len(lines) != 3 ||
		!strings.HasSuffix(lines[1], "cli  closed") || !strings.HasSuffix(lines[2], "cli  open") {
		t.Errorf("history = %q, want two CLI revisions, newest first", history)
	}

	mustInstance(t, "", "set", "name", "Example")
	mustInstance(t, "", "set", "-locale", "ja", "name", "例")
	mustInstance(t, "説明文\n2 行目\n", "set", "-locale", "ja", "description", "-")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"get", "name"}, "Example\n"},
		{[]string{"get", "-locale", "ja-JP", "name"}, "例\n"},
		{[]string{"get", "-locale", "fr", "name"}, "Example\n"},
		{[]string{"get", "-locale", "ja", "description"}, "説明文\n2 行目\n"},
	} {
		if got := mustInstance(t, "", tc.args...); got != tc.want {
			t.Errorf("%q = %q, want %q", tc.args, got, tc.want)
		}
	}

	list := mustInstance(t, "", "list")
	for _, want := range []string{"registration_mode           closed  set", "max_status_length           500     default", `name         ja`, `"例"`} {
		if !strings.Contains(list, want) {
			t.Errorf("list = %q, missing %q", list, want)
		}
	}
}

func TestInstanceRejectsBadInput(t *testing.T) {
	newAdminEnv(t)
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{nil, 2, "Usage:"},
		{[]string{"frobnicate"}, 2, "unknown subcommand"},
		{[]string{"get"}, 2, "want 1 argument"},
		{[]string{"get", "no_such_key"}, 2, `unknown key "no_such_key"`},
		{[]string{"set", "-locale", "ja", "registration_mode", "open"}, 2, "-locale applies only to text keys"},
		{[]string{"set", "max_status_length", "0"}, 1, "outside 1..100000"},
		{[]string{"set", "registration_mode", "everyone"}, 1, "not one of open, approval, invite, closed"},
		{[]string{"get", "terms"}, 1, "not set in any locale"},
	} {
		code, _, stderr := runInstanceCmd(t, "", tc.args...)
		if code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("my instance %q = %d, %q; want %d and %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}
