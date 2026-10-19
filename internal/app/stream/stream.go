// SPDX-License-Identifier: AGPL-3.0-only

// Package stream defines the ports of the real-time updates that the streaming endpoint serves
// (FR-207, ADR-0016): modules publish signals after their transactions commit, and each open
// stream subscribes to the signals of its account. It also holds Hub, the in-process broker
// that implements both ports for one process, and Relay, which implements them for every
// process on a Bus, the port of the cross-process brokers (ADR-0050).
//
// A signal names what changed, not what a viewer sees: the subscriber reads the change back
// through the use cases of the module that owns it, which apply the visibility policy for that
// viewer. Delivery is best effort: a signal published while no stream is open, or to a stream
// that does not keep up, is lost, and clients catch up by reading the timelines and
// notifications again when they reconnect.
package stream

import (
	"context"
	"errors"
)

// Kind is the kind of a signal.
type Kind string

const (
	// KindHomeEntry signals a new entry of the home timelines (a post or a boost). It goes to
	// every subscriber, which decides whether its own home timeline lists the entry.
	KindHomeEntry Kind = "home_entry"
	// KindNotifications signals that an account may have new notifications.
	KindNotifications Kind = "notifications"
)

// Event is one signal.
type Event struct {
	Kind Kind
	// AccountID is the account whose notifications changed (KindNotifications).
	AccountID string
	// EntryID is the ID of the new timeline entry (KindHomeEntry): a status ID for a post, a
	// boost ID for a boost.
	EntryID string
}

// Publisher publishes signals. Publish never blocks on subscribers or brokers and never fails
// the caller, so commit hooks can call it (uow.CommitHook).
type Publisher interface {
	Publish(e Event)
}

// Errors of Subscriber.Subscribe.
var (
	// ErrTooManySubscriptions reports that the account, or the process, has as many open
	// subscriptions as it allows.
	ErrTooManySubscriptions = errors.New("stream: too many open subscriptions")
	// ErrClosed reports a broker that is shutting down.
	ErrClosed = errors.New("stream: the broker is closed")
	// ErrUnavailable reports a broker that cannot receive the signals of other processes right
	// now: its listener failed to start or was not ready in time. A later attempt may succeed.
	ErrUnavailable = errors.New("stream: the broker is unavailable")
)

// Subscriber opens subscriptions.
type Subscriber interface {
	// Subscribe opens a subscription to the signals of accountID: its notifications and every
	// home timeline entry. Close it when done. It may wait until the broker is ready to receive
	// the signals of every process, for as long as ctx allows and at most a bound of the
	// broker's own; a broker that is not ready by then reports ErrUnavailable.
	Subscribe(ctx context.Context, accountID string) (Subscription, error)
}

// Subscription is one open subscription.
type Subscription interface {
	// Events delivers the signals in the order they were published in each process; signals
	// of different processes may interleave in any order, and a cross-process broker may
	// deliver one twice.
	Events() <-chan Event
	// Done is closed when the subscription ends: Close was called, the subscriber fell behind
	// (its buffer filled up), the broker may have missed signals (the listener of a Relay
	// broke), or the broker closed.
	Done() <-chan struct{}
	// Close ends the subscription and releases it. Calling it again does nothing.
	Close()
}
