// SPDX-License-Identifier: AGPL-3.0-only

package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
)

const (
	// JobExecuteDeletions deletes the accounts whose scheduled deletion is due (data-model
	// §4.2, §8): a ban's, or one that the account's owner asked for.
	JobExecuteDeletions = "account.execute_deletions"
	// DeletionInterval is the period of the JobExecuteDeletions schedule.
	DeletionInterval = time.Hour
	// deletionBatch bounds the accounts that one run deletes; the next run takes the rest.
	deletionBatch = 100
)

// ScheduleDeletionByModeration records inside tx that a moderation action (a ban) schedules
// the deletion of a local account at due: a deletion_scheduled lifecycle event initiated by a
// moderator that names the action (data-model §4.2). An account that is deleted already, or
// whose deletion is due no later than due, is left as it is. A remote account fails with
// ErrNotLocal.
func (s *Service) ScheduleDeletionByModeration(ctx context.Context, tx uow.Tx, accountID, actionID string, due time.Time) error {
	a, err := s.d.Repository.Account(ctx, tx, accountID)
	if err != nil {
		return err
	}
	if !a.IsLocal() {
		return fmt.Errorf("account %s: %w", accountID, ErrNotLocal)
	}
	due = kernel.Normalize(due)
	st, err := s.d.Repository.LifecycleState(ctx, tx, accountID)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return err
	case st.State == account.StateDeleted:
		return nil
	case st.State == account.StateDeletionScheduled && st.DeletionDueAt != nil && !st.DeletionDueAt.After(due):
		return nil
	}
	e := account.LifecycleEvent{
		ID: s.d.IDs.NewID(), AccountID: accountID, Kind: account.LifecycleDeletionScheduled,
		InitiatedBy: account.InitiatedByModerator, ModerationActionID: actionID, DeletionDueAt: &due,
		OccurredAt: s.d.Clock.Now(),
	}
	if err := s.d.Repository.InsertLifecycleEvent(ctx, tx, e); err != nil {
		return err
	}
	return tx.Emit(ctx, LifecycleEventRecorded{Event: e})
}

// RegisterDeletionJobs registers the JobExecuteDeletions handler and its hourly schedule.
func (s *Service) RegisterDeletionJobs(reg *queue.JobHandlerRegistry, sched *queue.Scheduler) {
	reg.Register(JobExecuteDeletions, func(ctx context.Context, _ queue.Job) queue.Result {
		if _, err := s.ExecuteDueDeletions(ctx); err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
	sched.Add(queue.Schedule{Name: JobExecuteDeletions, Every: DeletionInterval, Job: queue.NewJob{Type: JobExecuteDeletions}})
}

// ExecuteDueDeletions deletes up to deletionBatch accounts whose scheduled deletion is due: each
// gets a deleted lifecycle event initiated by the system, in a short transaction of its own,
// which hides it and its statuses at once and sends the Delete of the actor (federation.md
// §7.1). An account whose deletion was cancelled or rescheduled meanwhile is left as it is, so
// running it again is safe. It returns how many accounts it deleted.
func (s *Service) ExecuteDueDeletions(ctx context.Context) (int, error) {
	now := kernel.Normalize(s.d.Clock.Now())
	due, err := s.d.Repository.ListDueDeletions(ctx, now, deletionBatch)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, d := range due {
		id := s.d.IDs.NewID()
		var done bool
		err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			done = false
			st, err := s.d.Repository.LifecycleState(ctx, tx, d.AccountID)
			if err != nil {
				return err
			}
			if st.State != account.StateDeletionScheduled || st.DeletionDueAt == nil || st.DeletionDueAt.After(now) {
				return nil
			}
			e := account.LifecycleEvent{
				ID: id, AccountID: d.AccountID, Kind: account.LifecycleDeleted, InitiatedBy: account.InitiatedBySystem, OccurredAt: now,
			}
			if err := s.d.Repository.InsertLifecycleEvent(ctx, tx, e); err != nil {
				return err
			}
			done = true
			return tx.Emit(ctx, LifecycleEventRecorded{Event: e})
		})
		if err != nil {
			return deleted, fmt.Errorf("account: delete account %s: %w", d.AccountID, err)
		}
		if done {
			deleted++
		}
	}
	return deleted, nil
}
