// SPDX-License-Identifier: AGPL-3.0-only

package stream_test

import (
	"errors"
	"testing"

	"github.com/best-friends/my/internal/app/stream"
)

func subscribe(t *testing.T, h *stream.Hub, accountID string) stream.Subscription {
	t.Helper()
	s, err := h.Subscribe(accountID)
	if err != nil {
		t.Fatalf("Subscribe(%q): %v", accountID, err)
	}
	t.Cleanup(s.Close)
	return s
}

func receive(t *testing.T, s stream.Subscription) (stream.Event, bool) {
	t.Helper()
	select {
	case e := <-s.Events():
		return e, true
	default:
		return stream.Event{}, false
	}
}

func isDone(s stream.Subscription) bool {
	select {
	case <-s.Done():
		return true
	default:
		return false
	}
}

func TestHubRoutesSignals(t *testing.T) {
	h := stream.NewHub(stream.HubOptions{})
	alice, bob := subscribe(t, h, "alice"), subscribe(t, h, "bob")

	note := stream.Event{Kind: stream.KindNotifications, AccountID: "alice"}
	h.Publish(note)
	if e, ok := receive(t, alice); !ok || e != note {
		t.Errorf("alice received %+v, %v; want %+v", e, ok, note)
	}
	if e, ok := receive(t, bob); ok {
		t.Errorf("bob received alice's signal %+v", e)
	}

	entry := stream.Event{Kind: stream.KindHomeEntry, EntryID: "s1"}
	h.Publish(entry)
	for name, s := range map[string]stream.Subscription{"alice": alice, "bob": bob} {
		if e, ok := receive(t, s); !ok || e != entry {
			t.Errorf("%s received %+v, %v; want %+v", name, e, ok, entry)
		}
	}
}

func TestHubLimitsSubscriptions(t *testing.T) {
	h := stream.NewHub(stream.HubOptions{MaxPerAccount: 2, MaxTotal: 3})
	first := subscribe(t, h, "alice")
	subscribe(t, h, "alice")
	if _, err := h.Subscribe("alice"); !errors.Is(err, stream.ErrTooManySubscriptions) {
		t.Errorf("third subscription of alice: %v, want ErrTooManySubscriptions", err)
	}
	subscribe(t, h, "bob")
	if _, err := h.Subscribe("carol"); !errors.Is(err, stream.ErrTooManySubscriptions) {
		t.Errorf("fourth subscription: %v, want ErrTooManySubscriptions", err)
	}

	// Closing releases the slot, once.
	first.Close()
	first.Close()
	if !isDone(first) {
		t.Error("a closed subscription is not done")
	}
	if got := h.Len(); got != 2 {
		t.Errorf("Len after Close = %d, want 2", got)
	}
	subscribe(t, h, "alice")
}

func TestHubDropsSlowSubscribers(t *testing.T) {
	h := stream.NewHub(stream.HubOptions{Buffer: 2})
	slow := subscribe(t, h, "alice")
	for range 3 {
		h.Publish(stream.Event{Kind: stream.KindHomeEntry, EntryID: "s"})
	}
	if !isDone(slow) {
		t.Fatal("a subscription with a full buffer is not done")
	}
	if got := h.Len(); got != 0 {
		t.Errorf("Len = %d, want 0", got)
	}
	// The signals it held stay readable.
	for range 2 {
		if _, ok := receive(t, slow); !ok {
			t.Error("a buffered signal was lost")
		}
	}
}

func TestHubClose(t *testing.T) {
	h := stream.NewHub(stream.HubOptions{})
	s := subscribe(t, h, "alice")
	h.Close()
	if !isDone(s) {
		t.Error("Close left a subscription open")
	}
	if _, err := h.Subscribe("alice"); !errors.Is(err, stream.ErrClosed) {
		t.Errorf("Subscribe after Close: %v, want ErrClosed", err)
	}
	h.Publish(stream.Event{Kind: stream.KindHomeEntry, EntryID: "s"})
	if got := h.Len(); got != 0 {
		t.Errorf("Len = %d, want 0", got)
	}
}
