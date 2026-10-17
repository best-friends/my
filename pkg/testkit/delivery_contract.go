// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// This file holds the cases of FederationRepositoryContract that cover one delivery
// (federation.md §7.3): the classification of responses, the circuit breaker, the health of
// remote servers and its availability events, the dead job after the retry schedule, and the
// daily probe. The network is an in-memory HTTPClient that routes by host.

// deliveryEnv adds the delivery, the health of servers, and the probe to a fedEnv.
type deliveryEnv struct {
	*fedEnv
	net       *deliveryNetwork
	keys      *deliveryKeys
	breaker   *appfederation.CircuitBreaker
	health    *appfederation.Health
	deliverer *appfederation.Deliverer
	// actor is a local account with a signing key.
	actor string
}

func newDeliveryEnv(t *testing.T, e *fedEnv) *deliveryEnv {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	d := &deliveryEnv{
		fedEnv: e, net: &deliveryNetwork{routes: map[string]deliveryRoute{}, requests: map[string]int{}},
		keys: &deliveryKeys{keys: map[string]appfederation.SigningKey{}}, breaker: appfederation.NewCircuitBreaker(),
	}
	d.actor = e.local()
	d.keys.keys[d.actor] = appfederation.SigningKey{KeyID: e.uri(d.actor) + "#main-key", PrivateKey: priv}
	appfederation.RegisterHealthProjectors(e.projectors, appfederation.HealthProjectorDeps{Repository: e.h.Federation, IDs: e.ids})
	d.health = appfederation.NewHealth(appfederation.HealthDeps{UnitOfWork: e.h.UnitOfWork, Repository: e.h.Federation, Clock: e.clock})
	d.deliverer = appfederation.NewDeliverer(appfederation.DeliverDeps{
		UnitOfWork: e.h.UnitOfWork, Repository: e.h.Federation,
		Client: appfederation.NewSignedHTTPClient(d.net, deliverySigner{}, NewKeyValueStore(e.clock), e.clock),
		Keys:   d.keys, Domains: e.domains, Breaker: d.breaker, Clock: e.clock, IDs: e.ids,
		Logger: slog.New(slog.DiscardHandler),
	})
	d.deliverer.RegisterJobs(e.handlers)
	return d
}

// deliveryRoute answers the requests to one host.
type deliveryRoute func(r *kernel.HTTPRequest) (status int, header http.Header, body string, err error)

// deliveryNetwork is a kernel.HTTPClient that routes requests by host; an unknown host fails
// like an unreachable one.
type deliveryNetwork struct {
	mu       sync.Mutex
	routes   map[string]deliveryRoute
	requests map[string]int
}

func (n *deliveryNetwork) route(host string, r deliveryRoute) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.routes[host] = r
}

func (n *deliveryNetwork) answer(host string, status int) {
	n.route(host, func(*kernel.HTTPRequest) (int, http.Header, string, error) { return status, nil, "", nil })
}

func (n *deliveryNetwork) count(host string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.requests[host]
}

var errUnreachable = errors.New("dial tcp: connection refused")

func (n *deliveryNetwork) Do(_ context.Context, req *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	u, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", kernel.ErrInvalidHTTPRequest, err)
	}
	n.mu.Lock()
	n.requests[u.Host]++
	r, ok := n.routes[u.Host]
	n.mu.Unlock()
	if !ok {
		return nil, errUnreachable
	}
	status, h, body, err := r(req)
	if err != nil {
		return nil, err
	}
	if h == nil {
		h = http.Header{}
	}
	return &kernel.HTTPResponse{StatusCode: status, Header: h, URL: req.URL, Body: io.NopCloser(strings.NewReader(body))}, nil
}

// deliverySigner is a federation.Signer that only names the scheme it signed with.
type deliverySigner struct{}

