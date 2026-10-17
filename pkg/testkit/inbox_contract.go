// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appfederation "github.com/best-friends/my/internal/app/federation"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/kernel"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// InboxHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, the federation module's repositories on it, and the account repository that the
// handlers of the own actor's Update and Delete write through.
type InboxHarness struct {
	UnitOfWork uow.UnitOfWork
	Inbox      appfederation.InboxRepository
	Federation appfederation.Repository
	// Graph and Moderation are the repositories of the modules whose inbox handlers
	// InboxProcessingContract runs for real (Block, Undo Block, Flag).
	Graph      appgraph.Repository
	Moderation appmoderation.Repository
	Instances  appfederation.InstanceRepository
	Accounts   appaccount.Repository
	// Statuses is the content module's repository, which the handlers of remote posts write in
	// their cases of the order and duplicate property test (inboxOrderCases).
	Statuses appcontent.Repository
}

// InboxFactory returns a harness on a new, empty database whose UnitOfWork uses opts. The
// factory registers the cleanup of everything it creates with t.Cleanup.
type InboxFactory func(t *testing.T, opts uow.Options) InboxHarness

// InboxContract verifies the inbox repository of a database adapter through the synchronous part
// of the inbox (federation.md §3.1): an activity recorded once with its document and one
// federation.inbox job, the URI synthesized for an activity without an id, trust = refetch for an
// activity whose signer is not its actor, every refusal answered with its status before the
// database or the queue is touched, an unknown actor's own Delete dropped without fetching its
// key, and the signer's server marked available again. It also checks that a tombstone of a
// remote object is recorded once. Signatures, keys, and the other modules are in-memory
// stand-ins.
func InboxContract(t *testing.T, newHarness InboxFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *inboxEnv)
	}{
		{"RecordsAnActivityOnce", inboxRecordsOnce},
		{"SynthesizesTheURIOfAnActivityWithoutID", inboxSynthesizesURI},
		{"MarksActivitiesOfOtherSignersForRefetch", inboxMarksRefetch},
		{"RefusesWithoutTouchingTheDatabaseOrTheQueue", inboxRefuses},
		{"DropsAnUnknownActorsOwnDelete", inboxDropsUnknownSelfDelete},
		{"MarksTheSignersServerAvailable", inboxMarksServerAvailable},
		{"RecordsTombstonesOnce", inboxRecordsTombstonesOnce},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t, newInboxEnv(t, newHarness))
		})
	}
}

const (
	inboxAlice    = "https://remote.example/users/alice"
	inboxAliceKey = inboxAlice + "#main-key"
	inboxRelay    = "https://relay.example/actor"
	inboxRelayKey = inboxRelay + "#main-key"
)

// inboxRSAKey is the public key of every signer; the stand-in Verifier never checks it.
var inboxRSAKey = sync.OnceValue(func() *rsa.PublicKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return &k.PublicKey
})

type inboxEnv struct {
	h        InboxHarness
	clock    *Clock
	kv       *KeyValueStore
	jobs     *inboxJobs
	keys     *inboxKeys
	accounts *inboxAccounts
	domains  inboxDomains
	health   *appfederation.Health
	outbox   queue.Outbox
}

func newInboxEnv(t *testing.T, newHarness InboxFactory) *inboxEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	clock := NewClock(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
	appfederation.RegisterHealthProjectors(projectors, appfederation.HealthProjectorDeps{Repository: h.Federation, IDs: NewIDGenerator(clock)})
	e := &inboxEnv{
		h: h, clock: clock, kv: NewKeyValueStore(clock), jobs: &inboxJobs{seen: map[string]bool{}},
		keys: &inboxKeys{owners: map[string]string{
			inboxAliceKey: inboxAlice, inboxRelayKey: inboxRelay,
			// A key whose owner is on another host than the key speaks for no one.
			"https://evil.example/key": inboxAlice,
		}, unavailable: map[string]bool{}, calls: map[string]int{}},
		accounts: &inboxAccounts{known: map[string]bool{inboxAlice: true}},
		domains:  inboxDomains{"blocked.example": true},
		health:   appfederation.NewHealth(appfederation.HealthDeps{UnitOfWork: h.UnitOfWork, Repository: h.Federation, Clock: clock}),
	}
	e.outbox = queue.AfterCommit(projectors, e.jobs, nil)
	return e
}

