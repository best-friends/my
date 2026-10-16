// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfig(t *testing.T) {
	kek := base64.StdEncoding.EncodeToString(make([]byte, 32))
	file := filepath.Join(t.TempDir(), "my.toml")
	if err := os.WriteFile(file, []byte(`role = "web"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{
			name:       "valid",
			args:       []string{"config", "check", "-config", file},
			env:        map[string]string{"MY_HOST": "social.example.org", "MY_DATABASE_URL": "file:my.db", "MY_KEK": kek},
			wantCode:   0,
			wantStdout: []string{"web", "file", "(redacted)", "configuration OK"},
		},
		{
			name:     "invalid lists every issue",
			args:     []string{"config", "check"},
			env:      map[string]string{"MY_DB": "mysql", "LC_ALL": "C"},
			wantCode: 1,
			wantStderr: []string{
				"The required setting MY_HOST is not set.",
				"The setting MY_DB has an invalid value.",
				`db: unknown value "mysql"`,
				"The required setting MY_DATABASE_URL is not set.",
			},
		},
		{name: "no subcommand", args: []string{"config"}, wantCode: 2, wantStderr: []string{"Usage: my config check"}},
		{name: "unknown subcommand", args: []string{"config", "nope"}, wantCode: 2, wantStderr: []string{`unknown subcommand "nope"`}},
		{name: "extra argument", args: []string{"config", "check", "x"}, wantCode: 2, wantStderr: []string{`unexpected argument "x"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LANGUAGE", "")
			for _, k := range []string{"MY_CONFIG", "MY_HOST", "MY_DATABASE_URL", "MY_KEK", "MY_DB", "LC_ALL"} {
				t.Setenv(k, tt.env[k]) // restores the original value after the test
				if _, ok := tt.env[k]; !ok {
					if err := os.Unsetenv(k); err != nil {
						t.Fatal(err)
					}
				}
			}
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr = %s", code, tt.wantCode, stderr.String())
			}
			for _, w := range tt.wantStdout {
				if !strings.Contains(stdout.String(), w) {
					t.Errorf("stdout = %q, want it to contain %q", stdout.String(), w)
				}
			}
			for _, w := range tt.wantStderr {
				if !strings.Contains(stderr.String(), w) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), w)
				}
			}
		})
	}
}