func (deliverySigner) Sign(req *kernel.HTTPRequest, key appfederation.SigningKey, opts appfederation.SignOptions) error {
	req.Header.Set("Signature", fmt.Sprintf("keyId=%q,scheme=%q", key.KeyID, opts.Scheme))
	return nil
}

func (deliverySigner) ParseAcceptSignature(string) ([]appfederation.RequestedSignature, error) {
	return nil, appfederation.ErrMalformedSignature
}

// deliveryKeys stands in for the account module's signing keys.
type deliveryKeys struct {
	mu   sync.Mutex
	keys map[string]appfederation.SigningKey
}

func (k *deliveryKeys) SigningKey(_ context.Context, accountID string) (appfederation.SigningKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	key, ok := k.keys[accountID]
	if !ok {
		return appfederation.SigningKey{}, fmt.Errorf("signing key of %s: %w", accountID, appaccount.ErrNotFound)
	}
	return key, nil
}

// delivery records an outbound activity of type typ by the env's actor, its document, and one
// delivery of it to https://host/inbox.
func (d *deliveryEnv) delivery(t *testing.T, typ federation.ActivityType, host string) federation.Delivery {
	t.Helper()
	ctx := t.Context()
	act := federation.OutboundActivity{
		ID: d.ids.NewID(), Type: typ, ActorAccountID: d.actor, ObjectKind: federation.OutboundStatus,
		ObjectID: d.ids.NewID(), CreatedAt: d.clock.Now(),
	}
	act.ActivityURI = federation.IRI("https://" + contractHost + "/ap/activities/" + act.ID)
	del := federation.Delivery{
		ID: d.ids.NewID(), OutboundActivityID: act.ID, InboxURL: federation.IRI("https://" + host + "/inbox"),
		Domain: host, CreatedAt: d.clock.Now(),
	}
	err := d.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := d.h.Federation.InsertOutboundActivity(ctx, tx, act); err != nil {
			return err
		}
		doc := []byte(`{"id":"` + string(act.ActivityURI) + `","type":"` + string(typ) + `"}`)
		if _, err := d.h.Federation.InsertOutboundDocument(ctx, tx, act.ID, doc, d.clock.Now()); err != nil {
			return err
		}
		_, err := d.h.Federation.InsertDelivery(ctx, tx, del)
		return err
	})
	if err != nil {
		t.Fatalf("record delivery: %v", err)
	}
	return del
}

// deliver runs the JobDeliver of del once.
func (d *deliveryEnv) deliver(t *testing.T, del federation.Delivery) queue.Result {
	t.Helper()
	h, _, ok := d.handlers.Lookup(appfederation.JobDeliver)
	if !ok {
		t.Fatal("no handler for " + appfederation.JobDeliver)
	}
	return h(t.Context(), d.deliverJob(t, del))
}

func (d *deliveryEnv) deliverJob(t *testing.T, del federation.Delivery) queue.Job {
	t.Helper()
	j, err := queue.NewJob{Type: appfederation.JobDeliver, Payload: []byte(`{"delivery_id":"` + del.ID + `"}`)}.Build(d.ids.NewID(), d.clock.Now())
	if err != nil {
		t.Fatalf("build job: %v", err)
	}
	return j
}

// attempts returns the recorded attempts of del.
func (d *deliveryEnv) attempts(t *testing.T, del federation.Delivery) []federation.DeliveryAttempt {
	t.Helper()
	as, err := d.h.Federation.ListDeliveryAttempts(t.Context(), del.ID)
	if err != nil {
		t.Fatalf("ListDeliveryAttempts: %v", err)
	}
	return as
}

// last returns the newest attempt of del.
func (d *deliveryEnv) last(t *testing.T, del federation.Delivery) federation.DeliveryAttempt {
	t.Helper()
	as := d.attempts(t, del)
	if len(as) == 0 {
		t.Fatalf("delivery %s has no attempt", del.ID)
	}
	return as[len(as)-1]
}

