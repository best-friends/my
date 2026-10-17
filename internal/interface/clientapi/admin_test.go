// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	appfederation "github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/domain/federation"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakePermissions grants each user the permissions listed for them.
type fakePermissions map[string][]identity.Permission

func (f fakePermissions) HasPermission(_ context.Context, userID string, p identity.Permission) (bool, error) {
	return slices.Contains(f[userID], p), nil
}

// fakeHealth keeps the health of servers (in domain order) and their events (oldest first) with
// the semantics of appfederation.Health.
type fakeHealth struct {
	servers []federation.InstanceHealth
	events  []federation.AvailabilityEvent
	now     time.Time
}

func (f *fakeHealth) Find(_ context.Context, domain string) (federation.InstanceHealth, error) {
	for _, h := range f.servers {
		if h.Domain == domain {
			return h, nil
		}
	}
	return federation.InstanceHealth{Domain: domain, State: federation.InstanceAvailable}, nil
}

func (f *fakeHealth) List(_ context.Context, state federation.InstanceState, after string, limit int) ([]federation.InstanceHealth, error) {
	if state != "" && state != federation.InstanceAvailable && state != federation.InstanceUnavailable {
		return nil, appfederation.ErrInvalidInstanceState
	}
	var out []federation.InstanceHealth
	for _, h := range f.servers {
		if (state == "" || h.State == state) && h.Domain > after && len(out) < limit {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *fakeHealth) Events(_ context.Context, domain, before string, limit int) ([]federation.AvailabilityEvent, error) {
	var out []federation.AvailabilityEvent
	for _, e := range slices.Backward(f.events) {
		if e.Domain == domain && (before == "" || e.ID < before) && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeHealth) MarkAvailable(ctx context.Context, domain string) (federation.InstanceHealth, error) {
	for i, h := range f.servers {
		if h.Domain != domain {
			continue
		}
		next, changed, marked := h.Reached(f.now)
		if changed {
			f.servers[i] = next
		}
		if marked {
			f.events = append(f.events, federation.AvailabilityEvent{
				ID: "01890a5d-ac96-774b-bcce-b302099a7fff", Domain: domain, Kind: federation.MarkedAvailable,
				Reason: federation.ReasonOperator, OccurredAt: f.now,
			})
		}
	}
	return f.Find(ctx, domain)
}

// fakeDeadJobs keeps dead jobs, most recently failed first, and the jobs requeued from them.
type fakeDeadJobs struct {
	dead     []queue.DeadJob
	enqueued []queue.DeadJob
}

func (f *fakeDeadJobs) ListDeadJobs(_ context.Context, after queue.DeadJobPosition, limit int) ([]queue.DeadJob, error) {
	var out []queue.DeadJob
	for _, d := range f.dead {
		c := d.FailedAt.Compare(after.FailedAt)
		if (after.ID == "" || c < 0 || (c == 0 && d.ID < after.ID)) && len(out) < limit {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDeadJobs) RequeueDeadJob(_ context.Context, id string) error {
	i := slices.IndexFunc(f.dead, func(d queue.DeadJob) bool { return d.ID == id })
	if i < 0 {
		return queue.ErrDeadJobNotFound
	}
	f.enqueued = append(f.enqueued, f.dead[i])
	f.dead = slices.Delete(f.dead, i, i+1)
	return nil
}

var adminNow = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

const (
	deadJobA = "01890a5d-ac96-774b-bcce-b302099a6001"
	deadJobB = "01890a5d-ac96-774b-bcce-b302099a6002"
	deadJobC = "01890a5d-ac96-774b-bcce-b302099a6003"
	eventOne = "01890a5d-ac96-774b-bcce-b302099a7001"
	eventTwo = "01890a5d-ac96-774b-bcce-b302099a7002"
)

// newAdminEnv is newAuthEnv whose user alice holds every permission and bob none, with the
// tokens "admin" (alice) and "bob-admin" (bob) of the admin scope and "admin-read" (alice) of
// admin:read alone.
func newAdminEnv(t *testing.T) (*authEnv, *fakeHealth, *fakeDeadJobs) {
	t.Helper()
	health := &fakeHealth{
		now: adminNow,
		servers: []federation.InstanceHealth{
			{Domain: "a.example", State: federation.InstanceUnavailable, Since: adminNow.Add(-time.Hour), FailureDays: 7, LastFailureDay: "20261016"},
			{Domain: "b.example", State: federation.InstanceAvailable, Since: adminNow.Add(-48 * time.Hour), FailureDays: 2, LastFailureDay: "20261015"},
			{Domain: "c.example", State: federation.InstanceUnavailable, Since: adminNow.Add(-2 * time.Hour), FailureDays: 9, LastFailureDay: "20261017"},
		},
		events: []federation.AvailabilityEvent{
			{ID: eventOne, Domain: "a.example", Kind: federation.MarkedUnavailable, Reason: federation.ReasonFailureDays, OccurredAt: adminNow.Add(-3 * time.Hour)},
			{ID: eventTwo, Domain: "a.example", Kind: federation.MarkedUnavailable, Reason: federation.ReasonFailureDays, OccurredAt: adminNow.Add(-time.Hour)},
		},
	}
	failed := adminNow.Add(-time.Minute)
	dead := &fakeDeadJobs{dead: []queue.DeadJob{
		{ID: deadJobC, Type: "federation.deliver", Payload: json.RawMessage(`{"delivery_id":"x"}`), IdempotencyKey: "deliver:x", OrderingKey: "inbox", Attempt: 9, LastError: "502", CreatedAt: failed.Add(-96 * time.Hour), FailedAt: failed},
		{ID: deadJobB, Type: "federation.deliver", Payload: json.RawMessage(`{}`), Attempt: 1, LastError: "bug", CreatedAt: failed, FailedAt: failed},
		{ID: deadJobA, Type: "kv.cleanup", Payload: json.RawMessage(`null`), Attempt: 3, LastError: "db down", CreatedAt: failed.Add(-time.Hour), FailedAt: failed.Add(-time.Second)},
	}}
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{"user-" + aliceAccountID: identity.AllPermissions()}
		o.InstanceHealth = health
		o.DeadJobs = dead
	})
	e.auth.tokens["admin"] = bearer("admin", aliceAccountID)
	e.auth.tokens["admin-read"] = bearer("admin:read", aliceAccountID)
	e.auth.tokens["bob-admin"] = bearer("admin", bobAccountID)
	return e, health, dead
}

func (e *authEnv) admin(t *testing.T, method, path, token string, header map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, clientapi.BasePath+path, nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return e.spec.Serve(t, e.h, req)
}

func decodeJSON[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

var adminOperations = []struct{ method, path string }{
	{http.MethodGet, "/admin/instances"},
	{http.MethodGet, "/admin/instances/a.example"},
	{http.MethodGet, "/admin/instances/a.example/availability_events"},
	{http.MethodPost, "/admin/instances/a.example/mark_available"},
	{http.MethodGet, "/admin/dead_jobs"},
	{http.MethodPost, "/admin/dead_jobs/" + deadJobA + "/retry"},
}

func TestAdminOperationsNeedAPermission(t *testing.T) {
	e, health, dead := newAdminEnv(t)
	for _, op := range adminOperations {
		res := e.admin(t, op.method, op.path, "bob-admin", map[string]string{"Accept-Language": "en"})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s by bob = %d, want 403", op.method, op.path, res.StatusCode)
			continue
		}
		if p := decodeProblem(t, res); p.Type != clientapi.ProblemTypeMissingPermission || p.Detail == nil || !strings.Contains(*p.Detail, "manage_") {
			t.Errorf("%s %s by bob: problem = %+v", op.method, op.path, p)
		}
	}
	res := e.admin(t, http.MethodGet, "/admin/dead_jobs", "bob-admin", map[string]string{"Accept-Language": "ja"})
	if p := decodeProblem(t, res); p.Detail == nil || *p.Detail != "ロールに権限 manage_jobs が必要です。" || res.Header.Get("Content-Language") != "ja" {
		t.Errorf("localized problem = %+v (%s)", p, res.Header.Get("Content-Language"))
	}
	if len(dead.enqueued) != 0 || health.servers[0].State != federation.InstanceUnavailable {
		t.Error("a refused operation changed something")
	}

	// read and write do not grant the admin scopes; admin:read does not grant admin:write.
	for token, op := range map[string]struct{ method, path string }{
		"read":       {http.MethodGet, "/admin/instances"},
		"write":      {http.MethodPost, "/admin/dead_jobs/" + deadJobA + "/retry"},
		"admin-read": {http.MethodPost, "/admin/instances/a.example/mark_available"},
	} {
		res := e.admin(t, op.method, op.path, token, nil)
		if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeInsufficientScope {
			t.Errorf("%s %s with %s = %d %+v, want 403 insufficient-scope", op.method, op.path, token, res.StatusCode, p)
		}
	}
}

func TestListInstanceHealth(t *testing.T) {
	e, _, _ := newAdminEnv(t)
	res := e.admin(t, http.MethodGet, "/admin/instances?limit=2", "admin-read", nil)
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d, Cache-Control = %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	page := decodeJSON[clientapi.AdminInstanceHealthPage](t, res)
	if len(page.Items) != 2 || page.Items[0].Domain != "a.example" || page.Items[1].Domain != "b.example" || page.NextCursor == nil {
		t.Fatalf("first page = %+v", page)
	}
	a := page.Items[0]
	if a.State != clientapi.AdminInstanceStateUnavailable || a.FailureDays != 7 || a.Since == nil || !a.Since.Equal(adminNow.Add(-time.Hour)) ||
		a.LastFailureDay == nil || a.LastFailureDay.String() != "2026-10-16" {
		t.Errorf("a.example = %+v", a)
	}
	res = e.admin(t, http.MethodGet, "/admin/instances?limit=2&cursor="+*page.NextCursor, "admin-read", nil)
	if rest := decodeJSON[clientapi.AdminInstanceHealthPage](t, res); len(rest.Items) != 1 || rest.Items[0].Domain != "c.example" || rest.NextCursor != nil {
		t.Errorf("second page = %+v", rest)
	}
	res = e.admin(t, http.MethodGet, "/admin/instances?state=unavailable", "admin-read", nil)
	if got := decodeJSON[clientapi.AdminInstanceHealthPage](t, res); len(got.Items) != 2 || got.Items[1].Domain != "c.example" {
		t.Errorf("unavailable = %+v", got)
	}
	for _, query := range []string{"state=gone", "limit=0", "cursor=%25"} {
		res := e.admin(t, http.MethodGet, "/admin/instances?"+query, "admin-read", nil)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("?%s = %d, want 400", query, res.StatusCode)
		}
	}
}

func TestGetInstanceHealth(t *testing.T) {
	e, _, _ := newAdminEnv(t)
	res := e.admin(t, http.MethodGet, "/admin/instances/B.Example", "admin-read", nil)
	if got := decodeJSON[clientapi.AdminInstanceHealth](t, res); res.StatusCode != http.StatusOK || got.Domain != "b.example" || got.FailureDays != 2 {
		t.Errorf("b.example = %d %+v", res.StatusCode, got)
	}
	res = e.admin(t, http.MethodGet, "/admin/instances/healthy.example", "admin-read", nil)
	got := decodeJSON[clientapi.AdminInstanceHealth](t, res)
	if got.State != clientapi.AdminInstanceStateAvailable || got.Since != nil || got.LastFailureDay != nil || got.FailureDays != 0 {
		t.Errorf("a server without a row = %+v, want available without since", got)
	}
}

func TestListInstanceAvailabilityEvents(t *testing.T) {
	e, _, _ := newAdminEnv(t)
	res := e.admin(t, http.MethodGet, "/admin/instances/a.example/availability_events?limit=1", "admin-read", nil)
	page := decodeJSON[clientapi.AdminAvailabilityEventPage](t, res)
	if res.StatusCode != http.StatusOK || len(page.Items) != 1 || page.Items[0].Id != eventTwo || page.NextCursor == nil ||
		page.Items[0].Kind != clientapi.AdminAvailabilityEventKindMarkedUnavailable || page.Items[0].Reason != clientapi.AdminAvailabilityEventReasonFailureDays {
		t.Fatalf("first page = %d %+v", res.StatusCode, page)
	}
	res = e.admin(t, http.MethodGet, "/admin/instances/a.example/availability_events?limit=1&cursor="+*page.NextCursor, "admin-read", nil)
	if rest := decodeJSON[clientapi.AdminAvailabilityEventPage](t, res); len(rest.Items) != 1 || rest.Items[0].Id != eventOne || rest.NextCursor != nil {
		t.Errorf("second page = %+v", rest)
	}
	res = e.admin(t, http.MethodGet, "/admin/instances/b.example/availability_events", "admin-read", nil)
	if none := decodeJSON[clientapi.AdminAvailabilityEventPage](t, res); len(none.Items) != 0 {
		t.Errorf("b.example = %+v, want no events", none)
	}
}

func TestMarkInstanceAvailable(t *testing.T) {
	e, health, _ := newAdminEnv(t)
	res := e.admin(t, http.MethodPost, "/admin/instances/a.example/mark_available", "admin", nil)
	got := decodeJSON[clientapi.AdminInstanceHealth](t, res)
	if res.StatusCode != http.StatusOK || got.State != clientapi.AdminInstanceStateAvailable || got.FailureDays != 0 || got.LastFailureDay != nil ||
		got.Since == nil || !got.Since.Equal(adminNow) {
		t.Fatalf("marked = %d %+v", res.StatusCode, got)
	}
	if last := health.events[len(health.events)-1]; last.Reason != federation.ReasonOperator || last.Kind != federation.MarkedAvailable {
		t.Errorf("last event = %+v, want marked_available (operator)", last)
	}
	res = e.admin(t, http.MethodGet, "/admin/instances/a.example/availability_events?limit=1", "admin", nil)
	if page := decodeJSON[clientapi.AdminAvailabilityEventPage](t, res); len(page.Items) != 1 || page.Items[0].Reason != clientapi.AdminAvailabilityEventReasonOperator {
		t.Errorf("newest event = %+v", page)
	}
}

func TestListDeadJobs(t *testing.T) {
	e, _, _ := newAdminEnv(t)
	res := e.admin(t, http.MethodGet, "/admin/dead_jobs?limit=2", "admin-read", nil)
	page := decodeJSON[clientapi.AdminDeadJobPage](t, res)
	if res.StatusCode != http.StatusOK || len(page.Items) != 2 || page.Items[0].Id != deadJobC || page.Items[1].Id != deadJobB || page.NextCursor == nil {
		t.Fatalf("first page = %d %+v", res.StatusCode, page)
	}
	c := page.Items[0]
	if c.Type != "federation.deliver" || c.Payload != `{"delivery_id":"x"}` || c.Attempts != 9 || c.LastError != "502" ||
		c.IdempotencyKey == nil || *c.IdempotencyKey != "deliver:x" || c.OrderingKey == nil || *c.OrderingKey != "inbox" {
		t.Errorf("dead job = %+v", c)
	}
	if b := page.Items[1]; b.IdempotencyKey != nil || b.OrderingKey != nil {
		t.Errorf("dead job without keys = %+v", b)
	}
	res = e.admin(t, http.MethodGet, "/admin/dead_jobs?limit=2&cursor="+*page.NextCursor, "admin-read", nil)
	if rest := decodeJSON[clientapi.AdminDeadJobPage](t, res); len(rest.Items) != 1 || rest.Items[0].Id != deadJobA || rest.NextCursor != nil {
		t.Errorf("second page = %+v", rest)
	}
	for _, cursor := range []string{"bm90IGEgY3Vyc29y", "%25"} {
		res := e.admin(t, http.MethodGet, "/admin/dead_jobs?cursor="+cursor, "admin-read", nil)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("cursor %s = %d, want 400", cursor, res.StatusCode)
		}
	}
}

func TestRetryDeadJob(t *testing.T) {
	e, _, dead := newAdminEnv(t)
	res := e.admin(t, http.MethodPost, "/admin/dead_jobs/"+deadJobB+"/retry", "admin", nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("retry = %d, want 204", res.StatusCode)
	}
	if len(dead.enqueued) != 1 || dead.enqueued[0].ID != deadJobB || slices.ContainsFunc(dead.dead, func(d queue.DeadJob) bool { return d.ID == deadJobB }) {
		t.Fatalf("enqueued = %+v, dead = %+v; want the job enqueued and no longer dead", dead.enqueued, dead.dead)
	}
	res = e.admin(t, http.MethodPost, "/admin/dead_jobs/"+deadJobB+"/retry", "admin", nil)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusNotFound || p.Type != clientapi.ProblemTypeBlank {
		t.Errorf("retry again = %d %+v, want 404", res.StatusCode, p)
	}
}

// TestAdminWithoutPermissions checks that a server without a Permissions port grants no
// permission.
func TestAdminWithoutPermissions(t *testing.T) {
	e := newAuthEnv(t, func(o *clientapi.Options) { o.DeadJobs = &fakeDeadJobs{} })
	e.auth.tokens["admin"] = bearer("admin", aliceAccountID)
	res := e.admin(t, http.MethodGet, "/admin/dead_jobs", "admin", nil)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeMissingPermission {
		t.Errorf("without permissions = %d %+v, want 403", res.StatusCode, p)
	}
}
