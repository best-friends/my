// SPDX-License-Identifier: AGPL-3.0-only

package ops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/best-friends/my/tools/agentkit/internal/repo"
)

// treeKey identifies the content of the working tree (every path and its blob id), so that committing the
// checked changes keeps the key.
func treeKey(r *repo.Repo) string {
	files, err := r.Files()
	if err != nil {
		return ""
	}
	lines := make([]string, len(files))
	for i, f := range files {
		lines[i] = f.Path + " " + f.SHA
	}
	sort.Strings(lines)
	h := sha256.New()
	h.Write([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(h.Sum(nil))
}

func passedFile(r *repo.Repo) string {
	return filepath.Join(r.Root, ".tmp", "agentkit", "check", "passed")
}

// CheckPassed reports whether `agentkit check` passed on the current content of the working tree.
func CheckPassed(r *repo.Repo) bool {
	b, err := os.ReadFile(passedFile(r))
	return err == nil && strings.TrimSpace(string(b)) == treeKey(r)
}

// Check runs `make check-changed BASE=base` with its output in a log file, and returns the lines to print:
// the contexts and their results, and on failure the failing lines. It records a pass for `pr open`.
func Check(r *repo.Repo, base string) (bool, []string, error) {
	dir, err := r.TmpDir("check")
	if err != nil {
		return false, nil, err
	}
	log := filepath.Join(dir, time.Now().Format("20060102-150405")+".log")
	key := treeKey(r)
	start := time.Now()
	cmd := exec.Command("make", "--no-print-directory", "check-changed", "BASE="+base)
	cmd.Dir = r.Root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if err := os.WriteFile(log, out.Bytes(), 0o600); err != nil {
		return false, nil, err
	}
	var lines []string
	inSummary := false
	for _, l := range strings.Split(out.String(), "\n") {
		switch {
		case strings.HasPrefix(l, "contexts changed since"), strings.HasPrefix(l, "also run"), strings.HasPrefix(l, "no check covers"):
			lines = append(lines, l)
		case l == "make check:":
			inSummary = true
			lines = append(lines, l)
		case inSummary && strings.HasPrefix(l, "  "):
			lines = append(lines, l)
		default:
			inSummary = false
		}
	}
	rel, _ := filepath.Rel(r.Root, log)
	ok := runErr == nil
	if ok {
		_ = os.WriteFile(passedFile(r), []byte(key+"\n"), 0o600)
		lines = append(lines, fmt.Sprintf("passed in %s (log: %s)", time.Since(start).Round(time.Second), rel))
		return true, lines, nil
	}
	_ = os.Remove(passedFile(r))
	lines = append(lines, "", "failing lines:")
	lines = append(lines, Failures(out.String(), 80)...)
	lines = append(lines, fmt.Sprintf("FAILED after %s (full log: %s)", time.Since(start).Round(time.Second), rel))
	return false, lines, nil
}

// generators maps the sources of generated code to the make target that regenerates it (workflow.md §7).
var generators = []struct {
	prefix, target string
}{
	{"api/spec/", "generate"},
	{"schema/", "sqlc"},
	{"queries/", "sqlc"},
	{"sqlc.yaml", "sqlc"},
	{"locales/", "i18n-gen"},
	{"app/web/src/routes/", "web-dist"},
	{"internal/platform/config/", "config-docs"},
	{"docs/adr/", "adr-index"},
}

// Regen runs the generators the files changed since base need, in a stable order, and returns one line each.
func Regen(r *repo.Repo, base string, dryRun bool) ([]string, error) {
	changed, err := r.Changed(base)
	if err != nil {
		return nil, err
	}
	need := map[string]bool{}
	var order []string
	for _, g := range generators {
		for _, f := range changed {
			if strings.HasPrefix(f, g.prefix) && !need[g.target] {
				need[g.target] = true
				order = append(order, g.target)
			}
		}
	}
	if len(order) == 0 {
		return []string{"no generator source changed since " + base}, nil
	}
	var lines []string
	for _, t := range order {
		if dryRun {
			lines = append(lines, "would run make "+t)
			continue
		}
		start := time.Now()
		cmd := exec.Command("make", "--no-print-directory", t)
		cmd.Dir = r.Root
		out, err := cmd.CombinedOutput()
		if err != nil {
			lines = append(lines, fmt.Sprintf("make %s: FAILED", t))
			lines = append(lines, Failures(string(out), 30)...)
			return lines, fmt.Errorf("make %s failed", t)
		}
		lines = append(lines, fmt.Sprintf("make %s: ok (%s)", t, time.Since(start).Round(time.Second)))
	}
	status, _ := repo.Git(r.Root, "status", "--short")
	if s := strings.TrimSpace(status); s != "" {
		lines = append(lines, "changed files now:", s)
	}
	return lines, nil
}