func (d *deliveryEnv) state(t *testing.T, domain string) federation.InstanceState {
	t.Helper()
	s, err := d.h.Federation.InstanceState(t.Context(), domain)
	if err != nil {
		t.Fatalf("InstanceState: %v", err)
	}
	return s
}

// events returns the availability events of domain, oldest first.
func (d *deliveryEnv) events(t *testing.T, domain string) []federation.AvailabilityEvent {
	t.Helper()
	evs, err := d.h.Federation.ListAvailabilityEvents(t.Context(), domain, 100)
	if err != nil {
		t.Fatalf("ListAvailabilityEvents: %v", err)
	}
	out := make([]federation.AvailabilityEvent, len(evs))
	for i, e := range evs {
		out[len(evs)-1-i] = e
	}
	return out
}

func fedClassifiesResponses(t *testing.T, e *fedEnv) {
	d := newDeliveryEnv(t, e)
	forbidden := func(*kernel.HTTPRequest) (int, http.Header, string, error) {
		return 0, nil, "", fmt.Errorf("%w: 10.0.0.1", kernel.ErrForbiddenDestination)
	}
	slow := func(*kernel.HTTPRequest) (int, http.Header, string, error) {
		return http.StatusTooManyRequests, http.Header{"Retry-After": {"120"}}, "", nil
	}
	cases := []struct {
		name    string
		route   deliveryRoute // nil: the host is unreachable
		outcome federation.DeliveryOutcome
		status  int
		retry   bool
		after   time.Duration
	}{
		{"202", answer(http.StatusAccepted), federation.DeliverySucceeded, 202, false, 0},
		{"200", answer(http.StatusOK), federation.DeliverySucceeded, 200, false, 0},
		{"408", answer(http.StatusRequestTimeout), federation.DeliveryRetryableFailure, 408, true, 0},
		{"429 with Retry-After", slow, federation.DeliveryRetryableFailure, 429, true, 2 * time.Minute},
		{"500", answer(http.StatusInternalServerError), federation.DeliveryRetryableFailure, 500, true, 0},
		{"503", answer(http.StatusServiceUnavailable), federation.DeliveryRetryableFailure, 503, true, 0},
		{"unreachable", nil, federation.DeliveryRetryableFailure, 0, true, 0},
		{"501", answer(http.StatusNotImplemented), federation.DeliveryPermanentFailure, 501, false, 0},
		{"400", answer(http.StatusBadRequest), federation.DeliveryPermanentFailure, 400, false, 0},
		{"401", answer(http.StatusUnauthorized), federation.DeliveryPermanentFailure, 401, false, 0},
		{"403", answer(http.StatusForbidden), federation.DeliveryPermanentFailure, 403, false, 0},
		{"404", answer(http.StatusNotFound), federation.DeliveryPermanentFailure, 404, false, 0},
		{"410", answer(http.StatusGone), federation.DeliveryPermanentFailure, 410, false, 0},
		{"413", answer(http.StatusRequestEntityTooLarge), federation.DeliveryPermanentFailure, 413, false, 0},
		{"422", answer(http.StatusUnprocessableEntity), federation.DeliveryPermanentFailure, 422, false, 0},
		{"forbidden destination", forbidden, federation.DeliveryPermanentFailure, 0, false, 0},
	}
	for i, tc := range cases {
		host := fmt.Sprintf("s%d.example", i)
		if tc.route != nil {
			d.net.route(host, tc.route)
		}
		del := d.delivery(t, federation.ActivityCreate, host)
		r := d.deliver(t, del)
		if r.IsRetry() != tc.retry || (!tc.retry && !r.IsDone()) || r.After() != tc.after {
			t.Errorf("%s: result = %+v (%v), want retry=%v after %v", tc.name, r, r.Err(), tc.retry, tc.after)
		}
		a := d.last(t, del)
		if a.Outcome != tc.outcome || a.HTTPStatus != tc.status || a.DeliveryID != del.ID || !a.AttemptedAt.Equal(d.clock.Now()) {
			t.Errorf("%s: attempt = %+v, want %s with status %d", tc.name, a, tc.outcome, tc.status)
		}
		if sent := tc.status != 0; sent != (a.Scheme != "") || (sent && a.Error != "") || (!sent && a.Error == "") {
			t.Errorf("%s: attempt = %+v, want a scheme with a response and an error without", tc.name, a)
		}
	}
	// A 401 is answered only after the request was sent again with the other scheme.
	if n := d.net.count("s9.example"); n != 2 {
		t.Errorf("requests after a 401 = %d, want 2 (double-knocking)", n)
	}
	// The body is the stored document, with the activity's Content-Type.
	var got *kernel.HTTPRequest
	d.net.route("body.example", func(r *kernel.HTTPRequest) (int, http.Header, string, error) {
		got = r
		return http.StatusAccepted, nil, "", nil
	})
	del := d.delivery(t, federation.ActivityCreate, "body.example")
	d.deliver(t, del)
	doc, err := d.h.Federation.FindOutboundDocument(t.Context(), del.OutboundActivityID)
	if err != nil || got == nil || got.Method != http.MethodPost || !bytes.Equal(got.Body, doc) ||
		got.Header.Get("Content-Type") != "application/activity+json" || got.Header.Get("Signature") == "" {
		t.Errorf("request = %+v (document %s, %v)", got, doc, err)
	}
	// A delivery or activity that does not exist, or an actor without a key, cannot succeed.
	if r := d.deliver(t, federation.Delivery{ID: d.ids.NewID()}); !r.IsFail() {
		t.Errorf("unknown delivery: %+v, want Fail", r)
	}
	d.net.answer("nokey.example", http.StatusAccepted)
	nokey := d.delivery(t, federation.ActivityCreate, "nokey.example")
	d.keys.mu.Lock()
	delete(d.keys.keys, d.actor)
	d.keys.mu.Unlock()
	if r := d.deliver(t, nokey); !r.IsFail() {
		t.Errorf("actor without a key: %+v, want Fail", r)
	}
}

