// SPDX-License-Identifier: AGPL-3.0-only

package uow

import (
	"context"
	"sync"
)

// Event is a persisted fact that a command writes and synchronous projectors consume.
// EventType names the kind of event, for example "graph.follow_created"; projectors subscribe
// by this name.
type Event interface {
	EventType() string
}

// Projector updates its module's projection tables from one event, inside the transaction of
// the command that emitted it (ADR-0020 rule 7). It reads only the event and its own tables,
// writes only its own projections, and must be safe to repeat, like the rest of the
// transaction function.
type Projector func(ctx context.Context, tx Tx, e Event) error

// CommitHook observes an event after the transaction that emitted it has committed. It runs
// outside any transaction, in the goroutine of the command, before UnitOfWork.Tx returns, so it
// must return quickly and never fail the command: use it to signal in-process listeners (such
// as the streaming endpoint), not for work that must happen (enqueue jobs through the outbox).
type CommitHook func(ctx context.Context, e Event)

// Projectors is a registry of synchronous projectors and commit hooks keyed by event type. The
// zero value is empty and ready to use; a nil *Projectors is an empty registry. It is safe for
// concurrent use.
type Projectors struct {
	mu     sync.RWMutex
	byType map[string][]Projector
	hooks  map[string][]CommitHook
}

// Register subscribes p to events of eventType. Projectors of one event type run in the order
// they were registered.
func (r *Projectors) Register(eventType string, p Projector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byType == nil {
		r.byType = make(map[string][]Projector)
	}
	r.byType[eventType] = append(r.byType[eventType], p)
}

// Project runs the projectors registered for e.EventType() with tx, in registration order,
// and stops at the first error. Adapters implement Tx.Emit with it.
func (r *Projectors) Project(ctx context.Context, tx Tx, e Event) error {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	ps := r.byType[e.EventType()]
	r.mu.RUnlock()
	for _, p := range ps {
		if err := p(ctx, tx, e); err != nil {
			return err
		}
	}
	return nil
}

// OnCommit subscribes h to events of eventType that committed transactions emitted. Hooks of
// one event type run in the order they were registered.
func (r *Projectors) OnCommit(eventType string, h CommitHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hooks == nil {
		r.hooks = make(map[string][]CommitHook)
	}
	r.hooks[eventType] = append(r.hooks[eventType], h)
}

// Committed runs the commit hooks of events, in order, once the transaction that emitted them
// through Tx.Emit has committed. Adapters call it after a successful commit with the events of
// that attempt only, so that a rolled-back or retried attempt signals nothing.
func (r *Projectors) Committed(ctx context.Context, events []Event) {
	if r == nil {
		return
	}
	for _, e := range events {
		r.mu.RLock()
		hs := r.hooks[e.EventType()]
		r.mu.RUnlock()
		for _, h := range hs {
			h(ctx, e)
		}
	}
}
