// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"net/http"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/domain/federation"
)

// Label keys of the federation metrics (ADR-0055 §3). No metric is labeled with a remote
// domain: the state of each server is in instance_health and the admin screens (§6).
const (
	LabelOutcome      = "my.outcome"
	LabelActivityType = "activity.type"
)

// The outcomes of a delivery attempt, the my.outcome label of the delivery metrics.
const (
	// DeliveryOutcomeSuccess is a 2xx answer.
	DeliveryOutcomeSuccess = "success"
	// DeliveryOutcomeRetry is a failure that the job retries, including an open circuit breaker.
	DeliveryOutcomeRetry = "retry"
	// DeliveryOutcomeGone is an inbox that is not there to take the activity: a 410 Gone, or a
	// server that is unavailable or suspended, to which nothing was sent.
	DeliveryOutcomeGone = "gone"
	// DeliveryOutcomeFailed is any other permanent failure.
	DeliveryOutcomeFailed = "failed"
)

// The outcomes of a POST to an inbox, the my.outcome label of my.inbox.activities.
const (
	// InboxOutcomeAccepted is a new activity, recorded and queued.
	InboxOutcomeAccepted = "accepted"
	// InboxOutcomeDuplicate is an activity that was already recorded.
	InboxOutcomeDuplicate = "duplicate"
	// InboxOutcomeRejected is a request that the inbox refused (the signature, a domain block,
	// a rate limit, or an invalid activity).
	InboxOutcomeRejected = "rejected"
	// InboxOutcomeIgnored is an unknown actor's Delete or Update of itself, dropped unverified.
	InboxOutcomeIgnored = "ignored"
	// InboxOutcomeError is an unexpected failure (a 500).
	InboxOutcomeError = "error"
)

// ActivityTypeOther is the activity.type of a request whose type is unknown or was not read.
const ActivityTypeOther = "other"

// The federation metrics of ADR-0055 §3.
var (
	DeliveryAttempts = kernel.Instrument{
		Name: "my.delivery.attempts", Unit: "{attempt}", Description: "Delivery attempts of outbound activities, by outcome.",
		LabelKeys: []string{LabelOutcome},
	}
	DeliveryDuration = kernel.Instrument{
		Name: "my.delivery.duration", Unit: "s", Description: "Duration of the HTTP request of a delivery attempt, by outcome.",
		LabelKeys: []string{LabelOutcome}, Buckets: kernel.DurationBuckets,
	}
	InboxActivities = kernel.Instrument{
		Name: "my.inbox.activities", Unit: "{activity}", Description: "Activities POSTed to the inboxes, by type and outcome.",
		LabelKeys: []string{LabelActivityType, LabelOutcome},
	}
)

// deliveryOutcome returns the my.outcome of a delivery attempt.
func deliveryOutcome(a federation.DeliveryAttempt) string {
	switch a.Outcome {
	case federation.DeliverySucceeded:
		return DeliveryOutcomeSuccess
	case federation.DeliveryRetryableFailure:
		return DeliveryOutcomeRetry
	case federation.DeliverySkipped:
		return DeliveryOutcomeGone
	}
	if a.HTTPStatus == http.StatusGone {
		return DeliveryOutcomeGone
	}
	return DeliveryOutcomeFailed
}

// activityTypeLabel returns the activity.type of a received document's type: a known activity
// type, or ActivityTypeOther.
func activityTypeLabel(t string) string {
	if federation.KnownActivityType(t) {
		return t
	}
	return ActivityTypeOther
}