// inbox returns the Inbox on the harness with the given rate limits (zero: the defaults).
func (e *inboxEnv) inbox(ipLimit, domainLimit kernel.Limit) *appfederation.Inbox {
	return appfederation.NewInbox(appfederation.InboxDeps{
		UnitOfWork: e.h.UnitOfWork, Repository: e.h.Inbox, Outbox: e.outbox,
		Signatures: appfederation.NewSignatureVerifier(inboxVerifier{}, e.keys, e.kv, e.clock),
		Accounts:   e.accounts, Domains: e.domains, Health: e.health,
		RateLimiter: kernel.NewRateLimiter(e.kv, e.clock), IPLimit: ipLimit, DomainLimit: domainLimit,
		Clock: e.clock,
	})
}

// inboxPost describes one POST to the shared inbox.
type inboxPost struct {
	body []byte
	// signature is the stand-in Signature header: "<keyId> <yes|no>"; "" sends none.
	signature string
	// digest overrides the SHA-256 of the body.
	digest []byte
	ip     string
}

func (e *inboxEnv) request(p inboxPost) appfederation.InboxRequest {
	origin := &url.URL{Scheme: "https", Host: contractHost}
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://internal:8080/ap/inbox", bytes.NewReader(p.body))
	r.Header.Set("Content-Type", "application/activity+json")
	r.Header.Set("Date", e.clock.Now().Format(http.TimeFormat))
	digest := p.digest
	if digest == nil {
		sum := sha256.Sum256(p.body)
		digest = sum[:]
	}
	r.Header.Set("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(digest))
	if p.signature != "" {
		r.Header.Set("Signature", p.signature)
	}
	ip := p.ip
	if ip == "" {
		ip = "192.0.2.1"
	}
	return appfederation.InboxRequest{Request: appfederation.NewSignedRequest(origin, r, p.body), ClientIP: ip}
}

func (e *inboxEnv) receive(t *testing.T, in *appfederation.Inbox, p inboxPost) (appfederation.Received, error) {
	t.Helper()
	return in.Receive(t.Context(), e.request(p))
}

func inboxActivity(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	doc := map[string]any{"@context": "https://www.w3.org/ns/activitystreams"}
	for k, v := range fields {
		if v != nil {
			doc[k] = v
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func inboxCreate(t *testing.T, id string) []byte {
	return inboxActivity(t, map[string]any{
		"id": id, "type": "Create", "actor": inboxAlice,
		"object": map[string]any{"id": inboxAlice + "/notes/1", "type": "Note", "content": "hi"},
	})
}

// inboxVerifier is a Verifier whose Signature header is "<keyId> <yes|no>": it covers the
// components of a draft-cavage POST and declares the Digest header; it verifies when it says yes.
type inboxVerifier struct{}

func (inboxVerifier) Parse(r *appfederation.SignedRequest) (*appfederation.Signature, error) {
	v := r.Header.Get("Signature")
	if v == "" {
		return nil, appfederation.ErrNoSignature
	}
	keyID, _, ok := strings.Cut(v, " ")
	if !ok {
		return nil, appfederation.ErrMalformedSignature
	}
	sig := &appfederation.Signature{
		Scheme: appfederation.SchemeCavage, KeyID: keyID, Algorithm: "rsa-sha256",
		Components: []string{"(request-target)", "host", "date", "digest"},
	}
	if alg, value, ok := strings.Cut(r.Header.Get("Digest"), "="); ok {
		b, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, appfederation.ErrMalformedSignature
		}
		sig.Digests = append(sig.Digests, appfederation.Digest{Algorithm: strings.ToLower(alg), Value: b})
	}
	return sig, nil
}

func (inboxVerifier) Verify(r *appfederation.SignedRequest, _ *appfederation.Signature, _ appfederation.VerifyOptions) error {
	if strings.HasSuffix(r.Header.Get("Signature"), " yes") {
		return nil
	}
	return appfederation.ErrSignatureMismatch
}

// inboxKeys resolves the keys of owners and counts the calls per key ID.
type inboxKeys struct {
	mu          sync.Mutex
	owners      map[string]string
	unavailable map[string]bool
	calls       map[string]int
}

func (k *inboxKeys) ResolvePublicKey(_ context.Context, keyID string, _ bool) (appfederation.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.calls[keyID]++
	if k.unavailable[keyID] {
		return appfederation.PublicKey{}, fmt.Errorf("%w: %s", appfederation.ErrPublicKeyUnavailable, keyID)
	}
	owner, ok := k.owners[keyID]
	if !ok {
		return appfederation.PublicKey{}, fmt.Errorf("%w: %s", appfederation.ErrPublicKeyNotFound, keyID)
	}
	return appfederation.PublicKey{KeyID: keyID, Owner: owner, Key: inboxRSAKey()}, nil
}

func (k *inboxKeys) called(keyID string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.calls[keyID]
}

// inboxAccounts knows the remote actors in known.
type inboxAccounts struct{ known map[string]bool }

func (a *inboxAccounts) FindByURI(_ context.Context, uri string) (appaccount.Actor, error) {
	if !a.known[uri] {
		return appaccount.Actor{}, appaccount.ErrNotFound
	}
	var actor appaccount.Actor
	actor.Account.URI = uri
	return actor, nil
}

type inboxDomains map[string]bool

func (d inboxDomains) FederationSuspended(_ context.Context, domain string) (bool, error) {
	return d[domain], nil
}

// inboxJobs is a queue.JobQueue that keeps the jobs given to it, skipping a job whose idempotency
// key it has seen.
type inboxJobs struct {
	mu   sync.Mutex
	jobs []queue.NewJob
	seen map[string]bool
}

func (q *inboxJobs) Enqueue(_ context.Context, jobs ...queue.NewJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range jobs {
		if j.IdempotencyKey != "" && q.seen[j.IdempotencyKey] {
			continue
		}
		q.seen[j.IdempotencyKey] = true
		q.jobs = append(q.jobs, j)
	}
	return nil
}

func (q *inboxJobs) all() []queue.NewJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]queue.NewJob(nil), q.jobs...)
}

