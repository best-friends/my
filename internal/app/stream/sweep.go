// SPDX-License-Identifier: AGPL-3.0-only

package stream

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/internal/app/queue"
)

const (
	// JobSweep deletes the per-process resources that processes left behind on the broker
	// (Sweeper); no payload.
	JobSweep = "stream.sweep"
	// SweepInterval is the period of the sweep schedule.
	SweepInterval = 15 * time.Minute
	// SweepMinAge is the age under which a resource is never swept: a process writes its
	// liveness marker before it creates its resources, and the marker lives LivenessTTL.
	SweepMinAge = LivenessTTL
)

// RegisterSweepJob registers the handler of JobSweep on reg and its schedule on sched. The
// sweep keeps the resources of the processes whose liveness marker is in liveness: the store
// that their Relays write it to (RelayOptions.Liveness).
func RegisterSweepJob(reg *queue.JobHandlerRegistry, sched *queue.Scheduler, sweeper Sweeper, liveness kernel.KeyValueStore, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	reg.Register(JobSweep, func(ctx context.Context, _ queue.Job) queue.Result {
		n, err := sweeper.Sweep(ctx, SweepMinAge, func(ctx context.Context, processID string) (bool, error) {
			return Alive(ctx, liveness, processID)
		})
		if n > 0 {
			logger.InfoContext(ctx, "swept the stream resources of ended processes", slog.Int("deleted", n))
		}
		if err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
	sched.Add(queue.Schedule{Name: JobSweep, Every: SweepInterval, Job: queue.NewJob{Type: JobSweep}})
}

// Alive reports whether the process processID has a live liveness marker in liveness.
func Alive(ctx context.Context, liveness kernel.KeyValueStore, processID string) (bool, error) {
	_, err := liveness.Get(ctx, LivenessNamespace, processID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, kernel.ErrKeyNotFound):
		return false, nil
	}
	return false, err
}
