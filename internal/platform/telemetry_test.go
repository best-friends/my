// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/metrics"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/platform"
	"github.com/best-friends/my/pkg/testkit"
)

// TestPrometheusMetrics serves the scrape on its own listener and checks the HTTP and job
// metrics of ADR-0055 §3 in it.
func TestPrometheusMetrics(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true), "MY_METRICS_EXPORTER": "prometheus"})
	mln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ran := make(chan struct{}, 1)
	s := start(t, cfg, platform.ServeOptions{MetricsListener: mln}, func(a *platform.App) {
		a.Handlers.Register("test.metrics", func(context.Context, queue.Job) queue.Result {
			ran <- struct{}{}
			return queue.Done()
		})
	})
	if err := s.app.Jobs.Enqueue(t.Context(), queue.NewJob{Type: "test.metrics"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ran:
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not run")
	}

	if resp, _ := s.do(t, http.MethodGet, "/api/v1/instance", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/instance = %d", resp.StatusCode)
	}
	s.do(t, http.MethodGet, "/ap/nope", nil)
	s.do(t, http.MethodGet, platform.PathLiveness, nil)
	// The public listener never serves the metrics.
	if _, body := s.do(t, http.MethodGet, platform.PathMetrics, nil); strings.Contains(body, "http_server_request_duration") {
		t.Error("the public listener serves the metrics")
	}

	scrape := func() string {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+mln.Addr().String()+platform.PathMetrics, nil)
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	var out string
	deadline := time.Now().Add(10 * time.Second)
	for {
		out = scrape()
		// The job's result is recorded after the handler returns.
		if strings.Contains(out, `my_job_duration_seconds_count{my_job_type="test.metrics",my_outcome="success"} 1`) || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, want := range []string{
		`http_server_request_duration_seconds_count{http_request_method="GET",http_response_status_code="200",http_route="/api/v1/instance",my_surface="client_api"} 1`,
		`http_server_request_duration_seconds_count{http_request_method="GET",http_response_status_code="404",http_route="other",my_surface="web"} 1`,
		`http_server_active_requests{my_surface="client_api"} 0`,
		`my_job_duration_seconds_count{my_job_type="test.metrics",my_outcome="success"} 1`,
		`my_job_wait_seconds_count{my_job_type="test.metrics"} 1`,
		"\nmy_queue_dead 0\n",
		"\nmy_instance_users 0\n",
		`my_media_stored_bytes{my_media_origin="remote_cache"`,
		`my_metrics_snapshot_age_seconds{my_metrics_snapshot_part="queue"`,
		`target_info{`,
		// The system metrics of ADR-0056 §1.
		"\ngo_goroutine_count ",
		`go_schedule_duration_seconds_bucket{`,
		`go_memory_gc_pause_duration_seconds_bucket{`,
		"\ngo_memory_gc_cycles_total ",
		"\nprocess_uptime_seconds ",
		`db_client_connection_count{db_client_connection_pool_name="sqlite",db_client_connection_state="used"}`,
		`my_adapter_operation_duration_seconds_count{my_adapter="queue.db",my_operation="enqueue",my_outcome="success"}`,
		`my_adapter_operation_duration_seconds_count{my_adapter="kv.db",`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("scrape lacks %s\n%s", want, out)
		}
	}
	if strings.Contains(out, `http_route="/healthz"`) || strings.Contains(out, `GET /healthz`) {
		t.Errorf("the health checks are measured\n%s", out)
	}
}

