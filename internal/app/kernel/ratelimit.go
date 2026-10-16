// SPDX-License-Identifier: AGPL-3.0-only

package kernel

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"time"
)

// ErrRateLimited matches a *RateLimitedError.
var ErrRateLimited = errors.New("rate limited")

// RateLimitedError reports that a rate limit refused an operation (ADR-0019 §5). Callers
// answer 429 Too Many Requests with a Retry-After of RetryAfter.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("rate limited: retry after %s", e.RetryAfter)
}

// Is makes errors.Is(err, ErrRateLimited) hold.
func (e *RateLimitedError) Is(target error) bool { return target == ErrRateLimited }

// Limit is a rate limit: at most Max events per Window for each key.
type Limit struct {
	// Name identifies the limit in the store, for example "login.ip". It must not contain ':'.
	Name   string
	Max    int64
	Window time.Duration
}

// Decision is the outcome of a rate-limit check, with what RateLimit response headers need.
type Decision struct {
	Allowed bool
	// Remaining is how many more events the limit allows now; for Take, after the one it took.
	Remaining int64
	// RetryAfter is how long until the limit allows an event again; 0 when Allowed.
	RetryAfter time.Duration
	// Reset is how long until the current window ends.
	Reset time.Duration
}

// RateLimiter enforces Limits with a sliding window counter on a KeyValueStore: the estimate
// of the events in the last Window is the count of the current fixed window plus the count of
// the previous one weighted by how much of it the sliding window still covers. It needs one
// atomic IncrBy per event and at most two reads per check, so any KeyValueStore adapter serves
// it, including kv/db (ADR-0019 §5).
//
// It is safe for concurrent use. Concurrent checks may let a few events beyond Max through,
// which is acceptable for abuse protection.
type RateLimiter struct {
	kv    KeyValueStore
	clock Clock
}

// RateLimitNamespace is the KeyValueStore namespace of the RateLimiter's counters.
const RateLimitNamespace = "ratelimit"

// NewRateLimiter returns a RateLimiter that keeps its counters in kv.
func NewRateLimiter(kv KeyValueStore, clock Clock) *RateLimiter {
	return &RateLimiter{kv: kv, clock: clock}
}

// window locates now within the fixed windows of lim.
type window struct {
	index   int64
	elapsed time.Duration
}

func (l *RateLimiter) window(lim Limit) window {
	ms := l.clock.Now().UnixMilli()
	w := lim.Window.Milliseconds()
	return window{index: ms / w, elapsed: time.Duration(ms%w) * time.Millisecond}
}

func counterKey(lim Limit, index int64, key string) string {
	return lim.Name + ":" + strconv.FormatInt(index, 10) + ":" + key
}

func (l *RateLimiter) count(ctx context.Context, lim Limit, index int64, key string) (int64, error) {
	v, err := l.kv.Get(ctx, RateLimitNamespace, counterKey(lim, index, key))
	if errors.Is(err, ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(string(v), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("kernel: rate limit counter %s: %w", lim.Name, err)
	}
	return n, nil
}

// Check reports whether lim allows one more event for key, without recording one.
func (l *RateLimiter) Check(ctx context.Context, lim Limit, key string) (Decision, error) {
	w := l.window(lim)
	prev, err := l.count(ctx, lim, w.index-1, key)
	if err != nil {
		return Decision{}, err
	}
	cur, err := l.count(ctx, lim, w.index, key)
	if err != nil {
		return Decision{}, err
	}
	return decide(lim, w, prev, cur, 1), nil
}

// Record records one event for key.
func (l *RateLimiter) Record(ctx context.Context, lim Limit, key string) error {
	_, err := l.incr(ctx, lim, l.window(lim), key)
	return err
}

// Take records one event for key and reports whether lim allows it, counting it.
func (l *RateLimiter) Take(ctx context.Context, lim Limit, key string) (Decision, error) {
	w := l.window(lim)
	prev, err := l.count(ctx, lim, w.index-1, key)
	if err != nil {
		return Decision{}, err
	}
	cur, err := l.incr(ctx, lim, w, key)
	if err != nil {
		return Decision{}, err
	}
	return decide(lim, w, prev, cur, 0), nil
}

// incr adds one to the current window's counter, which must outlive the next window, where it
// is the previous one.
func (l *RateLimiter) incr(ctx context.Context, lim Limit, w window, key string) (int64, error) {
	return l.kv.IncrBy(ctx, RateLimitNamespace, counterKey(lim, w.index, key), 1, 2*lim.Window-w.elapsed)
}

// decide estimates the events in the sliding window from the previous and current window
// counts and allows the event when the estimate plus pending stays at most Max.
func decide(lim Limit, w window, prev, cur, pending int64) Decision {
	weight := float64(lim.Window-w.elapsed) / float64(lim.Window)
	estimate := int64(math.Ceil(float64(prev)*weight)) + cur
	d := Decision{Reset: lim.Window - w.elapsed}
	if estimate+pending <= lim.Max {
		d.Allowed = true
		d.Remaining = lim.Max - estimate
		return d
	}
	d.RetryAfter = retryAfter(lim, w, prev, cur)
	return d
}

// retryAfter returns how long until the estimate drops below Max, so that one more event fits.
func retryAfter(lim Limit, w window, prev, cur int64) time.Duration {
	window := float64(lim.Window)
	if cur < lim.Max {
		// The previous window's share has to shrink: prev*(W-t)/W <= Max-cur-1 for t in this window.
		room := float64(lim.Max - cur - 1)
		t := time.Duration(window - room*window/float64(prev))
		return max(t-w.elapsed, time.Millisecond)
	}
	// Wait for the next window, where this one is the previous: cur*(W-t)/W <= Max-1.
	t := time.Duration(window - float64(lim.Max-1)*window/float64(cur))
	return lim.Window - w.elapsed + max(t, time.Millisecond)
}

// ClientKey returns the rate-limit key of a client IP address: the address itself for IPv4
// and its /64 prefix for IPv6, since one subscriber usually holds a whole /64 (ADR-0019 §5).
// It returns ip unchanged when it is not an IP address.
func ClientKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}
	p, _ := addr.Prefix(64)
	return p.String()
}
