// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// openStream opens GET /streaming with token on a real HTTP server, so that the response
// streams, and returns it with a reader of its lines. Canceling ctx disconnects.
func openStream(ctx context.Context, t *testing.T, srv *httptest.Server, token string) (*http.Request, *http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+clientapi.BasePath+"/streaming", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /streaming: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /streaming = %d, want 200", res.StatusCode)
	}
	return req, res, bufio.NewReader(res.Body)
}

// readLine returns the next non-empty line of a stream, failing t after a second.
func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if line = strings.TrimSuffix(line, "\n"); line != "" || err != nil {
				ch <- result{line, err}
				return
			}
		}
	}()
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("read the stream: %v", res.err)
		}
		return res.line
	case <-time.After(time.Second):
		t.Fatal("no line on the stream within a second")
		return ""
	}
}

// waitEnd fails t unless the stream ends within a second.
func waitEnd(t *testing.T, r *bufio.Reader) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, r)
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the stream did not end within a second")
	}
}

// waitReleased fails t unless h holds no subscription within a second.
func waitReleased(t *testing.T, h *stream.Hub) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for h.Len() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d subscriptions still open", h.Len())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStreamingHeartbeatAndTokenCheck(t *testing.T) {
	hub := stream.NewHub(stream.HubOptions{})
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Streams, o.StreamHeartbeat = hub, 20*time.Millisecond
	})
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)

	req, res, r := openStream(t.Context(), t, srv, "read")
	for k, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-store"} {
		if got := res.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if line := readLine(t, r); line != ": connected" {
		t.Errorf("first line = %q, want the connected comment", line)
	}
	if line := readLine(t, r); line != ": heartbeat" {
		t.Errorf("line = %q, want a heartbeat", line)
	}
	if hub.Len() != 1 {
		t.Errorf("open subscriptions = %d, want 1", hub.Len())
	}

	// A heartbeat finds the token revoked and ends the stream.
	e.auth.mu.Lock()
	e.auth.errs["read"] = appidentity.ErrInvalidAccessToken
	e.auth.mu.Unlock()
	waitEnd(t, r)
	waitReleased(t, hub)
	e.spec.ValidateResponse(t, req, res.StatusCode, res.Header, nil)
}

func TestStreamingReleasesOnDisconnectAndShutdown(t *testing.T) {
	hub := stream.NewHub(stream.HubOptions{})
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Streams = hub })
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	_, _, r := openStream(ctx, t, srv, "read")
	readLine(t, r)
	cancel()
	waitReleased(t, hub)

	_, _, r = openStream(t.Context(), t, srv, "read")
	readLine(t, r)
	hub.Close()
	waitEnd(t, r)
	waitReleased(t, hub)

	// A closed broker refuses new streams.
	res := e.get(t, "/streaming", map[string]string{"Authorization": "Bearer read"})
	if res.StatusCode != http.StatusServiceUnavailable || res.Header.Get("Retry-After") == "" {
		t.Errorf("after shutdown = %d (Retry-After %q), want 503 with Retry-After", res.StatusCode, res.Header.Get("Retry-After"))
	}
}

func TestStreamingBrokerUnavailable(t *testing.T) {
	broker := stream.NewMemoryBroker()
	broker.SetUnreachable(true)
	relay := stream.NewRelay(broker.Bus(), stream.RelayOptions{ReadyTimeout: 20 * time.Millisecond})
	t.Cleanup(func() { _ = relay.Close(context.Background()) })
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Streams = relay })

	res := e.get(t, "/streaming", map[string]string{"Authorization": "Bearer read"})
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.StatusCode)
	}
	if got := res.Header.Get("Retry-After"); got == "" || got == "0" {
		t.Errorf("Retry-After = %q, want seconds", got)
	}
	if p := decodeProblem(t, res); p.Type != clientapi.ProblemTypeBlank || res.Header.Get("Content-Language") != "ja" {
		t.Errorf("problem = %+v (%s), want about:blank in alice's locale", p, res.Header.Get("Content-Language"))
	}
	if got := relay.Len(); got != 0 {
		t.Errorf("open subscriptions = %d, want 0", got)
	}

	// Once the broker answers, the stream opens.
	broker.SetUnreachable(false)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	if _, _, r := openStream(t.Context(), t, srv, "read"); readLine(t, r) != ": connected" {
		t.Error("the stream did not open on a reachable broker")
	}
}

func TestStreamingLimitsStreamsPerAccount(t *testing.T) {
	hub := stream.NewHub(stream.HubOptions{MaxPerAccount: 1})
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Streams = hub })
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)

	_, _, r := openStream(t.Context(), t, srv, "read")
	readLine(t, r)
	res := e.get(t, "/streaming", map[string]string{"Authorization": "Bearer read"})
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second stream = %d, want 429", res.StatusCode)
	}
	p := decodeProblem(t, res)
	if p.Type != clientapi.ProblemTypeTooManyStreams || p.Detail == nil || res.Header.Get("Content-Language") != "ja" {
		t.Errorf("problem = %+v (%s), want too-many-streams in alice's locale", p, res.Header.Get("Content-Language"))
	}
	// Another account has its own streams.
	if _, _, r := openStream(t.Context(), t, srv, "bob"); readLine(t, r) != ": connected" {
		t.Error("bob's stream did not open")
	}
}

func TestStreamingRemembersRecentEntries(t *testing.T) {
	r := clientapi.NewRecentIDs(3)
	for _, id := range []string{"a", "b", "c", "a"} {
		r.Add(id)
	}
	for _, id := range []string{"a", "b", "c"} {
		if !r.Has(id) {
			t.Errorf("%s forgotten within the capacity", id)
		}
	}
	r.Add("d") // forgets the oldest, a
	r.Add("e") // and b
	for id, want := range map[string]bool{"a": false, "b": false, "c": true, "d": true, "e": true} {
		if got := r.Has(id); got != want {
			t.Errorf("Has(%s) = %v, want %v", id, got, want)
		}
	}
}

func TestStreamingWithoutBroker(t *testing.T) {
	e := newAuthEnv(t, nil)
	res := e.get(t, "/streaming", map[string]string{"Authorization": "Bearer read"})
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", res.StatusCode)
	}
}
