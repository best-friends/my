// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	localHost   = "my.example"
	remoteHost  = "mastodon.example"
	aliceID     = "https://mastodon.example/users/alice"
	aliceKeyID  = aliceID + "#main-key"
	bobInbox    = "/users/bob/inbox"
	followIDFix = "https://mastodon.example/7a1c3a0e-1b8a-4d3e-9a0f-2f0c5f6c9b11"
)

// fakeNet routes client requests to in-memory handlers by host, turning each
// client request into a server-side request (path-only URL, Host set).
type fakeNet map[string]http.Handler

func (f fakeNet) RoundTrip(req *http.Request) (*http.Response, error) {
	h, ok := f[req.URL.Host]
	if !ok {
		return nil, fmt.Errorf("fakeNet: unknown host %s", req.URL.Host)
	}
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	sreq := httptest.NewRequest(req.Method, req.URL.RequestURI(), bytes.NewReader(body))
	sreq.Host = req.URL.Host
	sreq.Header = req.Header.Clone()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, sreq)
	return rec.Result(), nil
}

// remote is a fake Mastodon: serves alice's actor and records inbox POSTs
// after verifying their signatures with the same Verifier.
type remote struct {
	key      *rsa.PrivateKey
	actor    []byte
	verifier *Verifier

	// rejectRFC9421 simulates Mastodon < 4.5, which only understands draft-cavage.
	rejectRFC9421 bool

	mu       sync.Mutex
	attempts []string // scheme of each inbox POST attempt
	received [][]byte
	schemes  []string // scheme of each accepted delivery
}

func (rm *remote) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/users/alice":
		w.Header().Set("Content-Type", contentTypeActivity)
		_, _ = w.Write(rm.actor)
	case r.Method == http.MethodPost && r.URL.Path == "/users/alice/inbox":
		body, _ := io.ReadAll(r.Body)
		scheme := SchemeCavage
		if r.Header.Get("Signature-Input") != "" {
			scheme = SchemeRFC9421
		}
		rm.mu.Lock()
		rm.attempts = append(rm.attempts, scheme)
		rm.mu.Unlock()
		if rm.rejectRFC9421 && scheme == SchemeRFC9421 {
			http.Error(w, "Verification failed", http.StatusUnauthorized)
			return
		}
		v, err := rm.verifier.Verify(r, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		rm.mu.Lock()
		rm.received = append(rm.received, body)
		rm.schemes = append(rm.schemes, v.Scheme)
		rm.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	default:
		http.NotFound(w, r)
	}
}

type env struct {
	srv    *Server
	h      http.Handler
	remote *remote
	net    fakeNet
}

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func pemOf(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSpace(b)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	net := fakeNet{}
	client := &http.Client{Transport: net}

	aliceKey := mustKey(t)
	quoted, _ := json.Marshal(pemOf(t, aliceKey))
	actor := bytes.Replace(readFixture(t, "mastodon_actor.json"), []byte(`"__PUBLIC_KEY_PEM__"`), quoted, 1)

	rm := &remote{key: aliceKey, actor: actor}
	remoteSigner := &SigningClient{HTTP: client, KeyID: aliceKeyID, Key: aliceKey}
	rm.verifier = &Verifier{
		Resolver:   &HTTPKeyResolver{Client: remoteSigner},
		PublicBase: &url.URL{Scheme: "https", Host: remoteHost},
	}

	srv := NewServer(&url.URL{Scheme: "https", Host: localHost}, "bob", mustKey(t), client,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv.Async = func(f func()) { f() } // deliver inline so tests can assert
	h := srv.Handler()

	net[localHost] = h
	net[remoteHost] = rm
	return &env{srv: srv, h: h, remote: rm, net: net}
}

// signLikeMastodonCavage reproduces Mastodon's outgoing draft-cavage signature
// by hand (independent of the library under test).
func signLikeMastodonCavage(t *testing.T, req *http.Request, body []byte, key *rsa.PrivateKey, keyID string, date time.Time) {
	t.Helper()
	sum := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
	req.Header.Set("Date", date.UTC().Format(http.TimeFormat))
	req.Header.Set("Digest", digest)
	req.Header.Set("Content-Type", contentTypeActivity)
	req.Header.Set("User-Agent", "http.rb/5.2.0 (Mastodon/4.3.0; +https://mastodon.example/)")
	signed := strings.Join([]string{
		"(request-target): " + strings.ToLower(req.Method) + " " + req.URL.RequestURI(),
		"host: " + req.Host,
		"date: " + req.Header.Get("Date"),
		"digest: " + digest,
		"content-type: " + contentTypeActivity,
	}, "\n")
	h := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Signature", fmt.Sprintf(
		`keyId="%s",algorithm="rsa-sha256",headers="(request-target) host date digest content-type",signature="%s"`,
		keyID, base64.StdEncoding.EncodeToString(sig)))
}

