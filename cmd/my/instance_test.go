// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/adapter/db/sqlite"
	"github.com/best-friends/my/internal/app/uow"
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

// TestNewUsersGetTheInstanceDefaultLocale checks the composition root: identity reads the
// default_locale instance setting from the database for each registration.
func TestNewUsersGetTheInstanceDefaultLocale(t *testing.T) {
	path := newAdminEnv(t)
	create := func(name string) string {
		t.Helper()
		code, _, stderr := runAdminCreate(t, adminPassword+"\n", "-username", name, "-email", name+"@example.com")
		if code != 0 {
			t.Fatalf("admin create %s: exit code %d, stderr %q", name, code, stderr)
		}
		db, err := sqlite.Open(t.Context(), path, uow.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = db.Close() }()
		repo := db.IdentityRepository()
		userID, err := repo.FindUserIDByEmail(t.Context(), name+"@example.com")
		if err != nil {
			t.Fatal(err)
		}
		prefs, err := repo.FindPreferences(t.Context(), userID)
		if err != nil {
			t.Fatal(err)
		}
		return prefs.Locale
	}
	if got := create("before"); got != "en" {
		t.Errorf("locale with default_locale unset = %q, want en", got)
	}
	mustInstance(t, "", "set", "default_locale", "ja_jp")
	if got := create("after"); got != "ja-JP" {
		t.Errorf("locale after setting default_locale = %q, want ja-JP", got)
	}
}
