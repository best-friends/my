// SPDX-License-Identifier: AGPL-3.0-only

package uow_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/uow"
)

var fast = uow.RetryPolicy{BaseDelay: time.Microsecond, MaxDelay: time.Microsecond}

func TestRetryStopsOnSuccess(t *testing.T) {
	var retried []int
	p := fast
	p.OnRetry = func(attempt int, err error) {
		if !errors.Is(err, uow.ErrRetryable) {
			t.Errorf("OnRetry(%d, %v): error is not ErrRetryable", attempt, err)
		}
		retried = append(retried, attempt)
	}
	calls := 0
	err := uow.Retry(t.Context(), p, func() error {
		calls++
		if calls < 3 {
			return uow.MarkRetryable(errors.New("conflict"))
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("Retry = %v after %d calls, want nil after 3", err, calls)
	}
	if !slices.Equal(retried, []int{1, 2}) {
		t.Errorf("OnRetry attempts = %v, want [1 2]", retried)
	}
}

func TestRetryGivesUpAfterMaxAttempts(t *testing.T) {
	for _, tc := range []struct {
		max, want int
	}{{0, uow.DefaultMaxAttempts}, {1, 1}, {3, 3}} {
		p := fast
		p.MaxAttempts = tc.max
		calls := 0
		err := uow.Retry(t.Context(), p, func() error {
			calls++
			return uow.MarkRetryable(errors.New("conflict"))
		})
		if !errors.Is(err, uow.ErrRetryable) || calls != tc.want {
			t.Errorf("MaxAttempts=%d: Retry = %v after %d calls, want ErrRetryable after %d", tc.max, err, calls, tc.want)
		}
	}
}

func TestRetryDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	err := uow.Retry(t.Context(), fast, func() error {
		calls++
		return fs.ErrNotExist
	})
	if !errors.Is(err, fs.ErrNotExist) || calls != 1 {
		t.Fatalf("Retry = %v after %d calls, want fs.ErrNotExist after 1", err, calls)
	}
}

func TestRetryStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	p := uow.RetryPolicy{BaseDelay: time.Hour, MaxDelay: time.Hour}
	calls := 0
	err := uow.Retry(ctx, p, func() error {
		calls++
		cancel()
		return uow.MarkRetryable(errors.New("conflict"))
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, uow.ErrRetryable) || calls != 1 {
		t.Fatalf("Retry = %v after %d calls, want context.Canceled and ErrRetryable after 1", err, calls)
	}
}

type driverError struct{ code string }

func (e *driverError) Error() string { return "driver: " + e.code }

func TestMarkRetryable(t *testing.T) {
	if uow.MarkRetryable(nil) != nil {
		t.Error("MarkRetryable(nil) != nil")
	}
	cause := &driverError{code: "40001"}
	err := uow.MarkRetryable(cause)
	if !errors.Is(err, uow.ErrRetryable) {
		t.Error("marked error is not ErrRetryable")
	}
	var de *driverError
	if !errors.As(err, &de) || de != cause {
		t.Error("marked error does not unwrap to the driver error")
	}
	if err.Error() != cause.Error() {
		t.Errorf("Error() = %q, want %q", err.Error(), cause.Error())
	}
	if again := uow.MarkRetryable(err); again != err { //nolint:errorlint // identity: a marked error must come back unwrapped again
		t.Error("MarkRetryable wrapped an already marked error again")
	}
}

type event string

func (e event) EventType() string { return string(e) }

type fakeTx struct{}

func (fakeTx) Emit(context.Context, uow.Event) error { return nil }

func TestProjectorsRunInRegistrationOrderAndStopAtError(t *testing.T) {
	var r uow.Projectors
	var got []string
	record := func(name string, err error) uow.Projector {
		return func(_ context.Context, tx uow.Tx, e uow.Event) error {
			if _, ok := tx.(fakeTx); !ok {
				t.Errorf("projector %s got tx %T, want the emitting tx", name, tx)
			}
			got = append(got, name+":"+e.EventType())
			return err
		}
	}
	boom := errors.New("boom")
	r.Register("a", record("first", nil))
	r.Register("b", record("other", nil))
	r.Register("a", record("second", boom))
	r.Register("a", record("third", nil))

	if err := r.Project(t.Context(), fakeTx{}, event("a")); !errors.Is(err, boom) {
		t.Fatalf("Project = %v, want %v", err, boom)
	}
	if want := []string{"first:a", "second:a"}; !slices.Equal(got, want) {
		t.Errorf("ran %v, want %v", got, want)
	}
	if err := r.Project(t.Context(), fakeTx{}, event("none")); err != nil {
		t.Errorf("Project with no subscribers = %v, want nil", err)
	}
}

func TestNilProjectorsIsEmpty(t *testing.T) {
	var r *uow.Projectors
	if err := r.Project(t.Context(), fakeTx{}, event("a")); err != nil {
		t.Fatalf("nil registry: Project = %v, want nil", err)
	}
}