// signRFC9421ByHand builds an RFC 9421 signature base manually (§2.5) to test
// the library against an independent implementation.
func signRFC9421ByHand(t *testing.T, req *http.Request, body []byte, key *rsa.PrivateKey, keyID, targetURI string) {
	t.Helper()
	sum := sha256.Sum256(body)
	cd := "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
	req.Header.Set("Content-Digest", cd)
	req.Header.Set("Content-Type", contentTypeActivity)
	params := fmt.Sprintf(`("@method" "@target-uri" "content-digest");created=%d;keyid="%s"`, time.Now().Unix(), keyID)
	base := strings.Join([]string{
		`"@method": ` + req.Method,
		`"@target-uri": ` + targetURI,
		`"content-digest": ` + cd,
		`"@signature-params": ` + params,
	}, "\n")
	h := sha256.Sum256([]byte(base))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Signature-Input", "sig1="+params)
	req.Header.Set("Signature", "sig1=:"+base64.StdEncoding.EncodeToString(sig)+":")
}

func inboxRequest(body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, bobInbox, bytes.NewReader(body))
	req.Host = localHost
	return req
}

func TestWebFinger(t *testing.T) {
	e := newEnv(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:bob@my.example", nil)
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var jrd struct {
		Subject string
		Links   []struct{ Rel, Type, Href string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jrd); err != nil {
		t.Fatal(err)
	}
	if jrd.Subject != "acct:bob@my.example" || len(jrd.Links) != 1 || jrd.Links[0].Href != "https://my.example/users/bob" {
		t.Fatalf("unexpected JRD: %+v", jrd)
	}

	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:eve@my.example", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user status = %d", rec.Code)
	}
}

func TestActorDocument(t *testing.T) {
	e := newEnv(t)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/bob", nil))
	if ct := rec.Header().Get("Content-Type"); ct != contentTypeActivity {
		t.Fatalf("content-type = %q", ct)
	}
	// Our own resolver must accept our own actor document.
	ra, err := (&HTTPKeyResolver{Client: e.srv.client}).ResolveKey(context.Background(), e.srv.KeyID())
	if err != nil {
		t.Fatal(err)
	}
	if !e.srv.key.PublicKey.Equal(ra.PublicKey) || ra.Inbox != "https://my.example/users/bob/inbox" {
		t.Fatalf("resolved %+v", ra)
	}
}