func answer(status int) deliveryRoute {
	return func(*kernel.HTTPRequest) (int, http.Header, string, error) { return status, nil, "", nil }
}

func fedBreaksTheCircuit(t *testing.T, e *fedEnv) {
	d := newDeliveryEnv(t, e)
	d.net.answer("busy.example", http.StatusServiceUnavailable)
	del := d.delivery(t, federation.ActivityCreate, "busy.example")
	for range federation.BreakerThreshold {
		if r := d.deliver(t, del); !r.IsRetry() {
			t.Fatalf("503: %+v, want Retry", r)
		}
	}
	if n := d.net.count("busy.example"); n != federation.BreakerThreshold {
		t.Fatalf("requests = %d, want %d", n, federation.BreakerThreshold)
	}
	// The breaker is open: the attempt is a retryable failure that sends nothing.
	if r := d.deliver(t, del); !r.IsRetry() {
		t.Fatalf("open breaker: %+v, want Retry", r)
	}
	if a := d.last(t, del); d.net.count("busy.example") != federation.BreakerThreshold ||
		a.Outcome != federation.DeliveryRetryableFailure || a.Scheme != "" || a.HTTPStatus != 0 || a.Error == "" {
		t.Fatalf("open breaker: attempt %+v after %d requests", a, d.net.count("busy.example"))
	}
	// Other hosts are not affected.
	d.net.answer("fine.example", http.StatusAccepted)
	if r := d.deliver(t, d.delivery(t, federation.ActivityCreate, "fine.example")); !r.IsDone() {
		t.Fatalf("other host: %+v, want Done", r)
	}
	// After the cooldown the request goes out again, and a success closes the breaker.
	d.clock.Advance(federation.BreakerCooldown)
	d.net.answer("busy.example", http.StatusAccepted)
	if r := d.deliver(t, del); !r.IsDone() || d.net.count("busy.example") != federation.BreakerThreshold+1 {
		t.Fatalf("after the cooldown: %+v, %d requests", r, d.net.count("busy.example"))
	}
	if !d.breaker.Allows("busy.example", d.clock.Now()) {
		t.Fatal("breaker still open after a success")
	}
}

