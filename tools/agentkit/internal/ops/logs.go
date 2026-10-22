// SPDX-License-Identifier: AGPL-3.0-only

// Package ops wraps the workflow steps of docs/agents/workflow.md — claiming and blocking issues, opening pull
// requests, waiting for CI, running the checks and the generators — so that each prints what the agent needs
// to act on and nothing else.
package ops

import (
	"regexp"
	"strings"
)

// errorPatterns match the lines of Go, golangci-lint, TypeScript, Vitest, Playwright, markdownlint, shellcheck,
// Atlas and make output that say what failed.
var errorPatterns = regexp.MustCompile(strings.Join([]string{
	`^\s*--- FAIL`,
	`^FAIL\b`,
	`^panic:`,
	`\.go:\d+(:\d+)?: `,
	`\.(ts|tsx|js|mjs):\d+(:\d+)?`,
	`\berror TS\d+`,
	`^\s*(×|✗|✘|FAIL)\s`,
	`^\s*(Error|AssertionError|TypeError|ReferenceError):`,
	`\bMD\d{3}\b`,
	`\bSC\d{4}\b`,
	`^make(\[\d+\])?: \*\*\*`,
	`^make check: .* FAILED`,
	`\bFAILED after\b`,
	`(?i)^\s*error:`,
	`^\s*Expected:|^\s*Received:|^\s*expected .* (to|but)`,
	`\bis stale\b|\bout of date\b|\bnot tidy\b`,
	`^diff --git `,
}, "|"))

// followers are the matches whose next lines stay with them: a failing test's output, a panic's stack.
var followers = regexp.MustCompile(`^\s*--- FAIL|^panic:|^\s*(Error|AssertionError):|\berror TS\d+|^\s*(×|✗|✘)\s`)

// timestamp strips the prefix GitHub Actions puts on every log line: "<job>\t<step>\t2026-10-22T01:02:03.456Z ".
var timestamp = regexp.MustCompile(`^(?:[^\t]*\t){0,2}\d{4}-\d\d-\d\dT[\d:.]+Z ?`)

// Failures returns the lines of log that say what failed, each with a little context, without duplicates,
// at most limit lines.
func Failures(log string, limit int) []string {
	lines := strings.Split(log, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(timestamp.ReplaceAllString(l, ""), "\r")
	}
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if !errorPatterns.MatchString(l) {
			continue
		}
		keep[i] = true
		if !followers.MatchString(l) {
			continue
		}
		// A panic keeps its stack; anything else keeps the indented lines under it (a test's output).
		panicking := strings.HasPrefix(l, "panic:")
		for j := i + 1; j < len(lines) && j <= i+12; j++ {
			if !panicking && !strings.HasPrefix(lines[j], " ") && !strings.HasPrefix(lines[j], "\t") {
				break
			}
			keep[j] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	gap := false
	for i, l := range lines {
		if !keep[i] {
			gap = true
			continue
		}
		t := strings.TrimSpace(l)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		if gap && len(out) > 0 {
			out = append(out, "  ⋮")
		}
		gap = false
		if len(out) >= limit {
			out = append(out, "  … more failures; see the full log")
			break
		}
		out = append(out, l)
	}
	return out
}
