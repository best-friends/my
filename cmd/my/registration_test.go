// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	appidentity "github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/platform"
	"github.com/mycroblog/my/internal/platform/config"
)

func runCmd(t *testing.T, run func([]string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func invite(args []string, out, errOut *bytes.Buffer) int { return runInvite(args, out, errOut) }

func registration(args []string, out, errOut *bytes.Buffer) int {
	return runRegistration(args, out, errOut)
}

var createdInvite = regexp.MustCompile(`^created invite ([A-Z2-7]{16}) \(2 uses, expires [^)]+\)\nhttps://social\.example/auth/sign_up\?invite=([A-Z2-7]{16})\n$`)

func TestInviteCommands(t *testing.T) {
	newAdminEnv(t)
	code, stdout, stderr := runCmd(t, invite, "create", "-max-uses", "2", "-expires-in", "48h")
	if code != 0 {
		t.Fatalf("invite create: %d %q", code, stderr)
	}
	m := createdInvite.FindStringSubmatch(stdout)
	if m == nil || m[1] != m[2] {
		t.Fatalf("invite create stdout = %q", stdout)
	}
	inviteCode := m[1]
	if code, stdout, _ = runCmd(t, invite, "create", "-max-uses", "0", "-expires-in", "0"); code != 0 ||
		!strings.Contains(stdout, "(unlimited uses, never expires)") {
		t.Errorf("unlimited invite create = %d %q", code, stdout)
	}
	if code, stdout, _ = runCmd(t, invite, "list"); code != 0 || !regexp.MustCompile(inviteCode+` +usable +0 +2 +\S+ +\S+ +cli`).MatchString(stdout) {
		t.Errorf("invite list = %d\n%s", code, stdout)
	}
	if code, stdout, _ = runCmd(t, invite, "revoke", strings.ToLower(inviteCode)); code != 0 || stdout != "revoked invite "+inviteCode+"\n" {
		t.Errorf("invite revoke = %d %q", code, stdout)
	}
	if _, stdout, _ = runCmd(t, invite, "list"); !regexp.MustCompile(inviteCode + ` +revoked`).MatchString(stdout) {
		t.Errorf("invite list after revoke:\n%s", stdout)
	}
	if code, _, stderr = runCmd(t, invite, "revoke", "NOSUCHCODE"); code != 1 || !strings.Contains(stderr, `no invite has the code "NOSUCHCODE"`) {
		t.Errorf("revoking an unknown code = %d %q", code, stderr)
	}
	for _, args := range [][]string{{}, {"burn"}, {"revoke"}, {"create", "extra"}} {
		if code, _, _ := runCmd(t, invite, args...); code != 2 {
			t.Errorf("invite %q = %d, want 2", args, code)
		}
	}
}

func TestRegistrationCommands(t *testing.T) {
	newAdminEnv(t)
	cfg, _, err := config.Load(config.Source{})
	if err != nil {
		t.Fatal(err)
	}
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// The default registration mode is approval.
	for _, name := range []string{"alice", "bob"} {
		_, err := app.Identity.SignUp(t.Context(), appidentity.SignUpInput{
			Username: name, Email: name + "@example.com", Password: "correct horse battery", Reason: "Hi, I am " + name,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = app.Close()

	code, stdout, stderr := runCmd(t, registration, "list")
	if code != 0 {
		t.Fatalf("registration list = %d %q", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[1], "alice ") || !strings.Contains(lines[1], `alice@example.com  no `) ||
		!strings.HasSuffix(lines[1], `"Hi, I am alice"`) || !strings.HasPrefix(lines[2], "bob ") {
		t.Fatalf("registration list:\n%s", stdout)
	}
	if code, stdout, stderr = runCmd(t, registration, "approve", "@Alice"); code != 0 || !strings.HasPrefix(stdout, "approved the registration of @Alice") {
		t.Errorf("approve = %d %q %q", code, stdout, stderr)
	}
	if code, stdout, _ = runCmd(t, registration, "reject", "-note", "spam", "bob"); code != 0 || !strings.HasPrefix(stdout, "rejected the registration of @bob") {
		t.Errorf("reject = %d %q", code, stdout)
	}
	if code, _, stderr = runCmd(t, registration, "reject", "alice"); code != 1 || !strings.Contains(stderr, "already decided") {
		t.Errorf("deciding again = %d %q", code, stderr)
	}
	if code, _, stderr = runCmd(t, registration, "approve", "nobody"); code != 1 || !strings.Contains(stderr, `no local account has the username "nobody"`) {
		t.Errorf("approving an unknown user = %d %q", code, stderr)
	}
	if _, stdout, _ = runCmd(t, registration, "list"); strings.Count(stdout, "\n") != 1 {
		t.Errorf("pending after the decisions:\n%s", stdout)
	}
	if _, stdout, _ = runCmd(t, registration, "list", "-state", "rejected"); !strings.Contains(stdout, "bob ") {
		t.Errorf("rejected list:\n%s", stdout)
	}
	if code, _, _ := runCmd(t, registration, "list", "-state", "maybe"); code != 2 {
		t.Errorf("list -state maybe = %d, want 2", code)
	}

	// A user created with the CLI has no registration to decide.
	if code, _, stderr := runAdminCreate(t, adminPassword+"\n", "-username", "root", "-email", "root@example.com"); code != 0 {
		t.Fatal(stderr)
	}
	if code, _, stderr = runCmd(t, registration, "approve", "root"); code != 1 || !strings.Contains(stderr, "nothing to decide") {
		t.Errorf("approving an admin = %d %q", code, stderr)
	}
}
