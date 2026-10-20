// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/uow"
)

// UnitOfWorkHarness is one uow.UnitOfWork under test, connected to its own empty database that
// holds a probe table: a table keyed by a unique string, which the contract writes to observe
// commits and rollbacks.
type UnitOfWorkHarness struct {
	UnitOfWork uow.UnitOfWork
	// Insert writes a probe row with key inside tx. Inserting a key that exists in the
	// transaction's view of the table must fail.
	Insert func(ctx context.Context, tx uow.Tx, key string) error
	// Keys returns the committed probe keys in any order, reading outside every transaction of
	// UnitOfWork. It must not block while a transaction is open.
	Keys func(ctx context.Context) ([]string, error)
	// Has reports whether a probe row with key exists, reading as the adapter's repositories read
	// outside transactions: in the transaction that ctx reads in (uow.ReadingIn), and on its own
	// otherwise.
	Has func(ctx context.Context, key string) (bool, error)
}

// UnitOfWorkFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type UnitOfWorkFactory func(t *testing.T, opts uow.Options) UnitOfWorkHarness

// fastRetry keeps the contract's retry waits short.
var fastRetry = uow.RetryPolicy{BaseDelay: time.Microsecond, MaxDelay: time.Millisecond}

type probeEvent struct{ key string }

func (probeEvent) EventType() string { return "testkit.probe_inserted" }

// UnitOfWorkContract verifies that the UnitOfWork built by newHarness satisfies the uow
// contract: commit on success; rollback on error and panic; bounded retries of ErrRetryable
// failures in fresh transactions; no retry of other errors; respect for context cancellation;
// isolation of uncommitted writes; reads in the transaction a context carries (uow.ReadingIn);
// concurrent writers; synchronous projectors running in
// the same transaction as the command that emits their event; and commit hooks running once,
// after the commit, for the events of the attempt that committed.
func UnitOfWorkContract(t *testing.T, newHarness UnitOfWorkFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, newHarness UnitOfWorkFactory)
	}{
		{"CommitsOnSuccess", uowCommitsOnSuccess},
		{"RollsBackOnError", uowRollsBackOnError},
		{"RollsBackOnPanic", uowRollsBackOnPanic},
		{"RetriesRetryableErrorsInFreshTransactions", uowRetriesInFreshTransactions},
		{"GivesUpAfterMaxAttempts", uowGivesUpAfterMaxAttempts},
		{"DoesNotRetryOtherErrors", uowDoesNotRetryOtherErrors},
		{"StopsWhenContextEnds", uowStopsWhenContextEnds},
		{"HidesUncommittedWrites", uowHidesUncommittedWrites},
		{"ReadsInTheTransactionOfTheContext", uowReadsInTheTransactionOfTheContext},
		{"SerializesConcurrentWriters", uowConcurrentWriters},
		{"RunsProjectorsInTheSameTransaction", uowRunsProjectors},
		{"RollsBackWhenAProjectorFails", uowRollsBackOnProjectorError},
		{"RunsCommitHooksAfterTheCommit", uowRunsCommitHooks},
		{"RunsNoCommitHooksOnRollback", uowRunsNoCommitHooksOnRollback},
		{"RunsCommitHooksOnlyForTheCommittedAttempt", uowRunsCommitHooksOnce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newHarness) })
	}
}

func wantKeys(t *testing.T, h UnitOfWorkHarness, want ...string) {
	t.Helper()
	got, err := h.Keys(t.Context())
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("committed keys = %q, want %q", got, want)
	}
}

func uowCommitsOnSuccess(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: fastRetry})
	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		return h.Insert(t.Context(), tx, "b")
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	wantKeys(t, h, "a", "b")
}

func uowRollsBackOnError(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: fastRetry})
	boom := errors.New("boom")
	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		return fmt.Errorf("wrapped: %w", boom)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx = %v, want an error wrapping %v", err, boom)
	}
	wantKeys(t, h)
}

func uowRollsBackOnPanic(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: fastRetry})
	recovered := func() (r any) {
		defer func() { r = recover() }()
		_ = h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
			if err := h.Insert(t.Context(), tx, "a"); err != nil {
				return err
			}
			panic("boom")
		})
		return nil
	}()
	if recovered != "boom" {
		t.Fatalf("recovered %v, want the panic to propagate out of Tx", recovered)
	}
	wantKeys(t, h)
	// The rollback must also have released the connection and its locks.
	if err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error { return h.Insert(t.Context(), tx, "b") }); err != nil {
		t.Fatalf("Tx after a panic = %v", err)
	}
	wantKeys(t, h, "b")
}

