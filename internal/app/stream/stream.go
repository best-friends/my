// SPDX-License-Identifier: AGPL-3.0-only

// Package stream defines the ports of the real-time updates that the streaming endpoint serves
// (FR-207, ADR-0016): modules publish signals after their transactions commit, and each open
// stream subscribes to the signals of its account. It also holds Hub, the in-process broker
// that implements both ports for one process.
//
// A signal names what changed, not what a viewer sees: the subscriber reads the change back
// through the use cases of the module that owns it, which apply the visibility policy for that
// viewer. Delivery is best effort: a signal published while no stream is open, or to a stream
// that does not keep up, is lost, and clients catch up by reading the timelines and
// notifications again when they reconnect.
package stream

import "errors"

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

// Publisher publishes signals. Publish never blocks on subscribers and never fails the caller,
// so commit hooks can call it (uow.CommitHook).
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
)

// Subscriber opens subscriptions.
type Subscriber interface {
	// Subscribe opens a subscription to the signals of accountID: its notifications and every
	// home timeline entry. Close it when done.
	Subscribe(accountID string) (Subscription, error)
}

// Subscription is one open subscription.
type Subscription interface {
	// Events delivers the signals in the order they were published.
	Events() <-chan Event
	// Done is closed when the subscription ends: Close was called, the subscriber fell behind
	// (its buffer filled up), or the broker closed.
	Done() <-chan struct{}
	// Close ends the subscription and releases it. Calling it again does nothing.
	Close()
}
