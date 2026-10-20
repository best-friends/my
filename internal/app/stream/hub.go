// SPDX-License-Identifier: AGPL-3.0-only

package stream

import (
	"context"
	"sync"
)

// Defaults of the zero fields of HubOptions.
const (
	DefaultMaxPerAccount = 8
	DefaultMaxTotal      = 1000
	DefaultBuffer        = 64
)

// HubOptions bound a Hub. Zero fields take the defaults above.
type HubOptions struct {
	// MaxPerAccount is the number of subscriptions one account may hold open at once (tabs and
	// devices).
	MaxPerAccount int
	// MaxTotal is the number of subscriptions the process holds open at once.
	MaxTotal int
	// Buffer is the number of signals a subscription holds before its subscriber reads them.
	// A subscription whose buffer is full when a signal arrives ends, so that one slow
	// subscriber never delays the others or the publisher.
	Buffer int
}

// Hub is the in-process broker: it implements Publisher and Subscriber for the subscribers of
// one process. A signal published in another process (another web replica, or a worker that
// processes the inbox) never reaches it. It is safe for concurrent use.
type Hub struct {
	o HubOptions

	mu        sync.Mutex
	subs      map[*subscription]struct{}
	byAccount map[string]map[*subscription]struct{}
	closed    bool
}

var (
	_ Publisher  = (*Hub)(nil)
	_ Subscriber = (*Hub)(nil)
)

// NewHub returns an empty Hub bounded by o.
func NewHub(o HubOptions) *Hub {
	if o.MaxPerAccount <= 0 {
		o.MaxPerAccount = DefaultMaxPerAccount
	}
	if o.MaxTotal <= 0 {
		o.MaxTotal = DefaultMaxTotal
	}
	if o.Buffer <= 0 {
		o.Buffer = DefaultBuffer
	}
	return &Hub{o: o, subs: map[*subscription]struct{}{}, byAccount: map[string]map[*subscription]struct{}{}}
}

// Subscribe implements Subscriber. A Hub is always ready, so it waits for nothing.
func (h *Hub) Subscribe(ctx context.Context, accountID string) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	if len(h.subs) >= h.o.MaxTotal || len(h.byAccount[accountID]) >= h.o.MaxPerAccount {
		return nil, ErrTooManySubscriptions
	}
	s := &subscription{hub: h, accountID: accountID, events: make(chan Event, h.o.Buffer), done: make(chan struct{})}
	h.subs[s] = struct{}{}
	if h.byAccount[accountID] == nil {
		h.byAccount[accountID] = map[*subscription]struct{}{}
	}
	h.byAccount[accountID][s] = struct{}{}
	return s, nil
}

// Publish implements Publisher. A signal of KindNotifications goes to the subscriptions of its
// account, one of KindHomeEntry or KindAnnouncement to every subscription.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	targets := h.subs
	if e.Kind == KindNotifications {
		targets = h.byAccount[e.AccountID]
	}
	for s := range targets {
		select {
		case s.events <- e:
		default:
			h.removeLocked(s) // the subscriber fell behind; it reconnects and reads again
		}
	}
}

// Len returns the number of open subscriptions.
func (h *Hub) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// Close ends every subscription and refuses new ones, for a shutdown: the open streams return,
// so that the HTTP server's graceful shutdown does not wait for them.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for s := range h.subs {
		h.removeLocked(s)
	}
}

// endAll ends every subscription but keeps accepting new ones, for a Relay whose listener
// broke: the open streams may have missed signals, so they end and their clients read again.
func (h *Hub) endAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		h.removeLocked(s)
	}
}

// removeLocked ends s; h.mu is held.
func (h *Hub) removeLocked(s *subscription) {
	if _, ok := h.subs[s]; !ok {
		return
	}
	delete(h.subs, s)
	if acc := h.byAccount[s.accountID]; acc != nil {
		delete(acc, s)
		if len(acc) == 0 {
			delete(h.byAccount, s.accountID)
		}
	}
	close(s.done)
}

type subscription struct {
	hub       *Hub
	accountID string
	events    chan Event
	done      chan struct{}
}

func (s *subscription) Events() <-chan Event  { return s.events }
func (s *subscription) Done() <-chan struct{} { return s.done }

func (s *subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.hub.removeLocked(s)
}
