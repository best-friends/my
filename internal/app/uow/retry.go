// SPDX-License-Identifier: AGPL-3.0-only

package uow

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

// ErrRetryable marks a transaction failure that a new attempt may resolve: a serialization
// failure or deadlock (PostgreSQL SQLSTATE 40001 / 40P01) or a lock timeout (SQLITE_BUSY).
// Adapters wrap driver errors with MarkRetryable; errors.As still reaches the driver error.
var ErrRetryable = errors.New("uow: retryable transaction conflict")

// Defaults applied to the zero fields of a RetryPolicy.
const (
	DefaultMaxAttempts = 5
	DefaultBaseDelay   = 2 * time.Millisecond
	DefaultMaxDelay    = 100 * time.Millisecond
)

// RetryPolicy bounds the retries of UnitOfWork.Tx. Zero fields take the defaults above.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts int
	// BaseDelay is the upper bound of the first backoff. It doubles after every retry, up to
	// MaxDelay; each wait is drawn uniformly from (0, bound] (full jitter).
	BaseDelay time.Duration
	MaxDelay  time.Duration
	// OnRetry, when set, is called with the failed attempt's number (from 1) and its error
	// before waiting for the next attempt; use it for metrics and logs.
	OnRetry func(attempt int, err error)
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = DefaultMaxAttempts
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = DefaultBaseDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = DefaultMaxDelay
	}
	return p
}

// Retry calls attempt until it returns nil or an error that is not ErrRetryable, the policy's
// attempts run out, or ctx ends. It returns the last attempt's error; when ctx ends during a
// backoff, the error also matches ctx.Err().
func Retry(ctx context.Context, p RetryPolicy, attempt func() error) error {
	p = p.withDefaults()
	bound := p.BaseDelay
	for n := 1; ; n++ {
		err := attempt()
		if err == nil || !errors.Is(err, ErrRetryable) || n >= p.MaxAttempts {
			return err
		}
		if p.OnRetry != nil {
			p.OnRetry(n, err)
		}
		wait := time.Duration(rand.Int64N(int64(bound))) + 1 //nolint:gosec // backoff jitter needs no cryptographic randomness
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
		bound = min(bound*2, p.MaxDelay)
	}
}

// MarkRetryable wraps err so that errors.Is(err, ErrRetryable) holds while errors.Is and
// errors.As still reach err. It returns nil for nil and err itself when it is already marked.
func MarkRetryable(err error) error {
	if err == nil || errors.Is(err, ErrRetryable) {
		return err
	}
	return retryableError{err}
}

type retryableError struct{ err error }

func (e retryableError) Error() string   { return e.err.Error() }
func (e retryableError) Unwrap() []error { return []error{e.err, ErrRetryable} }
