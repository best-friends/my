// SPDX-License-Identifier: AGPL-3.0-only

package queue_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
)

// recordingMeter keeps the histogram values by metric and labels. (testkit.Meter does the same,
// but the application layer does not import pkg/testkit.)
type recordingMeter struct {
	kernel.NopMeter
	mu     sync.Mutex
	values map[string][]float64
}

type recordingHistogram struct {
	m *recordingMeter
	i kernel.Instrument
}

func (h recordingHistogram) Record(_ context.Context, v float64, l kernel.Labels) {
	h.m.mu.Lock()
	defer h.m.mu.Unlock()
	key := h.i.Name
	for k, val := range l.All() {
		if !h.i.Accepts(k) {
			panic("undeclared label " + k)
		}
		key += " " + k + "=" + val
	}
	h.m.values[key] = append(h.m.values[key], v)
}

func (m *recordingMeter) Histogram(i kernel.Instrument) kernel.Histogram {
	return recordingHistogram{m: m, i: i}
}

// jobsSource hands out its jobs one by one, due an hour before the clock.
type jobsSource struct {
	emptySource
	jobs []queue.Job
}

func (s *jobsSource) Claim(_ context.Context, _ string, now time.Time, _ time.Duration) (queue.Job, bool, error) {
	if len(s.jobs) == 0 {
		return queue.Job{}, false, nil
	}
	j := s.jobs[0]
	s.jobs = s.jobs[1:]
	j.Attempt, j.RunAt = 1, now.Add(-time.Hour)
	return j, true, nil
}

func TestRunnerRecordsJobMetrics(t *testing.T) {
	src := &jobsSource{emptySource: emptySource{claims: make(chan struct{}, 1)}, jobs: []queue.Job{
		{ID: "1", Type: "ok"}, {ID: "2", Type: "flaky"}, {ID: "3", Type: "bad"}, {ID: "4", Type: "unknown.type"},
	}}
	reg := &queue.JobHandlerRegistry{}
	reg.Register("ok", func(context.Context, queue.Job) queue.Result { return queue.Done() })
	reg.Register("flaky", func(context.Context, queue.Job) queue.Result { return queue.Retry(0, errors.New("later")) })
	reg.Register("bad", func(context.Context, queue.Job) queue.Result { return queue.Fail(errors.New("never")) })
	m := &recordingMeter{values: map[string][]float64{}}
	r := queue.NewPullRunner(src, reg, queue.RunnerOptions{Clock: clock{}, Meter: m})
	if n, err := r.Drain(t.Context()); err != nil || n != 4 {
		t.Fatalf("Drain() = %d, %v", n, err)
	}
	for _, key := range []string{
		"my.job.duration my.job.type=ok my.outcome=success",
		"my.job.duration my.job.type=flaky my.outcome=retry",
		"my.job.duration my.job.type=bad my.outcome=dead",
		"my.job.duration my.job.type=other my.outcome=retry",
		"my.job.wait my.job.type=ok",
		"my.job.wait my.job.type=other",
	} {
		if len(m.values[key]) != 1 {
			t.Errorf("%s: %v, want one value", key, m.values[key])
		}
	}
	if got := m.values["my.job.wait my.job.type=ok"]; len(got) == 1 && got[0] != 3600 {
		t.Errorf("my.job.wait = %v, want 3600 seconds", got[0])
	}
}