func fedMarksServersUnavailable(t *testing.T, e *fedEnv) {
	d := newDeliveryEnv(t, e)
	ctx := t.Context()
	d.net.answer("down.example", http.StatusBadGateway)
	del := d.delivery(t, federation.ActivityCreate, "down.example")
	fail := func(day, times int) {
		t.Helper()
		// Many failures on one day count as one day.
		for range times {
			d.breaker.Record("down.example", federation.DeliverySucceeded, d.clock.Now())
			if r := d.deliver(t, del); !r.IsRetry() {
				t.Fatalf("day %d: %+v, want Retry", day, r)
			}
			d.clock.Advance(time.Minute)
		}
		d.clock.Advance(24*time.Hour - time.Duration(times)*time.Minute)
	}
	for day := 1; day < federation.FailureDaysThreshold; day++ {
		fail(day, 3)
		h, err := d.h.Federation.FindInstanceHealth(ctx, "down.example")
		if err != nil || h.State != federation.InstanceAvailable || h.FailureDays != day {
			t.Fatalf("after %d failure days: %+v, %v", day, h, err)
		}
	}
	fail(federation.FailureDaysThreshold, 1)
	if s := d.state(t, "down.example"); s != federation.InstanceUnavailable {
		t.Fatalf("after %d failure days: %s, want unavailable", federation.FailureDaysThreshold, s)
	}
	if evs := d.events(t, "down.example"); len(evs) != 1 || evs[0].Kind != federation.MarkedUnavailable || evs[0].Reason != federation.ReasonFailureDays {
		t.Fatalf("events = %+v, want one marked_unavailable (failure_days)", evs)
	}

	// An unavailable server is not sent anything but Follow, Accept, and Reject.
	sent := d.net.count("down.example")
	if r := d.deliver(t, del); !r.IsDone() {
		t.Fatalf("unavailable: %+v, want Done", r)
	}
	if a := d.last(t, del); a.Outcome != federation.DeliverySkipped || d.net.count("down.example") != sent {
		t.Fatalf("unavailable: attempt %+v, %d requests (was %d)", a, d.net.count("down.example"), sent)
	}
	d.net.answer("down.example", http.StatusAccepted)
	follow := d.delivery(t, federation.ActivityFollow, "down.example")
	if r := d.deliver(t, follow); !r.IsDone() || d.last(t, follow).Outcome != federation.DeliverySucceeded {
		t.Fatalf("Follow to an unavailable server: %+v, %+v", r, d.last(t, follow))
	}
	// The successful delivery marks it available again.
	if s := d.state(t, "down.example"); s != federation.InstanceAvailable {
		t.Fatalf("after a success: %s, want available", s)
	}
	evs := d.events(t, "down.example")
	if len(evs) != 2 || evs[1].Kind != federation.MarkedAvailable || evs[1].Reason != federation.ReasonDeliverySucceeded {
		t.Fatalf("events = %+v, want marked_available (delivery_succeeded) last", evs)
	}
	if h, err := d.h.Federation.FindInstanceHealth(ctx, "down.example"); err != nil || h.FailureDays != 0 || h.LastFailureDay != "" {
		t.Fatalf("health after a success = %+v, %v", h, err)
	}

	// A signed request from the server brings it back too; a healthy server gets no event.
	e.markUnavailable(t, "quiet.example")
	if err := d.health.Reached(ctx, "quiet.example", federation.ReasonInboundRequest); err != nil {
		t.Fatalf("Reached: %v", err)
	}
	if err := d.health.Reached(ctx, "quiet.example", federation.ReasonInboundRequest); err != nil {
		t.Fatalf("Reached again: %v", err)
	}
	if err := d.health.Reached(ctx, "unknown.example", federation.ReasonInboundRequest); err != nil {
		t.Fatalf("Reached(unknown): %v", err)
	}
	if s := d.state(t, "quiet.example"); s != federation.InstanceAvailable {
		t.Fatalf("after an inbound request: %s, want available", s)
	}
	if evs := d.events(t, "quiet.example"); len(evs) != 1 || evs[0].Reason != federation.ReasonInboundRequest {
		t.Fatalf("events = %+v, want one marked_available (inbound_request)", evs)
	}
	if _, err := d.h.Federation.FindInstanceHealth(ctx, "unknown.example"); !errors.Is(err, appfederation.ErrNotFound) {
		t.Fatalf("FindInstanceHealth(unknown) = %v, want ErrNotFound", err)
	}

	// A permanent failure leaves the failure days alone.
	d.net.answer("strict.example", http.StatusForbidden)
	d.deliver(t, d.delivery(t, federation.ActivityCreate, "strict.example"))
	if _, err := d.h.Federation.FindInstanceHealth(ctx, "strict.example"); !errors.Is(err, appfederation.ErrNotFound) {
		t.Fatalf("health after a permanent failure: %v, want no row", err)
	}
}

