// SPDX-License-Identifier: AGPL-3.0-only

package queue_test

import (
	"context"
	"testing"

	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
)

// fakeTx emits events to its registry and keeps them, as a database adapter does, so that the
// test decides whether the transaction commits.
type fakeTx struct {
	projectors *uow.Projectors
	events     []uow.Event
}

func (tx *fakeTx) Emit(ctx context.Context, e uow.Event) error {
	tx.events = append(tx.events, e)
	return tx.projectors.Project(ctx, tx, e)
}

type keptJobs struct{ jobs []queue.NewJob }

func (k *keptJobs) Enqueue(_ context.Context, jobs ...queue.NewJob) error {
	k.jobs = append(k.jobs, jobs...)
	return nil
}

func TestAfterCommitEnqueuesOnlyCommittedJobs(t *testing.T) {
	projectors := &uow.Projectors{}
	mine, other := &keptJobs{}, &keptJobs{}
	outbox := queue.AfterCommit(projectors, mine, nil)
	queue.AfterCommit(projectors, other, nil)

	rolledBack := &fakeTx{projectors: projectors}
	if err := outbox.EnqueueTx(t.Context(), rolledBack, queue.NewJob{Type: "rolled.back"}); err != nil {
		t.Fatal(err)
	}
	if len(mine.jobs) != 0 {
		t.Fatalf("jobs before the commit = %v, want none", mine.jobs)
	}

	committed := &fakeTx{projectors: projectors}
	if err := outbox.EnqueueTx(t.Context(), committed, queue.NewJob{Type: "a"}, queue.NewJob{Type: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := outbox.EnqueueTx(t.Context(), committed); err != nil {
		t.Fatal(err)
	}
	projectors.Committed(t.Context(), committed.events)
	if len(mine.jobs) != 2 || mine.jobs[0].Type != "a" || mine.jobs[1].Type != "b" {
		t.Errorf("jobs after the commit = %v, want a and b", mine.jobs)
	}
	if len(other.jobs) != 0 {
		t.Errorf("another AfterCommit outbox got %v, want none", other.jobs)
	}
}