func uowRetriesInFreshTransactions(t *testing.T, newHarness UnitOfWorkFactory) {
	var retried []int
	p := fastRetry
	p.OnRetry = func(attempt int, _ error) { retried = append(retried, attempt) }
	h := newHarness(t, uow.Options{Retry: p})
	attempts := 0
	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		attempts++
		// The same key every attempt: this fails unless the previous attempt was rolled back.
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		if attempts < 3 {
			return uow.MarkRetryable(errors.New("conflict"))
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("Tx = %v after %d attempts, want nil after 3", err, attempts)
	}
	if !slices.Equal(retried, []int{1, 2}) {
		t.Errorf("OnRetry attempts = %v, want [1 2]", retried)
	}
	wantKeys(t, h, "a")
}

func uowGivesUpAfterMaxAttempts(t *testing.T, newHarness UnitOfWorkFactory) {
	p := fastRetry
	p.MaxAttempts = 4
	h := newHarness(t, uow.Options{Retry: p})
	attempts := 0
	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		attempts++
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		return uow.MarkRetryable(errors.New("conflict"))
	})
	if !errors.Is(err, uow.ErrRetryable) || attempts != 4 {
		t.Fatalf("Tx = %v after %d attempts, want ErrRetryable after 4", err, attempts)
	}
	wantKeys(t, h)
}

func uowDoesNotRetryOtherErrors(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: fastRetry})
	boom := errors.New("boom")
	attempts := 0
	err := h.UnitOfWork.Tx(t.Context(), func(uow.Tx) error {
		attempts++
		return boom
	})
	if !errors.Is(err, boom) || attempts != 1 {
		t.Fatalf("Tx = %v after %d attempts, want %v after 1", err, attempts, boom)
	}
	// A duplicate key is a constraint violation, not a conflict: it must not be retried either.
	attempts = 0
	err = h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		attempts++
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		return h.Insert(t.Context(), tx, "a")
	})
	if err == nil || errors.Is(err, uow.ErrRetryable) || attempts != 1 {
		t.Fatalf("duplicate key: Tx = %v after %d attempts, want a non-retryable error after 1", err, attempts)
	}
	wantKeys(t, h)
}

func uowStopsWhenContextEnds(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{BaseDelay: time.Hour, MaxDelay: time.Hour}})

	ctx, cancel := context.WithCancel(t.Context())
	attempts := 0
	err := h.UnitOfWork.Tx(ctx, func(uow.Tx) error {
		attempts++
		cancel()
		return uow.MarkRetryable(errors.New("conflict"))
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("cancel during backoff: Tx = %v after %d attempts, want context.Canceled after 1", err, attempts)
	}

	attempts = 0
	err = h.UnitOfWork.Tx(ctx, func(uow.Tx) error {
		attempts++
		return nil
	})
	if !errors.Is(err, context.Canceled) || attempts != 0 {
		t.Fatalf("canceled context: Tx = %v after %d attempts, want context.Canceled without calling fn", err, attempts)
	}
	wantKeys(t, h)
}

func uowHidesUncommittedWrites(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: fastRetry})
	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		got, err := h.Keys(t.Context())
		if err != nil {
			return err
		}
		if len(got) != 0 {
			t.Errorf("keys visible outside the open transaction = %q, want none", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	wantKeys(t, h, "a")
}

func uowReadsInTheTransactionOfTheContext(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: fastRetry})
	ctx := t.Context()
	err := h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := h.Insert(ctx, tx, "a"); err != nil {
			return err
		}
		if _, ok := uow.ReadingTx(ctx); ok {
			t.Error("ReadingTx of a plain context = true")
		}
		outside, err := h.Has(ctx, "a")
		if err != nil {
			return err
		}
		inside, err := h.Has(uow.ReadingIn(ctx, tx), "a")
		if err != nil {
			return err
		}
		if outside || !inside {
			t.Errorf("the uncommitted row read outside = %t, in the transaction = %t; want false, true", outside, inside)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	wantKeys(t, h, "a")
}

func uowConcurrentWriters(t *testing.T, newHarness UnitOfWorkFactory) {
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}})
	const writers = 16
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	want := make([]string, writers)
	for i := range writers {
		key := fmt.Sprintf("k%02d", i)
		want[i] = key
		wg.Go(func() {
			errs <- h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
				if err := h.Insert(t.Context(), tx, key); err != nil {
					return err
				}
				return h.Insert(t.Context(), tx, key+"-2")
			})
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Tx = %v", err)
		}
	}
	for i := range writers {
		want = append(want, want[i]+"-2")
	}
	wantKeys(t, h, want...)
}