func fedBuriesExhaustedDeliveries(t *testing.T, e *fedEnv) {
	d := newDeliveryEnv(t, e)
	d.net.answer("gone.example", http.StatusServiceUnavailable)
	del := d.delivery(t, federation.ActivityCreate, "gone.example")
	src := &oneJobSource{job: d.deliverJob(t, del)}
	runner := queue.NewPullRunner(src, d.handlers, queue.RunnerOptions{Clock: d.clock, Random: FixedRandom(0)})
	var waits []time.Duration
	for n := 0; !src.buried; n++ {
		if n > 50 {
			t.Fatal("the delivery is never buried")
		}
		ran, err := runner.RunOnce(t.Context())
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if !ran && !src.buried {
			waits = append(waits, src.job.RunAt.Sub(d.clock.Now()))
			d.clock.Set(src.job.RunAt)
		}
	}
	p := appfederation.DeliveryRetryPolicy
	if src.job.Attempt != p.MaxAttempts || len(d.attempts(t, del)) != p.MaxAttempts {
		t.Fatalf("buried after %d runs and %d attempts, want %d", src.job.Attempt, len(d.attempts(t, del)), p.MaxAttempts)
	}
	if len(waits) != len(p.Delays) {
		t.Fatalf("waits = %v, want %v", waits, p.Delays)
	}
	for i, w := range waits {
		if w != p.Delays[i] {
			t.Fatalf("waits = %v, want %v", waits, p.Delays)
		}
	}
}

// oneJobSource is a queue.Source of one job.
type oneJobSource struct {
	job            queue.Job
	leased, buried bool
	completed      bool
}

func (s *oneJobSource) Claim(_ context.Context, _ string, now time.Time, _ time.Duration) (queue.Job, bool, error) {
	if s.leased || s.buried || s.completed || s.job.RunAt.After(now) {
		return queue.Job{}, false, nil
	}
	s.leased = true
	s.job.Attempt++
	return s.job, true, nil
}

func (s *oneJobSource) Complete(context.Context, string, queue.Job) error {
	s.leased, s.completed = false, true
	return nil
}

func (s *oneJobSource) Release(_ context.Context, _ string, _ queue.Job, attempt int, runAt time.Time, lastErr string) error {
	s.leased = false
	s.job.Attempt, s.job.RunAt, s.job.LastError = attempt, runAt, lastErr
	return nil
}

func (s *oneJobSource) Bury(_ context.Context, _ string, _ queue.Job, lastErr string, _ time.Time) error {
	s.leased, s.buried = false, true
	s.job.LastError = lastErr
	return nil
}

