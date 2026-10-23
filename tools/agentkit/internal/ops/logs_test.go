// SPDX-License-Identifier: AGPL-3.0-only

package ops

import (
	"strings"
	"testing"
)

func TestFailures(t *testing.T) {
	log := strings.Join([]string{
		"go\tgo test\t2026-10-22T01:02:03.456Z ok  \tgithub.com/mycroblog/my/internal/app/content\t1.2s",
		"go\tgo test\t2026-10-22T01:02:03.456Z --- FAIL: TestSubmitRequest (0.01s)",
		"go\tgo test\t2026-10-22T01:02:03.456Z     request_test.go:42: got 2 requests, want 1",
		"go\tgo test\t2026-10-22T01:02:03.456Z FAIL",
		"go\tgo test\t2026-10-22T01:02:03.456Z FAIL\tgithub.com/mycroblog/my/internal/app/content\t0.3s",
		"unrelated noise",
		"src/features/a.tsx(3,1): error TS2322: Type 'string' is not assignable to type 'number'.",
		`task: Failed to run task "go:test": exit status 1`,
	}, "\n")
	got := Failures(log, 50)
	joined := strings.Join(got, "\n")
	for _, want := range []string{"--- FAIL: TestSubmitRequest", "request_test.go:42: got 2 requests", "error TS2322", `task: Failed to run task "go:test"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("Failures lacks %q:\n%s", want, joined)
		}
	}
	for _, unwanted := range []string{"unrelated noise", "2026-10-22T", "ok  \t"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("Failures keeps %q:\n%s", unwanted, joined)
		}
	}
}

func TestFailuresMapCheck(t *testing.T) {
	log := "2026-10-22T02:44:27.1Z bin/agentkit map check\n" +
		"2026-10-22T02:44:27.2Z error: app/web/src/features/supporters/: matches strict pattern \"app/web/src/features/*\" but no entry names it\n" +
		"2026-10-22T02:44:27.3Z task: Failed to run task \"agentkit:map-check\": exit status 1\n"
	if got := strings.Join(Failures(log, 10), "\n"); !strings.Contains(got, "features/supporters/: matches strict pattern") {
		t.Errorf("Failures lacks the map check problem:\n%s", got)
	}
}

// TestFailuresTaskOutput checks the local output of go-task: the task name before each line, which must not
// hide a failing test's output, and the commands it echoes, which are not failures.
func TestFailuresTaskOutput(t *testing.T) {
	log := strings.Join([]string{
		"task: [go:test] go test ./...",
		"[go:test] --- FAIL: TestSubmitRequest (0.01s)",
		"[go:test]     request_test.go:42: got 2 requests, want 1",
		"task: [go:vet] echo '    x_test.go:3: not a failure'",
		"[go:vet] ok",
		"\x1b[31mtask: Failed to run task \"check\": task: Failed to run task \"go:test\": exit status 1\x1b[0m",
	}, "\n")
	got := Failures(log, 50)
	joined := strings.Join(got, "\n")
	for _, want := range []string{"--- FAIL: TestSubmitRequest", "    request_test.go:42: got 2 requests", `Failed to run task "go:test"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("Failures lacks %q:\n%s", want, joined)
		}
	}
	for _, unwanted := range []string{"[go:test]", "not a failure", "\x1b["} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("Failures keeps %q:\n%s", unwanted, joined)
		}
	}
}

func TestFailuresCap(t *testing.T) {
	log := strings.Repeat("FAIL\tpkg/x\n--- FAIL: TestY\n", 100)
	if got := Failures(log, 10); len(got) > 12 {
		t.Errorf("Failures returned %d lines, want at most about 10", len(got))
	}
}