// projectInto registers a projector that writes "<key>-projected" for every probe event.
func projectInto(h UnitOfWorkHarness, r *uow.Projectors, fail error) {
	r.Register(probeEvent{}.EventType(), func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		if err := h.Insert(ctx, tx, e.(probeEvent).key+"-projected"); err != nil {
			return err
		}
		return fail
	})
}

func uowRunsProjectors(t *testing.T, newHarness UnitOfWorkFactory) {
	var projectors uow.Projectors
	h := newHarness(t, uow.Options{Retry: fastRetry, Projectors: &projectors})
	projectInto(h, &projectors, nil)

	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		return tx.Emit(t.Context(), probeEvent{key: "a"})
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	wantKeys(t, h, "a", "a-projected")
}

func uowRollsBackOnProjectorError(t *testing.T, newHarness UnitOfWorkFactory) {
	var projectors uow.Projectors
	h := newHarness(t, uow.Options{Retry: fastRetry, Projectors: &projectors})
	boom := errors.New("projector failed")
	projectInto(h, &projectors, boom)

	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := h.Insert(t.Context(), tx, "a"); err != nil {
			return err
		}
		return tx.Emit(t.Context(), probeEvent{key: "a"})
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx = %v, want %v", err, boom)
	}
	wantKeys(t, h)
}

// hookInto registers a commit hook that records the key of every probe event with the
// committed keys it sees.
func hookInto(t *testing.T, h UnitOfWorkHarness, r *uow.Projectors) *[]string {
	t.Helper()
	var seen []string
	r.OnCommit(probeEvent{}.EventType(), func(ctx context.Context, e uow.Event) {
		keys, err := h.Keys(ctx)
		if err != nil {
			t.Errorf("Keys in a commit hook: %v", err)
		}
		slices.Sort(keys)
		seen = append(seen, fmt.Sprintf("%s%q", e.(probeEvent).key, keys))
	})
	return &seen
}

func uowRunsCommitHooks(t *testing.T, newHarness UnitOfWorkFactory) {
	var projectors uow.Projectors
	h := newHarness(t, uow.Options{Retry: fastRetry, Projectors: &projectors})
	seen := hookInto(t, h, &projectors)

	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		for _, key := range []string{"a", "b"} {
			if err := h.Insert(t.Context(), tx, key); err != nil {
				return err
			}
			if err := tx.Emit(t.Context(), probeEvent{key: key}); err != nil {
				return err
			}
			if len(*seen) > 0 {
				t.Errorf("a commit hook ran inside the transaction: %q", *seen)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	// Each hook sees the whole commit, in the order the events were emitted.
	want := []string{`a["a" "b"]`, `b["a" "b"]`}
	if !slices.Equal(*seen, want) {
		t.Errorf("commit hooks saw %q, want %q", *seen, want)
	}
}

func uowRunsNoCommitHooksOnRollback(t *testing.T, newHarness UnitOfWorkFactory) {
	var projectors uow.Projectors
	h := newHarness(t, uow.Options{Retry: fastRetry, Projectors: &projectors})
	seen := hookInto(t, h, &projectors)
	boom := errors.New("command failed")

	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := tx.Emit(t.Context(), probeEvent{key: "a"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx = %v, want %v", err, boom)
	}
	if len(*seen) > 0 {
		t.Errorf("commit hooks ran after a rollback: %q", *seen)
	}
}

func uowRunsCommitHooksOnce(t *testing.T, newHarness UnitOfWorkFactory) {
	var projectors uow.Projectors
	h := newHarness(t, uow.Options{Retry: fastRetry, Projectors: &projectors})
	seen := hookInto(t, h, &projectors)

	attempts := 0
	err := h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		attempts++
		key := fmt.Sprintf("attempt-%d", attempts)
		if err := h.Insert(t.Context(), tx, key); err != nil {
			return err
		}
		if err := tx.Emit(t.Context(), probeEvent{key: key}); err != nil {
			return err
		}
		if attempts < 2 {
			return uow.MarkRetryable(errors.New("conflict"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
	if want := []string{`attempt-2["attempt-2"]`}; !slices.Equal(*seen, want) {
		t.Errorf("commit hooks saw %q, want %q", *seen, want)
	}
}
