// SPDX-License-Identifier: AGPL-3.0-only

package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, event string, in map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run(event, bytes.NewReader(b), &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("hook printed invalid JSON %q: %v", out.String(), err)
	}
	return v["hookSpecificOutput"].(map[string]any)
}

func TestPre(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.md")
	if err := os.WriteFile(big, []byte("# Big\n\n"+strings.Repeat("本文。\n", 20000)), 0o600); err != nil {
		t.Fatal(err)
	}
	small := filepath.Join(dir, "small.md")
	if err := os.WriteFile(small, []byte("# Small\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		in   map[string]any
		want string // permissionDecision, "hint", or "" for no output
	}{
		{"read big doc", map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": big}, "cwd": dir}, "deny"},
		{"read big doc in part", map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": big, "offset": 10, "limit": 50}, "cwd": dir}, ""},
		{"read small doc", map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": small}, "cwd": dir}, ""},
		{"cat big doc", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "cat big.md"}, "cwd": dir}, "deny"},
		{"cat big doc piped", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "cat big.md | grep x"}, "cwd": dir}, ""},
		{"watch checks", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "gh pr checks 12 --watch"}, "cwd": dir}, "deny"},
		{"run watch", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "git push && gh run watch 123"}, "cwd": dir}, "deny"},
		{"watch in a string", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "cat > doc.md <<'EOF'\nnever `gh pr checks --watch`\nEOF"}, "cwd": dir}, ""},
		{"grep all", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "grep -rn Foo ."}, "cwd": dir}, "hint"},
		{"grep files only", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "grep -rl Foo ."}, "cwd": dir}, ""},
		{"grep head", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "rg Foo | head -20"}, "cwd": dir}, ""},
		{"other", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "go test ./..."}, "cwd": dir}, ""},
	}
	for _, c := range cases {
		out := run(t, "pre", c.in)
		got := ""
		if out != nil {
			if d, ok := out["permissionDecision"].(string); ok {
				got = d
			} else if _, ok := out["additionalContext"]; ok {
				got = "hint"
			}
			if _, ok := out["permissionDecision"]; ok && out["permissionDecision"] == "allow" {
				t.Errorf("%s: the hook must never allow", c.name)
			}
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q (%v)", c.name, got, c.want, out)
		}
	}
}

func TestPost(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("a line of output\n", 5000)
	out := run(t, "post", map[string]any{
		"tool_name": "Bash", "cwd": dir,
		"tool_input":    map[string]any{"command": "go test -v ./..."},
		"tool_response": map[string]any{"stdout": long, "stderr": "", "interrupted": false, "isImage": false},
	})
	if out == nil {
		t.Fatal("a long output was not cut")
	}
	resp := out["updatedToolOutput"].(map[string]any)
	stdout := resp["stdout"].(string)
	if len(stdout) > 20000 || !strings.Contains(stdout, "lines (") || resp["interrupted"] != false {
		t.Errorf("cut output is wrong: %d characters, interrupted=%v", len(stdout), resp["interrupted"])
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".tmp", "agentkit", "out", "*.txt"))
	if len(files) != 1 {
		t.Fatalf("the whole output was not saved: %v", files)
	}
	if b, _ := os.ReadFile(files[0]); string(b) != long {
		t.Error("the saved output differs from the original")
	}
	short := run(t, "post", map[string]any{
		"tool_name": "Bash", "cwd": dir, "tool_input": map[string]any{"command": "ls"},
		"tool_response": map[string]any{"stdout": "a\nb\n", "stderr": ""},
	})
	if short != nil {
		t.Error("a short output was changed")
	}
	own := run(t, "post", map[string]any{
		"tool_name": "Bash", "cwd": dir, "tool_input": map[string]any{"command": "scripts/agentkit.sh ctx 916"},
		"tool_response": map[string]any{"stdout": long, "stderr": ""},
	})
	if own != nil {
		t.Error("agentkit's own output was cut")
	}
}
