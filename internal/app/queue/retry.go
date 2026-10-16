// SPDX-License-Identifier: AGPL-3.0-only

package queue

import (
	"math"
	"time"
)

// Defaults applied to the zero fields of a RetryPolicy.
const (
	DefaultMaxAttempts = 10
	DefaultBaseDelay   = 30 * time.Second
	DefaultMaxDelay    = 24 * time.Hour
	DefaultJitter      = 0.2
)

// RetryPolicy decides how often and how late a failing job runs again (ADR-0004): exponential
// backoff with jitter, up to MaxAttempts runs, after which the job becomes a dead job.
// Zero fields take the defaults above.
type RetryPolicy struct {
	// MaxAttempts is the total number of runs, including the first.
	MaxAttempts int
	// BaseDelay is the backoff after the first failed run. It doubles after every further
	// failed run, up to MaxDelay.
	BaseDelay time.Duration
	MaxDelay  time.Duration
	// Jitter is the fraction of the backoff that is random: each wait is drawn uniformly from
	// [d*(1-Jitter), d], where d is the exponential backoff. Zero takes DefaultJitter; a
	// negative value disables jitter. Values above 1 count as 1.
	Jitter float64
}

// Or returns p with its zero fields taken from fallback.
func (p RetryPolicy) Or(fallback RetryPolicy) RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = fallback.MaxAttempts
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = fallback.BaseDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = fallback.MaxDelay
	}
	if p.Jitter == 0 {
		p.Jitter = fallback.Jitter
	}
	return p
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	return p.Or(RetryPolicy{
		MaxAttempts: DefaultMaxAttempts,
		BaseDelay:   DefaultBaseDelay,
		MaxDelay:    DefaultMaxDelay,
		Jitter:      DefaultJitter,
	})
}

// Backoff returns the wait after the failed run number attempt (from 1). random returns a
// number in [0, 1); pass a fixed function in tests for deterministic delays.
func (p RetryPolicy) Backoff(attempt int, random func() float64) time.Duration {
	p = p.withDefaults()
	d := p.MaxDelay
	if attempt < 1 {
		attempt = 1
	}
	// BaseDelay * 2^(attempt-1), computed in floating point to avoid overflow, capped at MaxDelay.
	if f := float64(p.BaseDelay) * math.Exp2(float64(attempt-1)); f < float64(p.MaxDelay) {
		d = time.Duration(f)
	}
	if j := min(p.Jitter, 1); j > 0 {
		d -= time.Duration(j * random() * float64(d))
	}
	return max(d, 0)
}

// NextVisible returns when a job that should run at runAt becomes visible again in a queue
// whose delays are limited to maxDelay (0 for unlimited). A runner that receives a job before
// its runAt re-enqueues it for the returned time: the re-enqueue chain that lets backoffs
// exceed a service's delay limit (ADR-0004).
func NextVisible(now, runAt time.Time, maxDelay time.Duration) time.Time {
	if maxDelay > 0 && runAt.Sub(now) > maxDelay {
		return now.Add(maxDelay)
	}
	return runAt
}
