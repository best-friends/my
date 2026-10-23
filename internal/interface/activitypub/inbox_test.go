// SPDX-License-Identifier: AGPL-3.0-only

package activitypub_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	appfederation "github.com/mycroblog/my/internal/app/federation"
)

// fakeInbox records the requests it receives and answers with err: an *InboxRejection as the
// refusal, any other error as an unexpected failure.
type fakeInbox struct {
	mu       sync.Mutex
	requests []appfederation.InboxRequest
	err      error
}

func (f *fakeInbox) Receive(_ context.Context, in appfederation.InboxRequest) (appfederation.Received, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, in)
	var rej *appfederation.InboxRejection
	if errors.As(f.err, &rej) {
		return appfederation.Received{Rejection: rej}, nil
	}
	if f.err != nil {
		return appfederation.Received{}, f.err
	}
	return appfederation.Received{Recorded: true}, nil
}

func (f *fakeInbox) received() []appfederation.InboxRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]appfederation.InboxRequest(nil), f.requests...)
}

// post serves one POST arriving on an internal address, as behind a reverse proxy.
func (s *server) post(t *testing.T, target, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://internal:8080"+target, bytes.NewReader(body))
	req.RemoteAddr = "198.51.100.7:4321"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return serve(s, req)
}

func TestInbox(t *testing.T) {
	body := []byte(`{"type":"Follow"}`)
	t.Run("accepts every ActivityPub content type", func(t *testing.T) {
		for _, ct := range []string{
			"application/activity+json",
			"application/activity+json; charset=utf-8",
			`application/ld+json; profile="https://www.w3.org/ns/activitystreams"`,
			`application/ld+json; profile="https://www.w3.org/ns/activitystreams#"`,
			"application/json",
		} {
			s := newServer(t, host)
			rec := s.post(t, "/ap/inbox", ct, body)
			if rec.Code != http.StatusAccepted {
				t.Errorf("POST /ap/inbox (%s) = %d %s, want 202", ct, rec.Code, rec.Body)
			}
			got := s.inbox.received()
			if len(got) != 1 {
				t.Fatalf("(%s) Receive called %d times, want 1", ct, len(got))
			}
			r := got[0]
			if r.ClientIP != "198.51.100.7" || !bytes.Equal(r.Request.Body, body) ||
				r.Request.URL.String() != "https://"+host+"/ap/inbox" || r.Request.Header.Get("Host") != host {
				t.Errorf("(%s) request = %+v, want the body on the public origin from 198.51.100.7", ct, r)
			}
		}
	})

	t.Run("actor inboxes of active local actors only", func(t *testing.T) {
		s := newServer(t, host)
		for _, id := range []string{aliceID, instanceID} {
			if rec := s.post(t, "/ap/actors/"+id+"/inbox", "application/activity+json", body); rec.Code != http.StatusAccepted {
				t.Errorf("POST inbox of %s = %d, want 202", id, rec.Code)
			}
		}
		for _, id := range []string{missingID, carolID, daveID, remoteID, "not-a-uuid"} {
			if rec := s.post(t, "/ap/actors/"+id+"/inbox", "application/activity+json", body); rec.Code != http.StatusNotFound {
				t.Errorf("POST inbox of %s = %d, want 404", id, rec.Code)
			}
		}
		if n := len(s.inbox.received()); n != 2 {
			t.Errorf("Receive called %d times, want 2", n)
		}
	})

	t.Run("refuses before the federation module", func(t *testing.T) {
		tests := []struct {
			name, contentType string
			body              []byte
			status            int
		}{
			{"no content type", "", body, http.StatusUnsupportedMediaType},
			{"text", "text/plain", body, http.StatusUnsupportedMediaType},
			{"JSON-LD without the profile", "application/ld+json", body, http.StatusUnsupportedMediaType},
			{"unparsable content type", "application/", body, http.StatusUnsupportedMediaType},
			{"body over 1 MiB", "application/activity+json", bytes.Repeat([]byte(" "), appfederation.InboxMaxBodyBytes+1), http.StatusRequestEntityTooLarge},
		}
		for _, tt := range tests {
			s := newServer(t, host)
			rec := s.post(t, "/ap/inbox", tt.contentType, tt.body)
			if rec.Code != tt.status || rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s: POST = %d (Cache-Control %q), want %d no-store", tt.name, rec.Code, rec.Header().Get("Cache-Control"), tt.status)
			}
			if n := len(s.inbox.received()); n != 0 {
				t.Errorf("%s: Receive called %d times, want 0", tt.name, n)
			}
		}
	})

	t.Run("answers the status of the refusing step", func(t *testing.T) {
		tests := []struct {
			name   string
			err    error
			status int
			header map[string]string
		}{
			{"rate limited", &appfederation.InboxRejection{Status: http.StatusTooManyRequests, RetryAfter: 1500 * time.Millisecond}, http.StatusTooManyRequests, map[string]string{"Retry-After": "2"}},
			{"bad signature", &appfederation.InboxRejection{Status: http.StatusUnauthorized, AcceptSignature: `sig1=("@method");created`}, http.StatusUnauthorized, map[string]string{"Accept-Signature": `sig1=("@method");created`}},
			{"suspended", &appfederation.InboxRejection{Status: http.StatusForbidden}, http.StatusForbidden, nil},
			{"key unavailable", &appfederation.InboxRejection{Status: http.StatusServiceUnavailable}, http.StatusServiceUnavailable, nil},
			{"unexpected", errors.New("database down"), http.StatusInternalServerError, nil},
		}
		for _, tt := range tests {
			s := newServer(t, host)
			s.inbox.err = tt.err
			rec := s.post(t, "/ap/inbox", "application/activity+json", body)
			if rec.Code != tt.status {
				t.Errorf("%s: POST = %d, want %d", tt.name, rec.Code, tt.status)
			}
			for k, v := range tt.header {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s: %s = %q, want %q", tt.name, k, got, v)
				}
			}
		}
	})
}