func fedProbesUnavailableServers(t *testing.T, e *fedEnv) {
	d := newDeliveryEnv(t, e)
	ctx := t.Context()
	nodeinfo := func(host, href string) deliveryRoute {
		return func(r *kernel.HTTPRequest) (int, http.Header, string, error) {
			switch r.URL {
			case "https://" + host + "/.well-known/nodeinfo":
				return http.StatusOK, nil, `{"links":[{"rel":"http://nodeinfo.diaspora.software/ns/schema/2.0","href":"` + href + `"}]}`, nil
			case "https://" + host + "/nodeinfo/2.0":
				return http.StatusOK, nil, `{"version":"2.0","software":{"name":"mastodon","version":"4.3.0"}}`, nil
			}
			return http.StatusNotFound, nil, "", nil
		}
	}
	for _, s := range []string{"a-back.example", "b-down.example", "c-elsewhere.example", "d-back.example"} {
		e.markUnavailable(t, s)
	}
	d.net.route("a-back.example", nodeinfo("a-back.example", "https://a-back.example/nodeinfo/2.0"))
	d.net.answer("b-down.example", http.StatusServiceUnavailable)
	// A NodeInfo document on another host does not count.
	d.net.route("c-elsewhere.example", nodeinfo("c-elsewhere.example", "https://a-back.example/nodeinfo/2.0"))
	d.net.route("d-back.example", nodeinfo("d-back.example", "https://d-back.example/nodeinfo/2.0"))

	jobs := &fedJobs{seen: map[string]bool{}}
	handlers := &queue.JobHandlerRegistry{}
	scheduler := queue.NewScheduler(jobs, d.clock, nil)
	appfederation.NewProber(appfederation.ProbeDeps{
		Repository: d.h.Federation, Health: d.health, HTTP: d.net, Jobs: jobs, Batch: 2,
		Logger: slog.New(slog.DiscardHandler),
	}).RegisterJobs(handlers, scheduler)
	if s := scheduler.Schedules(); len(s) != 1 || s[0].Every != appfederation.ProbeInterval || s[0].Job.Type != appfederation.JobProbeInstances {
		t.Fatalf("schedules = %+v", s)
	}
	scheduler.Tick(ctx)
	probe, _, _ := handlers.Lookup(appfederation.JobProbeInstances)
	for n := 0; ; n++ {
		if n > 10 {
			t.Fatal("the probe does not end")
		}
		nj, ok := jobs.pop()
		if !ok {
			break
		}
		j, err := nj.Build(d.ids.NewID(), d.clock.Now())
		if err != nil {
			t.Fatalf("build job: %v", err)
		}
		if r := probe(ctx, j); !r.IsDone() {
			t.Fatalf("probe job %s = %+v (%v), want Done", j.Payload, r, r.Err())
		}
	}
	want := map[string]federation.InstanceState{
		"a-back.example": federation.InstanceAvailable, "b-down.example": federation.InstanceUnavailable,
		"c-elsewhere.example": federation.InstanceUnavailable, "d-back.example": federation.InstanceAvailable,
	}
	for host, s := range want {
		if got := d.state(t, host); got != s {
			t.Errorf("%s after the probe: %s, want %s", host, got, s)
		}
	}
	if evs := d.events(t, "d-back.example"); len(evs) != 1 || evs[0].Kind != federation.MarkedAvailable || evs[0].Reason != federation.ReasonProbeSucceeded {
		t.Errorf("events of d-back.example = %+v, want one marked_available (probe_succeeded)", evs)
	}
	left, err := d.h.Federation.ListUnavailableInstances(ctx, "", 10)
	if err != nil || len(left) != 2 || left[0].Domain != "b-down.example" || left[1].Domain != "c-elsewhere.example" {
		t.Errorf("ListUnavailableInstances = %+v, %v", left, err)
	}
}
