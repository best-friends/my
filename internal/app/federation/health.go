// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// Event types of the health of remote servers (federation.md §7.3).
const (
	// EventDeliveryAttempted is emitted with every delivery_attempts row.
	EventDeliveryAttempted = "federation.delivery_attempted"
	// EventInstanceReached is emitted when a server shows that it is up outside a delivery: a
	// signed request from it, or a successful probe.
	EventInstanceReached = "federation.instance_reached"
)

// DeliveryAttempted is the event of one recorded delivery attempt to the server Domain.
type DeliveryAttempted struct {
	Attempt federation.DeliveryAttempt
	Domain  string
}

// EventType implements uow.Event.
func (DeliveryAttempted) EventType() string { return EventDeliveryAttempted }

// InstanceReached is the event of a server that answered at At, for Reason.
type InstanceReached struct {
	Domain string
	Reason federation.AvailabilityReason
	At     time.Time
}

// EventType implements uow.Event.
func (InstanceReached) EventType() string { return EventInstanceReached }

// HealthProjectorDeps are the ports of the instance_health projector.
type HealthProjectorDeps struct {
	Repository Repository
	IDs        kernel.IDGenerator
}

// RegisterHealthProjectors subscribes the instance_health projector to the delivery attempts and
// the servers reached otherwise (data-model §4.8, federation.md §7.3). In the transaction that
// records the event, it:
//
//   - counts a retryable failure on a new UTC day as one more failure day, and marks the server
//     unavailable (instance_availability_events, failure_days) on the FailureDaysThreshold-th;
//   - on a successful delivery, a signed request from the server, or a successful probe, clears
//     the failure days and marks an unavailable server available again with that reason. A
//     server without failures is not written (the projection is thinned);
//   - ignores permanent failures and skipped attempts: the server answered, or nothing was sent.
func RegisterHealthProjectors(r *uow.Projectors, d HealthProjectorDeps) {
	p := healthProjector{d: d}
	r.Register(EventDeliveryAttempted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(DeliveryAttempted)
		if !ok {
			return nil
		}
		switch ev.Attempt.Outcome {
		case federation.DeliveryRetryableFailure:
			return p.failed(ctx, tx, ev.Domain, ev.Attempt.AttemptedAt)
		case federation.DeliverySucceeded:
			return p.reached(ctx, tx, ev.Domain, federation.ReasonDeliverySucceeded, ev.Attempt.AttemptedAt)
		}
		return nil
	})
	r.Register(EventInstanceReached, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(InstanceReached)
		if !ok {
			return nil
		}
		return p.reached(ctx, tx, ev.Domain, ev.Reason, ev.At)
	})
}

type healthProjector struct{ d HealthProjectorDeps }

// current returns the health of domain in tx, or a new healthy row when it has none.
func (p healthProjector) current(ctx context.Context, tx uow.Tx, domain string, at time.Time) (federation.InstanceHealth, bool, error) {
	h, err := p.d.Repository.LockInstanceHealth(ctx, tx, domain)
	if errors.Is(err, ErrNotFound) {
		return federation.NewInstanceHealth(domain, at), false, nil
	}
	return h, err == nil, err
}

func (p healthProjector) failed(ctx context.Context, tx uow.Tx, domain string, at time.Time) error {
	h, _, err := p.current(ctx, tx, domain, at)
	if err != nil {
		return err
	}
	next, marked := h.Failed(at)
	if next == h {
		return nil
	}
	if err := p.d.Repository.UpsertInstanceHealth(ctx, tx, next); err != nil {
		return err
	}
	if !marked {
		return nil
	}
	return p.mark(ctx, tx, domain, federation.MarkedUnavailable, federation.ReasonFailureDays, at)
}

func (p healthProjector) reached(ctx context.Context, tx uow.Tx, domain string, reason federation.AvailabilityReason, at time.Time) error {
	h, found, err := p.current(ctx, tx, domain, at)
	if err != nil || !found {
		return err
	}
	next, changed, marked := h.Reached(at)
	if !changed {
		return nil
	}
	if err := p.d.Repository.UpsertInstanceHealth(ctx, tx, next); err != nil {
		return err
	}
	if !marked {
		return nil
	}
	return p.mark(ctx, tx, domain, federation.MarkedAvailable, reason, at)
}

// mark records an availability event. Its ID is generated in the transaction: a retried attempt
// writes a new one, and nothing of the rolled-back attempt remains.
func (p healthProjector) mark(ctx context.Context, tx uow.Tx, domain string, kind federation.AvailabilityKind, reason federation.AvailabilityReason, at time.Time) error {
	err := p.d.Repository.InsertAvailabilityEvent(ctx, tx, federation.AvailabilityEvent{
		ID: p.d.IDs.NewID(), Domain: domain, Kind: kind, Reason: reason, OccurredAt: at,
	})
	if err != nil {
		return fmt.Errorf("mark %s %s: %w", domain, kind, err)
	}
	return nil
}

// HealthDeps are the ports of Health.
type HealthDeps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	Clock      kernel.Clock
}

// Health records that remote servers are up outside deliveries (federation.md §7.3): the inbox
// calls Reached with ReasonInboundRequest once a signature from a server verifies (§3.1 step
// 14), and the probe with ReasonProbeSucceeded.
type Health struct{ d HealthDeps }

// NewHealth returns the Health on d.
func NewHealth(d HealthDeps) *Health { return &Health{d: d} }

// Reached records that domain answered, for reason. It reads the committed health first and
// writes nothing for a server without failures, so that the common case costs one read.
func (h *Health) Reached(ctx context.Context, domain string, reason federation.AvailabilityReason) error {
	if !reason.Valid() {
		return fmt.Errorf("federation: availability reason %q", reason)
	}
	now := kernel.Normalize(h.d.Clock.Now())
	cur, err := h.d.Repository.FindInstanceHealth(ctx, domain)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, changed, _ := cur.Reached(now); !changed {
		return nil
	}
	return h.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return tx.Emit(ctx, InstanceReached{Domain: domain, Reason: reason, At: now})
	})
}