func assertAccept(t *testing.T, raw []byte) {
	t.Helper()
	var acc struct {
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"object"`
	}
	if err := json.Unmarshal(raw, &acc); err != nil {
		t.Fatal(err)
	}
	if acc.Type != "Accept" || acc.Actor != "https://my.example/users/bob" || acc.Object.ID != followIDFix || acc.Object.Type != "Follow" {
		t.Fatalf("unexpected Accept: %s", raw)
	}
}

func TestInboxMastodonCavageFollowGetsSignedAccept(t *testing.T) {
	e := newEnv(t)
	body := readFixture(t, "mastodon_follow.json")
	req := inboxRequest(body)
	signLikeMastodonCavage(t, req, body, e.remote.key, aliceKeyID, time.Now())

	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if len(e.remote.received) != 1 {
		t.Fatalf("remote received %d deliveries", len(e.remote.received))
	}
	assertAccept(t, e.remote.received[0])
	// Remote accepts RFC 9421, so the first knock succeeds.
	if got := e.remote.schemes[0]; got != SchemeRFC9421 {
		t.Fatalf("delivery scheme = %s", got)
	}
}

func TestInboxRFC9421FollowGetsSignedAccept(t *testing.T) {
	e := newEnv(t)
	body := readFixture(t, "mastodon_follow.json")
	req := inboxRequest(body)
	signRFC9421ByHand(t, req, body, e.remote.key, aliceKeyID, "https://my.example"+bobInbox)

	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if len(e.remote.received) != 1 {
		t.Fatalf("remote received %d deliveries", len(e.remote.received))
	}
	assertAccept(t, e.remote.received[0])
}

func TestDoubleKnockFallsBackToCavageAndRemembers(t *testing.T) {
	e := newEnv(t)
	e.remote.rejectRFC9421 = true
	body := readFixture(t, "mastodon_follow.json")

	for i := range 2 {
		req := inboxRequest(body)
		signLikeMastodonCavage(t, req, body, e.remote.key, aliceKeyID, time.Now())
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("follow %d: status = %d", i, rec.Code)
		}
	}
	want := []string{SchemeRFC9421, SchemeCavage, SchemeCavage}
	if fmt.Sprint(e.remote.attempts) != fmt.Sprint(want) {
		t.Fatalf("attempts = %v, want %v", e.remote.attempts, want)
	}
	if len(e.remote.received) != 2 {
		t.Fatalf("received %d", len(e.remote.received))
	}
	if p := e.srv.client.Preference(remoteHost); p != SchemeCavage {
		t.Fatalf("preference = %q", p)
	}
}

func TestInboxRejections(t *testing.T) {
	follow := readFixture(t, "mastodon_follow.json")
	spoofed := bytes.ReplaceAll(follow, []byte(aliceID), []byte("https://mastodon.example/users/mallory"))

	cases := []struct {
		name string
		mk   func(t *testing.T, e *env) *http.Request
		want int
	}{
		{"unsigned", func(t *testing.T, e *env) *http.Request {
			return inboxRequest(follow)
		}, http.StatusUnauthorized},
		{"cavage tampered body", func(t *testing.T, e *env) *http.Request {
			req := inboxRequest(spoofed)
			signLikeMastodonCavage(t, req, follow, e.remote.key, aliceKeyID, time.Now())
			return req
		}, http.StatusUnauthorized},
		{"cavage stale date", func(t *testing.T, e *env) *http.Request {
			req := inboxRequest(follow)
			signLikeMastodonCavage(t, req, follow, e.remote.key, aliceKeyID, time.Now().Add(-13*time.Hour))
			return req
		}, http.StatusUnauthorized},
		{"cavage wrong key", func(t *testing.T, e *env) *http.Request {
			req := inboxRequest(follow)
			signLikeMastodonCavage(t, req, follow, mustKey(t), aliceKeyID, time.Now())
			return req
		}, http.StatusUnauthorized},
		{"rfc9421 tampered body", func(t *testing.T, e *env) *http.Request {
			req := inboxRequest(spoofed)
			signRFC9421ByHand(t, req, follow, e.remote.key, aliceKeyID, "https://my.example"+bobInbox)
			return req
		}, http.StatusUnauthorized},
		{"rfc9421 wrong target-uri", func(t *testing.T, e *env) *http.Request {
			req := inboxRequest(follow)
			signRFC9421ByHand(t, req, follow, e.remote.key, aliceKeyID, "https://other.example"+bobInbox)
			return req
		}, http.StatusUnauthorized},
		{"actor does not own key", func(t *testing.T, e *env) *http.Request {
			req := inboxRequest(spoofed)
			signLikeMastodonCavage(t, req, spoofed, e.remote.key, aliceKeyID, time.Now())
			return req
		}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			rec := httptest.NewRecorder()
			e.h.ServeHTTP(rec, tc.mk(t, e))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if len(e.remote.received) != 0 {
				t.Fatalf("unexpected delivery")
			}
		})
	}
}

// TestSignVerifyRoundTrip signs with SigningClient and verifies with Verifier
// for both schemes and both GET (no body) and POST.
func TestSignVerifyRoundTrip(t *testing.T) {
	e := newEnv(t)
	var got []string
	verifier := &Verifier{Resolver: &HTTPKeyResolver{Client: e.srv.client}, PublicBase: &url.URL{Scheme: "https", Host: "rt.example"}}
	e.net["rt.example"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		v, err := verifier.Verify(r, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		got = append(got, r.Method+" "+v.Scheme+" "+v.ActorID)
	})
	ctx := context.Background()
	for _, scheme := range []string{SchemeRFC9421, SchemeCavage} {
		for _, m := range []string{http.MethodGet, http.MethodPost} {
			var body []byte
			if m == http.MethodPost {
				body = []byte(`{"type":"Create"}`)
			}
			resp, err := e.srv.client.attempt(ctx, m, "https://rt.example/x?y=1", body, contentTypeActivity, scheme)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s %s: %d %s", scheme, m, resp.StatusCode, b)
			}
		}
	}
	want := []string{
		"GET rfc9421 https://my.example/users/bob",
		"POST rfc9421 https://my.example/users/bob",
		"GET draft-cavage-12 https://my.example/users/bob",
		"POST draft-cavage-12 https://my.example/users/bob",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v", got)
	}
}