func inboxRecordsOnce(t *testing.T, e *inboxEnv) {
	ctx := t.Context()
	in := e.inbox(kernel.Limit{}, kernel.Limit{})
	id := inboxAlice + "/activities/1"
	body := inboxCreate(t, id)
	got, err := e.receive(t, in, inboxPost{body: body, signature: inboxAliceKey + " yes"})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if !got.Recorded || got.Dropped || got.ActivityURI != federation.IRI(id) || got.Trust != federation.TrustSigned {
		t.Errorf("Receive = %+v, want recorded %s, signed", got, id)
	}
	a, err := e.h.Inbox.FindReceivedActivity(ctx, federation.IRI(id))
	if err != nil {
		t.Fatalf("FindReceivedActivity: %v", err)
	}
	want := federation.ReceivedActivity{
		ActivityURI: federation.IRI(id), ActorURI: inboxAlice, Type: "Create", Scheme: "cavage",
		Trust: federation.TrustSigned, ReceivedAt: e.clock.Now(),
	}
	if !a.ReceivedAt.Equal(want.ReceivedAt) {
		t.Errorf("ReceivedAt = %v, want %v", a.ReceivedAt, want.ReceivedAt)
	}
	a.ReceivedAt = want.ReceivedAt
	if a != want {
		t.Errorf("received activity = %+v, want %+v", a, want)
	}
	doc, err := e.h.Inbox.FindReceivedDocument(ctx, federation.IRI(id))
	if err != nil {
		t.Fatalf("FindReceivedDocument: %v", err)
	}
	if !sameJSON(doc, body) {
		t.Errorf("document = %s, want %s", doc, body)
	}
	jobs := e.jobs.all()
	if len(jobs) != 1 {
		t.Fatalf("jobs = %+v, want one federation.inbox", jobs)
	}
	sum := sha256.Sum256([]byte(id))
	j := jobs[0]
	if j.Type != appfederation.JobInbox || j.IdempotencyKey != "inbox:"+hex.EncodeToString(sum[:]) || j.OrderingKey != "inbox:"+inboxAlice {
		t.Errorf("job = %s %q %q, want %s, inbox:<sha256 of the URI>, inbox:<actor>", j.Type, j.IdempotencyKey, j.OrderingKey, appfederation.JobInbox)
	}
	var payload map[string]any
	if err := json.Unmarshal(j.Payload, &payload); err != nil || payload["activity_uri"] != id || payload["trust"] != "signed" {
		t.Errorf("payload = %s, want activity_uri %s and trust signed", j.Payload, id)
	}

	// The same activity again, as a sender retries it, records nothing and enqueues nothing.
	e.clock.Advance(time.Minute)
	again, err := e.receive(t, in, inboxPost{body: body, signature: inboxAliceKey + " yes"})
	if err != nil {
		t.Fatalf("Receive again: %v", err)
	}
	if again.Recorded || again.ActivityURI != federation.IRI(id) {
		t.Errorf("Receive again = %+v, want not recorded", again)
	}
	if n := len(e.jobs.all()); n != 1 {
		t.Errorf("jobs after the retry = %d, want 1", n)
	}
	if a, err := e.h.Inbox.FindReceivedActivity(ctx, federation.IRI(id)); err != nil || !a.ReceivedAt.Equal(want.ReceivedAt) {
		t.Errorf("received activity after the retry = %+v, %v; want the first", a, err)
	}
}

