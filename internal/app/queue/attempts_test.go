// SPDX-License-Identifier: AGPL-3.0-only

package queue_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/queue"
)

func TestTruncateAttemptError(t *testing.T) {
	short := "connection refused"
	if got := queue.TruncateAttemptError(short); got != short {
		t.Errorf("TruncateAttemptError(%q) = %q", short, got)
	}
	// 3-byte runes: the cut at 2,000 bytes falls inside the 667th rune.
	long := strings.Repeat("あ", 700)
	got := queue.TruncateAttemptError(long)
	if len(got) != 1998 || got != long[:1998] {
		t.Errorf("TruncateAttemptError(700 runes) has %d bytes, want 1998 (666 whole runes)", len(got))
	}
}

// attemptRecorder is an AttemptLog that keeps what it is given, or fails.
type attemptRecorder struct {
	got []queue.Attempt
	err error
}

func (r *attemptRecorder) RecordAttempt(_ context.Context, a queue.Attempt) error {
	r.got = append(r.got, a)
	return r.err
}

func (r *attemptRecorder) ListAttempts(context.Context, string, queue.AttemptPosition, int) ([]queue.Attempt, error) {
	return nil, nil
}

func (r *attemptRecorder) DeleteAttemptsBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

// buryingSource is a jobsSource that records the results the runner gives it.
type buryingSource struct {
	jobsSource
	released, buried []string
}

func (s *buryingSource) Release(_ context.Context, _ string, j queue.Job, _ int, _ time.Time, _ string) error {
	s.released = append(s.released, j.ID)
	return nil
}

func (s *buryingSource) Bury(_ context.Context, _ string, j queue.Job, _ string, _ time.Time) error {
	s.buried = append(s.buried, j.ID)
	return nil
}

func TestRunnerRecordsUnsuccessfulAttempts(t *testing.T) {
	for _, recordErr := range []error{nil, errors.New("db down")} {
		src := &buryingSource{jobsSource: jobsSource{jobs: []queue.Job{
			{ID: "1", Type: "ok"}, {ID: "2", Type: "flaky"}, {ID: "3", Type: "bad"},
		}}}
		reg := &queue.JobHandlerRegistry{}
		reg.Register("ok", func(context.Context, queue.Job) queue.Result { return queue.Done() })
		reg.Register("flaky", func(context.Context, queue.Job) queue.Result { return queue.Retry(0, errors.New("later")) })
		reg.Register("bad", func(context.Context, queue.Job) queue.Result { return queue.Fail(errors.New("never")) })
		log := &attemptRecorder{err: recordErr}
		r := queue.NewPullRunner(src, reg, queue.RunnerOptions{
			Clock: clock{}, Worker: "w1", Attempts: log, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		if n, err := r.Drain(t.Context()); err != nil || n != 3 {
			t.Fatalf("Drain() = %d, %v", n, err)
		}
		// A failure to record leaves the results alone.
		if len(src.released) != 1 || src.released[0] != "2" || len(src.buried) != 1 || src.buried[0] != "3" {
			t.Errorf("released %v, buried %v; want [2], [3]", src.released, src.buried)
		}
		if len(log.got) != 2 {
			t.Fatalf("recorded %+v, want the retry and the dead job only", log.got)
		}
		now := clock{}.Now()
		want := []queue.Attempt{
			{JobID: "2", JobType: "flaky", Attempt: 1, Outcome: queue.OutcomeRetry, Error: "later", Worker: "w1", StartedAt: now, FinishedAt: now},
			{JobID: "3", JobType: "bad", Attempt: 1, Outcome: queue.OutcomeDead, Error: "never", Worker: "w1", StartedAt: now, FinishedAt: now},
		}
		for i, w := range want {
			if log.got[i] != w {
				t.Errorf("attempt %d = %+v, want %+v", i, log.got[i], w)
			}
		}
	}
}
