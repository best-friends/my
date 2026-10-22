// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/best-friends/my/pkg/testkit"
)

var createdEmoji = regexp.MustCompile(`^created :blobcat: \(emoji (\S+)\)\n$`)

func TestAdminEmojiCommands(t *testing.T) {
	newAdminEnv(t)
	dir := t.TempDir()
	png, jpeg := filepath.Join(dir, "blobcat.png"), filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(png, testkit.FixturePNG(32, 32, true), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jpeg, testkit.FixtureJPEG(32, 32, 1), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCmd(t, adminCmd, "emoji", "create", "-shortcode", "blobcat", "-category", "Cats", "-alias", "neko", "-alias", "kitty", png)
	m := createdEmoji.FindStringSubmatch(stdout)
	if code != 0 || m == nil {
		t.Fatalf("emoji create = %d %q %q", code, stdout, stderr)
	}
	id := m[1]
	if code, stdout, _ = runCmd(t, adminCmd, "emoji", "list", "-local"); code != 0 ||
		!regexp.MustCompile(id+` +blobcat +- +enabled +shown +Cats +kitty,neko +\S+`).MatchString(stdout) {
		t.Errorf("emoji list = %d\n%s", code, stdout)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "emoji", "disable", id); code != 0 || stdout != "disabled :blobcat: (emoji "+id+")\n" {
		t.Errorf("emoji disable = %d %q", code, stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "emoji", "list", "-shortcode", "BLOB"); !strings.Contains(stdout, "disabled") {
		t.Errorf("emoji list after disable:\n%s", stdout)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "emoji", "enable", id); code != 0 || stdout != "enabled :blobcat: (emoji "+id+")\n" {
		t.Errorf("emoji enable = %d %q", code, stdout)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "emoji", "update", "-category", "Animals", "-alias", "cat", "-hidden", id); code != 0 ||
		stdout != "updated :blobcat: (emoji "+id+")\n" {
		t.Errorf("emoji update = %d %q", code, stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "emoji", "list", "-local"); !regexp.MustCompile(id + ` +blobcat +- +enabled +hidden +Animals +cat +\S+`).MatchString(stdout) {
		t.Errorf("emoji list after update:\n%s", stdout)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "emoji", "update", "-category", "", "-no-aliases", "-hidden=false", id); code != 0 {
		t.Errorf("emoji update = %d %q", code, stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "emoji", "list", "-local"); !regexp.MustCompile(id + ` +blobcat +- +enabled +shown +- +- +\S+`).MatchString(stdout) {
		t.Errorf("emoji list after the second update:\n%s", stdout)
	}
	if code, stdout, stderr = runCmd(t, adminCmd, "emoji", "replace-image", id, png); code != 0 ||
		stdout != "replaced the image of :blobcat: (emoji "+id+")\n" {
		t.Errorf("emoji replace-image = %d %q %q", code, stdout, stderr)
	}
	if code, stdout, _ = runCmd(t, adminCmd, "audit", "list", "-target-kind", "custom_emoji"); code != 0 ||
		strings.Count(stdout, ":blobcat:") != 6 || !strings.Contains(stdout, "custom_emoji_created") || !strings.Contains(stdout, "custom_emoji_disabled") ||
		!strings.Contains(stdout, "custom_emoji_updated") || !strings.Contains(stdout, "custom_emoji_image_replaced") {
		t.Errorf("audit list:\n%s", stdout)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"emoji", "copy", id}, 1, "only the custom emoji of other servers can be copied"},
		{[]string{"emoji", "replace-image", id, jpeg}, 1, "must be a PNG, a GIF, or a WebP"},
		{[]string{"emoji", "replace-image", id}, 2, "want 2 argument(s)"},
		{[]string{"emoji", "update", id}, 2, "give at least one of"},
		{[]string{"emoji", "update", "-alias", "cat", "-no-aliases", id}, 2, "cannot be given together"},
		{[]string{"emoji", "update", "-alias", "x", id}, 1, "an alias is 2 to 30 letters"},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
	if code, stdout, _ = runCmd(t, adminCmd, "emoji", "delete", id); code != 0 || stdout != "deleted :blobcat: (emoji "+id+")\n" {
		t.Errorf("emoji delete = %d %q", code, stdout)
	}
	if _, stdout, _ = runCmd(t, adminCmd, "emoji", "list"); strings.Contains(stdout, id) {
		t.Errorf("emoji list after delete:\n%s", stdout)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"emoji", "create", "-shortcode", "photo", jpeg}, 1, "must be a PNG, a GIF, or a WebP"},
		{[]string{"emoji", "create", "-shortcode", "a", png}, 1, "2 to 30 letters"},
		{[]string{"emoji", "create", png}, 2, "-shortcode is required"},
		{[]string{"emoji", "create", "-shortcode", "missing", filepath.Join(dir, "missing.png")}, 1, "no such file"},
		{[]string{"emoji", "disable", "01890a5d-ac96-774b-bcce-b302099a0000"}, 1, "no custom emoji has the ID"},
		{[]string{"emoji", "delete"}, 2, "want 1 argument(s)"},
		{[]string{"emoji", "list", "-local", "-remote"}, 2, "cannot be given together"},
		{[]string{"emoji", "burn"}, 2, `unknown subcommand "burn"`},
	} {
		if code, _, stderr := runCmd(t, adminCmd, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("admin %q = %d %q; want %d with %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
}

func TestAdminEmojiRequestCommands(t *testing.T) {
	newAdminEnv(t)
	if code, stdout, stderr := runCmd(t, adminCmd, "emoji", "requests", "list"); code != 0 || !strings.HasPrefix(stdout, "ID ") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("emoji requests list = %d %q %q", code, stdout, stderr)
	}
	if code, _, stderr := runCmd(t, adminCmd, "emoji", "requests", "list", "-state", "nothing"); code != 2 || !strings.Contains(stderr, `unknown -state "nothing"`) {
		t.Errorf("emoji requests list -state nothing = %d %q", code, stderr)
	}
	const unknown = "0192f0c1-0000-7000-8000-00000000dead"
	for _, sub := range []string{"approve", "reject"} {
		if code, _, stderr := runCmd(t, adminCmd, "emoji", "requests", sub, unknown); code != 1 || !strings.Contains(stderr, "no custom emoji request has the ID") {
			t.Errorf("emoji requests %s unknown = %d %q", sub, code, stderr)
		}
	}
	if code, _, _ := runCmd(t, adminCmd, "emoji", "requests"); code != 2 {
		t.Errorf("emoji requests without a subcommand = %d, want 2", code)
	}
}