func inboxSynthesizesURI(t *testing.T, e *inboxEnv) {
	in := e.inbox(kernel.Limit{}, kernel.Limit{})
	body := inboxActivity(t, map[string]any{"type": "Like", "actor": inboxAlice, "object": "https://" + contractHost + "/ap/objects/x"})
	got, err := e.receive(t, in, inboxPost{body: body, signature: inboxAliceKey + " yes"})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	h := sha256.New()
	h.Write([]byte(inboxAlice + "\n"))
	h.Write(body)
	want := federation.IRI("urn:my:digest:" + hex.EncodeToString(h.Sum(nil)))
	if !got.Recorded || got.ActivityURI != want {
		t.Errorf("Receive = %+v, want recorded %s", got, want)
	}
	if _, err := e.h.Inbox.FindReceivedActivity(t.Context(), want); err != nil {
		t.Errorf("FindReceivedActivity(%s): %v", want, err)
	}
	if again, err := e.receive(t, in, inboxPost{body: body, signature: inboxAliceKey + " yes"}); err != nil || again.Recorded {
		t.Errorf("Receive again = %+v, %v; want not recorded", again, err)
	}
	if n := len(e.jobs.all()); n != 1 {
		t.Errorf("jobs = %d, want 1", n)
	}
}

func inboxMarksRefetch(t *testing.T, e *inboxEnv) {
	in := e.inbox(kernel.Limit{}, kernel.Limit{})
	tests := []struct {
		name, id, signature string
	}{
		{"relayed by another actor", inboxAlice + "/activities/relayed", inboxRelayKey + " yes"},
		{"id on another host than the actor", "https://elsewhere.example/activities/1", inboxAliceKey + " yes"},
		{"key owner on another host than the key", inboxAlice + "/activities/evil", "https://evil.example/key yes"},
	}
	for _, tt := range tests {
		got, err := e.receive(t, in, inboxPost{body: inboxCreate(t, tt.id), signature: tt.signature})
		if err != nil {
			t.Fatalf("%s: Receive: %v", tt.name, err)
		}
		if !got.Recorded || got.Trust != federation.TrustRefetch {
			t.Errorf("%s: Receive = %+v, want recorded with trust refetch", tt.name, got)
		}
		a, err := e.h.Inbox.FindReceivedActivity(t.Context(), federation.IRI(tt.id))
		if err != nil || a.Trust != federation.TrustRefetch {
			t.Errorf("%s: received activity = %+v, %v; want trust refetch", tt.name, a, err)
		}
	}
	for _, j := range e.jobs.all() {
		var payload map[string]any
		if err := json.Unmarshal(j.Payload, &payload); err != nil || payload["trust"] != "refetch" {
			t.Errorf("payload = %s, want trust refetch", j.Payload)
		}
	}
}

