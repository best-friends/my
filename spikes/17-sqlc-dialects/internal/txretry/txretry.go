// SPDX-License-Identifier: AGPL-3.0-only

// Package txretry is the bounded retry loop shared by every UnitOfWork adapter.
package txretry

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

// Policy bounds the retry loop.
type Policy struct {
	MaxAttempts int           // total attempts including the first; default 5
	BaseDelay   time.Duration // default 2ms; doubled each attempt, full jitter
	MaxDelay    time.Duration // default 100ms
	// OnRetry is called before each retry (for metrics / tests).
	OnRetry func(attempt int, err error)
}

func (p Policy) withDefaults() Policy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 5
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 2 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 100 * time.Millisecond
	}
	return p
}

// Run calls once until it succeeds, returns a non-retryable error, the context
// ends, or MaxAttempts is reached. An error is retryable when errors.Is(err,
// store.ErrRetryable); adapters are responsible for wrapping driver errors.
func Run(ctx context.Context, p Policy, once func() error) error {
	p = p.withDefaults()
	delay := p.BaseDelay
	for attempt := 1; ; attempt++ {
		err := once()
		if err == nil || !errors.Is(err, store.ErrRetryable) || attempt >= p.MaxAttempts {
			return err
		}
		if p.OnRetry != nil {
			p.OnRetry(attempt, err)
		}
		sleep := time.Duration(rand.Int64N(int64(delay)) + 1)
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), err)
		case <-time.After(sleep):
		}
		delay = min(delay*2, p.MaxDelay)
	}
}

// retryableError wraps a driver error so that errors.Is(err, store.ErrRetryable)
// holds while errors.As still reaches the driver error.
type retryableError struct{ err error }

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() []error {
	return []error{e.err, store.ErrRetryable}
}

// Mark wraps err as retryable when isRetryable(err) is true.
func Mark(err error, isRetryable func(error) bool) error {
	if err == nil || errors.Is(err, store.ErrRetryable) || !isRetryable(err) {
		return err
	}
	return retryableError{err}
}
