// SPDX-License-Identifier: AGPL-3.0-only

package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/mycroblog/my/internal/app/kernel"
)

// Schedule is a periodic job (ADR-0004): every Every, the scheduler enqueues Job. The trigger
// only enqueues; the job runs on the queue like any other, so its handler must be idempotent.
type Schedule struct {
	// Name identifies the schedule; it is part of the jobs' idempotency key.
	Name  string
	Every time.Duration
	Job   NewJob
}

// DefaultSchedulerTick is how often Scheduler.Run checks for due schedules.
const DefaultSchedulerTick = time.Minute

// Scheduler is the in-process Scheduler port (ADR-0004): it enqueues the job of every schedule
// once per period. Time is divided into slots of each schedule's period since the Unix epoch,
// and the job of a slot carries the idempotency key "schedule:<name>:<slot>", so that several
// processes with a scheduler enqueue one job per slot while it is pending.
//
// Platforms with their own scheduler (Cloud Scheduler, EventBridge Scheduler, a Kubernetes
// CronJob) can enqueue the same jobs instead and leave this one unused.
type Scheduler struct {
	q      JobQueue
	clock  kernel.Clock
	logger *slog.Logger

	mu        sync.Mutex
	schedules []Schedule
	last      map[string]int64
}

// NewScheduler returns a scheduler with no schedules that enqueues on q. A nil logger means
// slog.Default().
func NewScheduler(q JobQueue, clock kernel.Clock, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{q: q, clock: clock, logger: logger, last: map[string]int64{}}
}

// Add registers a schedule. It panics on an empty or repeated name, a period under a second,
// or an empty job type. Add schedules at startup, before Run.
func (s *Scheduler) Add(sc Schedule) {
	if sc.Name == "" || sc.Every < time.Second || sc.Job.Type == "" {
		panic("queue: a schedule needs a name, a period of at least a second, and a job type")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.schedules {
		if other.Name == sc.Name {
			panic(fmt.Sprintf("queue: schedule %q added twice", sc.Name))
		}
	}
	s.schedules = append(s.schedules, sc)
}

// Schedules returns the registered schedules.
func (s *Scheduler) Schedules() []Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Schedule(nil), s.schedules...)
}

// Tick enqueues the job of every schedule whose current slot it has not enqueued yet. The
// first Tick enqueues every schedule. A failed enqueue is logged and tried again on the next
// Tick.
func (s *Scheduler) Tick(ctx context.Context) {
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sc := range s.schedules {
		slot := now.UnixMilli() / sc.Every.Milliseconds()
		if last, ok := s.last[sc.Name]; ok && last == slot {
			continue
		}
		job := sc.Job
		job.IdempotencyKey = "schedule:" + sc.Name + ":" + strconv.FormatInt(slot, 10)
		if err := s.q.Enqueue(ctx, job); err != nil {
			s.logger.WarnContext(ctx, "scheduled job not enqueued", slog.String("schedule", sc.Name), slog.Any("error", err))
			continue
		}
		s.last[sc.Name] = slot
	}
}

// RunUntil calls Tick at once and then every tick (DefaultSchedulerTick when tick <= 0) until
// ctx ends or stop is closed.
func (s *Scheduler) RunUntil(ctx context.Context, stop <-chan struct{}, tick time.Duration) {
	if tick <= 0 {
		tick = DefaultSchedulerTick
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		s.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-t.C:
		}
	}
}
