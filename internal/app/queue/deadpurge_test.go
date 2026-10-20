// SPDX-License-Identifier: AGPL-3.0-only

package queue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/queue"
)

// fakeDeadJobs holds dead jobs by failure time and records the cutoffs it was asked for.
type fakeDeadJobs struct {
	failed  []time.Time
	cutoffs []time.Time
	err     error
}

func (f *fakeDeadJobs) ListDeadJobs(context.Context, queue.DeadJobPosition, int) ([]queue.DeadJob, error) {
	return nil, nil
}

func (f *fakeDeadJobs) RequeueDeadJob(context.Context, string) error { return nil }

func (f *fakeDeadJobs) DeleteDeadJobsBefore(_ context.Context, t time.Time, limit int) (int64, error) {
	f.cutoffs = append(f.cutoffs, t)
	if f.err != nil {
		return 0, f.err
	}
	var n int64
	kept := f.failed[:0]
	for _, at := range f.failed {
		if at.Before(t) && n < int64(limit) {
			n++
			continue
		}
		kept = append(kept, at)
	}
	f.failed = kept
	return n, nil
}

func TestDeadJobPurgerDeletesPastRetentionInBatches(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	f := &fakeDeadJobs{}
	for i := range 5 {
		f.failed = append(f.failed, now.Add(-31*24*time.Hour-time.Duration(i)*time.Minute))
	}
	recent := now.Add(-29 * 24 * time.Hour)
	f.failed = append(f.failed, recent)
	p := queue.NewDeadJobPurger(queue.DeadJobPurgeDeps{DeadJobs: f, Clock: &stepClock{now: now}, Batch: 2})
	n, err := p.Run(t.Context())
	if err != nil || n != 5 {
		t.Fatalf("Run = %d, %v; want 5", n, err)
	}
	if len(f.failed) != 1 || !f.failed[0].Equal(recent) {
		t.Errorf("left %v, want only the job within the retention", f.failed)
	}
	if want := now.Add(-queue.DefaultDeadJobRetention); len(f.cutoffs) != 3 || !f.cutoffs[0].Equal(want) {
		t.Errorf("cutoffs = %v, want 3 batches before %v", f.cutoffs, want)
	}
}

func TestDeadJobPurgerUsesTheConfiguredRetention(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	f := &fakeDeadJobs{}
	p := queue.NewDeadJobPurger(queue.DeadJobPurgeDeps{DeadJobs: f, Clock: &stepClock{now: now}, Retention: 48 * time.Hour})
	if _, err := p.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(f.cutoffs) != 1 || !f.cutoffs[0].Equal(now.Add(-48*time.Hour)) {
		t.Errorf("cutoffs = %v, want one before %v", f.cutoffs, now.Add(-48*time.Hour))
	}
	f.err = errors.New("db down")
	if _, err := p.Run(t.Context()); !errors.Is(err, f.err) {
		t.Errorf("Run error = %v, want %v", err, f.err)
	}
}

func TestDeadJobPurgerRegistersAnHourlySchedule(t *testing.T) {
	h := &queue.JobHandlerRegistry{}
	s := queue.NewScheduler(&recordingQueue{}, &stepClock{}, nil)
	queue.NewDeadJobPurger(queue.DeadJobPurgeDeps{DeadJobs: &fakeDeadJobs{}, Clock: &stepClock{}}).RegisterJobs(h, s)
	got := s.Schedules()
	if len(got) != 1 || got[0].Job.Type != queue.JobPurgeDeadJobs || got[0].Every != queue.DeadJobPurgeInterval {
		t.Errorf("schedules = %+v, want the hourly dead job purge", got)
	}
}