// TestEMFMetrics writes one EMF line for a request.
func TestEMFMetrics(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{
		"MY_DATABASE_URL": newDatabase(t, true), "MY_METRICS_EXPORTER": "emf", "MY_METRICS_EMF_DIMENSIONS": "my.surface",
	})
	var lines syncBuffer
	s := startApp(t, cfg, platform.AppOptions{MetricsWriter: &lines}, platform.ServeOptions{}, nil)
	if resp, _ := s.do(t, http.MethodGet, "/api/v1/instance", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/instance = %d", resp.StatusCode)
	}
	var found bool
	for line := range strings.Lines(lines.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		if m["http.route"] == "/api/v1/instance" {
			found = true
			if _, ok := m["http.server.request.duration"].(float64); !ok || m["my.surface"] != "client_api" || m["_aws"] == nil {
				t.Errorf("EMF line = %v", m)
			}
		}
	}
	if !found {
		t.Errorf("no EMF line for the request:\n%s", lines.String())
	}
}

// TestServiceMetricsLabels runs a post, its jobs, and a stream request through an App with
// testkit.Meter, which reports every label that an instrument did not declare.
func TestServiceMetricsLabels(t *testing.T) {
	t.Parallel()
	m := testkit.NewMeter()
	c := newStatusesClientWith(t, nil, platform.AppOptions{Meter: m}, "alice")
	c.post("alice", map[string]any{"text": "hello", "visibility": "public"})
	if _, err := c.app.Runner.Drain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := m.Err(); err != nil {
		t.Fatal(err)
	}
	if got := m.Sum("my.accounts.registered", nil); got != 1 {
		t.Errorf("my.accounts.registered = %v, want 1", got)
	}
	if got := m.Sum("my.statuses.created", map[string]string{"my.visibility": "public"}); got != 1 {
		t.Errorf("my.statuses.created = %v, want 1", got)
	}
	if got := len(m.Measurements("my.timeline.delivery.lag", nil)); got != 1 {
		t.Errorf("my.timeline.delivery.lag has %d values, want 1", got)
	}
	if got := m.Measurements("http.server.request.duration", map[string]string{
		"http.route": "/api/v1/statuses", "http.request.method": "POST", "my.surface": "client_api", "http.response.status_code": "200",
	}); len(got) != 1 {
		t.Errorf("http.server.request.duration of POST /api/v1/statuses: %v", got)
	}
	if got := m.Sum("http.server.active_requests", nil); got != 0 {
		t.Errorf("http.server.active_requests = %v after the requests, want 0", got)
	}
	if got := len(m.Measurements("my.job.duration", map[string]string{"my.outcome": "success"})); got == 0 {
		t.Error("no my.job.duration after the post's jobs")
	}

	// The instance-wide gauges come from the snapshot (ADR-0055 §4).
	if err := m.Collect(t.Context()); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{"my.instance.users": 1, "my.instance.statuses": 1, "my.queue.dead": 0} {
		if got := m.Measurements(name, nil); len(got) != 1 || got[0].Value != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	if err := m.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(c.app.Scheduler.Schedules(), func(s queue.Schedule) bool { return s.Name == metrics.JobSnapshot }) {
		t.Errorf("no schedule of %s", metrics.JobSnapshot)
	}
}

// TestEMFMetricsSnapshot writes the instance-wide gauges as EMF when the snapshot job runs.
func TestEMFMetricsSnapshot(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true), "MY_METRICS_EXPORTER": "emf"})
	var lines syncBuffer
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{MetricsWriter: &lines})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	h, _, ok := app.Handlers.Lookup(metrics.JobSnapshot)
	if !ok {
		t.Fatalf("no handler of %s", metrics.JobSnapshot)
	}
	if res := h(t.Context(), queue.Job{Type: metrics.JobSnapshot}); !res.IsDone() {
		t.Fatalf("%s = %+v", metrics.JobSnapshot, res)
	}
	var found bool
	for line := range strings.Lines(lines.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		if _, ok := m["my.instance.users"]; ok {
			found = true
			if m["my.instance.users"] != 0.0 || m["my.queue.dead"] != 0.0 || m["_aws"] == nil {
				t.Errorf("EMF line = %v", m)
			}
		}
	}
	if !found {
		t.Errorf("no EMF line of the snapshot:\n%s", lines.String())
	}
}
