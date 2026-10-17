// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"testing"
	"time"

	"github.com/best-friends/my/internal/domain/federation"
)

func TestClassifyResponse(t *testing.T) {
	cases := map[int]federation.DeliveryOutcome{
		200: federation.DeliverySucceeded,
		202: federation.DeliverySucceeded,
		204: federation.DeliverySucceeded,
		408: federation.DeliveryRetryableFailure,
		429: federation.DeliveryRetryableFailure,
		500: federation.DeliveryRetryableFailure,
		502: federation.DeliveryRetryableFailure,
		503: federation.DeliveryRetryableFailure,
		504: federation.DeliveryRetryableFailure,
		501: federation.DeliveryPermanentFailure,
		400: federation.DeliveryPermanentFailure,
		401: federation.DeliveryPermanentFailure,
		403: federation.DeliveryPermanentFailure,
		404: federation.DeliveryPermanentFailure,
		410: federation.DeliveryPermanentFailure,
		413: federation.DeliveryPermanentFailure,
		422: federation.DeliveryPermanentFailure,
	}
	for status, want := range cases {
		if got := federation.ClassifyResponse(status); got != want {
			t.Errorf("ClassifyResponse(%d) = %s, want %s", status, got, want)
		}
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"":                              0,
		"120":                           2 * time.Minute,
		" 30 ":                          30 * time.Second,
		"0":                             0,
		"-5":                            0,
		"soon":                          0,
		"999999999999":                  federation.MaxRetryAfter,
		"Sat, 17 Oct 2026 12:05:00 GMT": 5 * time.Minute,
		"Sat, 17 Oct 2026 11:00:00 GMT": 0,
		"Sun, 01 Nov 2026 12:00:00 GMT": federation.MaxRetryAfter,
	}
	for v, want := range cases {
		if got := federation.RetryAfter(v, now); got != want {
			t.Errorf("RetryAfter(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestBreaker(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	var s federation.BreakerState
	for i := 1; i < federation.BreakerThreshold; i++ {
		s = s.Failed(now)
		if !s.Allows(now) {
			t.Fatalf("breaker open after %d failures, want closed until %d", i, federation.BreakerThreshold)
		}
	}
	s = s.Failed(now)
	if s.Allows(now) || s.Allows(now.Add(federation.BreakerCooldown-time.Second)) {
		t.Fatalf("breaker closed after %d failures, want open for %v", federation.BreakerThreshold, federation.BreakerCooldown)
	}
	later := now.Add(federation.BreakerCooldown)
	if !s.Allows(later) {
		t.Fatal("breaker still open after the cooldown")
	}
	// One more failure after the cooldown opens it again at once.
	if s = s.Failed(later); s.Allows(later) {
		t.Fatal("breaker closed after a failure past the cooldown")
	}
	if s = s.Succeeded(); !s.Allows(later) || s.Failures != 0 {
		t.Fatalf("breaker after a success = %+v, want closed", s)
	}
}

func TestInstanceHealthFailureDays(t *testing.T) {
	day0 := time.Date(2026, 10, 1, 23, 0, 0, 0, time.UTC)
	h := federation.NewInstanceHealth("down.example", day0)
	for d := range federation.FailureDaysThreshold - 1 {
		at := day0.Add(time.Duration(d) * 24 * time.Hour)
		var marked bool
		// Several failures on one day count as one day.
		for range 3 {
			if h, marked = h.Failed(at); marked {
				t.Fatalf("marked unavailable on day %d", d+1)
			}
		}
		if h.FailureDays != d+1 || h.LastFailureDay != federation.FailureDay(at) {
			t.Fatalf("after day %d: %+v", d+1, h)
		}
	}
	at := day0.Add(time.Duration(federation.FailureDaysThreshold-1) * 24 * time.Hour)
	h, marked := h.Failed(at)
	if !marked || h.State != federation.InstanceUnavailable || !h.Since.Equal(at) {
		t.Fatalf("after %d days: %+v marked=%v, want unavailable since %v", federation.FailureDaysThreshold, h, marked, at)
	}
	// Further failures keep it unavailable without marking it again.
	if h2, marked := h.Failed(at.Add(24 * time.Hour)); marked || h2.State != federation.InstanceUnavailable || !h2.Since.Equal(at) {
		t.Fatalf("failure while unavailable: %+v marked=%v", h2, marked)
	}

	back := at.Add(time.Hour)
	h, changed, marked := h.Reached(back)
	if !changed || !marked || h.State != federation.InstanceAvailable || h.FailureDays != 0 || h.LastFailureDay != "" || !h.Since.Equal(back) {
		t.Fatalf("Reached = %+v changed=%v marked=%v, want available without failures", h, changed, marked)
	}
	if _, changed, marked := h.Reached(back.Add(time.Hour)); changed || marked {
		t.Fatal("Reached on a healthy server changed it")
	}
}

func TestInstanceHealthSuccessResetsTheRun(t *testing.T) {
	day0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	h := federation.NewInstanceHealth("flaky.example", day0)
	for d := range federation.FailureDaysThreshold - 1 {
		h, _ = h.Failed(day0.Add(time.Duration(d) * 24 * time.Hour))
	}
	h, changed, marked := h.Reached(day0.Add(6 * 24 * time.Hour))
	if !changed || marked || h.FailureDays != 0 || h.State != federation.InstanceAvailable || !h.Since.Equal(day0) {
		t.Fatalf("Reached = %+v changed=%v marked=%v", h, changed, marked)
	}
	// The run starts over: one more failure is one failure day.
	if h, marked = h.Failed(day0.Add(7 * 24 * time.Hour)); marked || h.FailureDays != 1 {
		t.Fatalf("Failed after the reset = %+v marked=%v", h, marked)
	}
}