func inboxRefuses(t *testing.T, e *inboxEnv) {
	e.keys.unavailable["https://down.example/actor#main-key"] = true
	one := kernel.Limit{Name: "test", Max: 1, Window: time.Minute}
	id := inboxAlice + "/activities/refused"
	good := inboxCreate(t, id)
	tests := []struct {
		name        string
		ipLimit     kernel.Limit
		domainLimit kernel.Limit
		// before is sent first, from another activity, to use up a limit.
		before          *inboxPost
		post            inboxPost
		status          int
		acceptSignature bool
	}{
		{name: "no signature", post: inboxPost{body: good}, status: http.StatusUnauthorized, acceptSignature: true},
		{name: "malformed signature", post: inboxPost{body: good, signature: "garbage"}, status: http.StatusBadRequest},
		{name: "keyId without a host", post: inboxPost{body: good, signature: "main-key yes"}, status: http.StatusUnauthorized, acceptSignature: true},
		{name: "suspended domain", post: inboxPost{body: good, signature: "https://blocked.example/actor#main-key yes"}, status: http.StatusForbidden},
		{
			name: "IP rate limit", ipLimit: one,
			before: &inboxPost{body: inboxCreate(t, inboxAlice+"/activities/first"), signature: inboxAliceKey + " yes", ip: "198.51.100.7"},
			post:   inboxPost{body: good, signature: inboxAliceKey + " yes", ip: "198.51.100.7"}, status: http.StatusTooManyRequests,
		},
		{
			name: "domain rate limit", domainLimit: one,
			before: &inboxPost{body: inboxCreate(t, inboxAlice+"/activities/second"), signature: inboxAliceKey + " yes"},
			post:   inboxPost{body: good, signature: inboxAliceKey + " yes"}, status: http.StatusTooManyRequests,
		},
		{name: "digest mismatch", post: inboxPost{body: good, signature: inboxAliceKey + " yes", digest: make([]byte, 32)}, status: http.StatusUnauthorized, acceptSignature: true},
		{name: "not JSON", post: inboxPost{body: []byte("{"), signature: inboxAliceKey + " yes"}, status: http.StatusBadRequest},
		{name: "no actor", post: inboxPost{body: inboxActivity(t, map[string]any{"id": id, "type": "Create"}), signature: inboxAliceKey + " yes"}, status: http.StatusBadRequest},
		{name: "unknown key", post: inboxPost{body: good, signature: "https://remote.example/users/nobody#main-key yes"}, status: http.StatusUnauthorized, acceptSignature: true},
		{name: "key origin unavailable", post: inboxPost{body: good, signature: "https://down.example/actor#main-key yes"}, status: http.StatusServiceUnavailable},
		{name: "signature does not verify", post: inboxPost{body: good, signature: inboxAliceKey + " no"}, status: http.StatusUnauthorized, acceptSignature: true},
	}
	for _, tt := range tests {
		in := e.inbox(tt.ipLimit, tt.domainLimit)
		if tt.before != nil {
			if _, err := e.receive(t, in, *tt.before); err != nil {
				t.Fatalf("%s: first request: %v", tt.name, err)
			}
		}
		jobs := len(e.jobs.all())
		_, err := e.receive(t, in, tt.post)
		var rej *appfederation.InboxRejection
		if !errors.As(err, &rej) {
			t.Errorf("%s: Receive = %v, want an InboxRejection", tt.name, err)
			continue
		}
		if rej.Status != tt.status || (rej.AcceptSignature != "") != tt.acceptSignature {
			t.Errorf("%s: rejection = %d %q, want %d (Accept-Signature %t)", tt.name, rej.Status, rej.AcceptSignature, tt.status, tt.acceptSignature)
		}
		if tt.status == http.StatusTooManyRequests && rej.RetryAfter <= 0 {
			t.Errorf("%s: RetryAfter = %v, want > 0", tt.name, rej.RetryAfter)
		}
		if _, err := e.h.Inbox.FindReceivedActivity(t.Context(), federation.IRI(id)); !errors.Is(err, appfederation.ErrNotFound) {
			t.Errorf("%s: FindReceivedActivity = %v, want ErrNotFound", tt.name, err)
		}
		if n := len(e.jobs.all()); n != jobs {
			t.Errorf("%s: jobs = %d, want %d", tt.name, n, jobs)
		}
		e.clock.Advance(time.Hour) // a fresh window for the next case's limits
	}
}

func inboxDropsUnknownSelfDelete(t *testing.T, e *inboxEnv) {
	in := e.inbox(kernel.Limit{}, kernel.Limit{})
	stranger := "https://remote.example/users/gone"
	e.keys.owners[stranger+"#main-key"] = stranger
	del := inboxActivity(t, map[string]any{"id": stranger + "#delete", "type": "Delete", "actor": stranger, "object": stranger})
	got, err := e.receive(t, in, inboxPost{body: del, signature: stranger + "#main-key yes"})
	if err != nil || !got.Dropped || got.Recorded {
		t.Errorf("Receive(unknown Delete) = %+v, %v; want dropped", got, err)
	}
	if n := e.keys.called(stranger + "#main-key"); n != 0 {
		t.Errorf("key resolved %d times for a dropped Delete, want 0", n)
	}
	if _, err := e.h.Inbox.FindReceivedActivity(t.Context(), federation.IRI(stranger+"#delete")); !errors.Is(err, appfederation.ErrNotFound) {
		t.Errorf("FindReceivedActivity = %v, want ErrNotFound", err)
	}
	// A known actor's Delete of itself is recorded.
	known := inboxActivity(t, map[string]any{"id": inboxAlice + "#delete", "type": "Delete", "actor": inboxAlice, "object": inboxAlice})
	got, err = e.receive(t, in, inboxPost{body: known, signature: inboxAliceKey + " yes"})
	if err != nil || !got.Recorded {
		t.Errorf("Receive(known Delete) = %+v, %v; want recorded", got, err)
	}
	if n := len(e.jobs.all()); n != 1 {
		t.Errorf("jobs = %d, want 1", n)
	}
}

