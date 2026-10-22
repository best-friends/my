// SPDX-License-Identifier: AGPL-3.0-only

package ops

import (
	"strings"
	"testing"
)

func TestFailures(t *testing.T) {
	log := strings.Join([]string{
		"go\tgo test\t2026-10-22T01:02:03.456Z ok  \tgithub.com/best-friends/my/internal/app/content\t1.2s",
		"go\tgo test\t2026-10-22T01:02:03.456Z --- FAIL: TestSubmitRequest (0.01s)",
		"go\tgo test\t2026-10-22T01:02:03.456Z     request_test.go:42: got 2 requests, want 1",
		"go\tgo test\t2026-10-22T01:02:03.456Z FAIL",
		"go\tgo test\t2026-10-22T01:02:03.456Z FAIL\tgithub.com/best-friends/my/internal/app/content\t0.3s",
		"unrelated noise",
		"src/features/a.tsx(3,1): error TS2322: Type 'string' is not assignable to type 'number'.",
		"make: *** [Makefile:12: go-test] Error 1",
	}, "\n")
	got := Failures(log, 50)
	joined := strings.Join(got, "\n")
	for _, want := range []string{"--- FAIL: TestSubmitRequest", "request_test.go:42: got 2 requests", "error TS2322", "make: ***"} {
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

func TestFailuresCap(t *testing.T) {
	log := strings.Repeat("FAIL\tpkg/x\n--- FAIL: TestY\n", 100)
	if got := Failures(log, 10); len(got) > 12 {
		t.Errorf("Failures returned %d lines, want at most about 10", len(got))
	}
}
