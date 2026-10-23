// SPDX-License-Identifier: AGPL-3.0-only

package supporters

import (
	"context"
	"errors"
	"time"

	"github.com/mycroblog/my/internal/app/account"
	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/internal/domain/supporters"
)

// The events of the subscriptions that the notification module tells the subscribers of
// (ADR-0072 §16).
const (
	// EventSubscriptionTransitioned follows every recorded transition of a subscription.
	EventSubscriptionTransitioned = "supporters.subscription_transitioned"
	// EventPlanRetired follows the retirement of a plan, with the subscriptions it ends.
	EventPlanRetired = "supporters.plan_retired"
)

// SubscriptionTransitioned is a transition of a subscription, recorded in the transaction that
// emits it.
type SubscriptionTransitioned struct {
	SubscriptionID string
	// TransitionID is the transition recorded (supporter_subscription_transitions.id).
	TransitionID string
	UserID       string
	// AccountID is the subscriber's local account; "" when it has none.
	AccountID string
	// From is the state before the transition.
	From   supporters.State
	To     supporters.Transition
	Reason supporters.Reason
	// PlanName is the name of the plan of To when the subscriber is told of the transition
	// (PaymentFailed or Ended); "" otherwise.
	PlanName string
	At       time.Time
}

// EventType implements uow.Event.
func (SubscriptionTransitioned) EventType() string { return EventSubscriptionTransitioned }

// GraceEndsAt is when a subscription that entered past_due loses the role unless a payment
// succeeds; zero without a grace period.
func (e SubscriptionTransitioned) GraceEndsAt() time.Time {
	if e.To.Grace == nil {
		return time.Time{}
	}
	return e.To.Grace.EndsAt
}

// EndReason is why the subscription ended, for Ended (supporters.EndReasonOf); "" otherwise.
func (e SubscriptionTransitioned) EndReason() supporters.EndReason {
	return supporters.EndReasonOf(e.To.State, e.Reason)
}

// PaymentFailed reports that the subscription entered past_due: a payment failed and the grace
// period began (supporter_payment_failed).
func (e SubscriptionTransitioned) PaymentFailed() bool {
	return e.To.State == supporters.StatePastDue && e.From != supporters.StatePastDue
}

// Ended reports that the subscription lost the plan's role by entering lapsed, expired, or
// revoked from a paid state (supporter_subscription_ended). A subscription that already lapsed
// does not end again when it expires, and one ended by the deletion of its account tells nobody.
func (e SubscriptionTransitioned) Ended() bool {
	switch e.To.State {
	case supporters.StateLapsed, supporters.StateExpired, supporters.StateRevoked:
	default:
		return false
	}
	return e.From.Member() && e.Reason != supporters.ReasonAccountClosed
}

// PlanRetired is the retirement of a plan.
type PlanRetired struct {
	PlanID   string
	PlanName string
	// Subscriptions are the subscriptions of the plan that the retirement ends and that had been
	// paid for (every live one but a pending one).
	Subscriptions []RetiredSubscription
	At            time.Time
}

// RetiredSubscription is a subscription that the retirement of its plan ends.
type RetiredSubscription struct {
	SubscriptionID string
	UserID         string
	// AccountID is the subscriber's local account; "" when it has none.
	AccountID string
}

// EventType implements uow.Event.
func (PlanRetired) EventType() string { return EventPlanRetired }

// accountOf returns inside tx the local account of a user; "" without one or without Accounts.
func (s *Service) accountOf(ctx context.Context, tx uow.Tx, userID string) (string, error) {
	if s.d.Accounts == nil {
		return "", nil
	}
	id, err := s.d.Accounts.LocalAccountIDOfUserTx(ctx, tx, userID)
	if errors.Is(err, account.ErrNotFound) {
		return "", nil
	}
	return id, err
}