func inboxMarksServerAvailable(t *testing.T, e *inboxEnv) {
	ctx := t.Context()
	err := e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return e.h.Federation.UpsertInstanceHealth(ctx, tx, federation.InstanceHealth{
			Domain: "remote.example", State: federation.InstanceUnavailable, Since: e.clock.Now(), FailureDays: 7, LastFailureDay: "20261017",
		})
	})
	if err != nil {
		t.Fatalf("UpsertInstanceHealth: %v", err)
	}
	in := e.inbox(kernel.Limit{}, kernel.Limit{})
	// A refused request does not show that the server is up.
	if _, err := e.receive(t, in, inboxPost{body: inboxCreate(t, inboxAlice+"/activities/bad"), signature: inboxAliceKey + " no"}); err == nil {
		t.Fatal("Receive with a bad signature succeeded")
	}
	if s, err := e.h.Federation.InstanceState(ctx, "remote.example"); err != nil || s != federation.InstanceUnavailable {
		t.Errorf("InstanceState after a refused request = %q, %v; want unavailable", s, err)
	}
	if _, err := e.receive(t, in, inboxPost{body: inboxCreate(t, inboxAlice+"/activities/up"), signature: inboxAliceKey + " yes"}); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if s, err := e.h.Federation.InstanceState(ctx, "remote.example"); err != nil || s != federation.InstanceAvailable {
		t.Errorf("InstanceState after a signed request = %q, %v; want available", s, err)
	}
	events, err := e.h.Federation.ListAvailabilityEvents(ctx, "remote.example", "", 10)
	if err != nil || len(events) != 1 || events[0].Kind != federation.MarkedAvailable || events[0].Reason != federation.ReasonInboundRequest {
		t.Errorf("availability events = %+v, %v; want one marked_available for inbound_request", events, err)
	}
}

func inboxRecordsTombstonesOnce(t *testing.T, e *inboxEnv) {
	ctx := t.Context()
	note := federation.IRI(inboxAlice + "/notes/gone")
	first := federation.RemoteObjectTombstone{
		URI: note, ActorURI: inboxAlice, ActivityURI: inboxAlice + "/activities/delete-1", ReceivedAt: e.clock.Now(),
	}
	if ok, err := e.h.Inbox.RemoteObjectTombstoned(ctx, note); err != nil || ok {
		t.Fatalf("RemoteObjectTombstoned before = %t, %v; want false", ok, err)
	}
	insert := func(ts federation.RemoteObjectTombstone) bool {
		t.Helper()
		var inserted bool
		err := e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			var err error
			inserted, err = e.h.Inbox.InsertRemoteObjectTombstone(ctx, tx, ts)
			return err
		})
		if err != nil {
			t.Fatalf("InsertRemoteObjectTombstone: %v", err)
		}
		return inserted
	}
	if !insert(first) {
		t.Error("InsertRemoteObjectTombstone = false, want the first tombstone recorded")
	}
	again := first
	again.ActivityURI = inboxAlice + "/activities/delete-2"
	again.ReceivedAt = e.clock.Now().Add(time.Minute)
	if insert(again) {
		t.Error("InsertRemoteObjectTombstone again = true, want the first tombstone to stay")
	}
	if ok, err := e.h.Inbox.RemoteObjectTombstoned(ctx, note); err != nil || !ok {
		t.Errorf("RemoteObjectTombstoned = %t, %v; want true", ok, err)
	}
	if ok, err := e.h.Inbox.RemoteObjectTombstoned(ctx, inboxAlice+"/notes/other"); err != nil || ok {
		t.Errorf("RemoteObjectTombstoned(other) = %t, %v; want false", ok, err)
	}
}
