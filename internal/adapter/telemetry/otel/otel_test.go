// SPDX-License-Identifier: AGPL-3.0-only

package otel_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/best-friends/my/internal/adapter/telemetry/otel"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/pkg/testkit"
)

func newPrometheus(t *testing.T) *otel.Meter {
	t.Helper()
	m, err := otel.New(t.Context(), otel.Options{Exporter: otel.ExporterPrometheus, ServiceVersion: "1.2.3", Role: "web"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(t.Context()) })
	return m
}

func TestMeterContract(t *testing.T) {
	testkit.MeterContract(t, func(t *testing.T) kernel.Meter { return newPrometheus(t) })
}

func scrape(t *testing.T, m *otel.Meter) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape status %d", rec.Code)
	}
	return rec.Body.String()
}

func TestPrometheusExposition(t *testing.T) {
	m := newPrometheus(t)
	h := m.Histogram(kernel.Instrument{
		Name: "http.server.request.duration", Unit: "s", Description: "Duration of HTTP server requests.",
		LabelKeys: []string{"http.route", "my.surface"}, Buckets: kernel.DurationBuckets,
	})
	c := m.Counter(kernel.Instrument{Name: "my.inbox.activities", Unit: "{activity}", Description: "Activities.", LabelKeys: []string{"my.outcome"}})
	u := m.UpDownCounter(kernel.Instrument{Name: "http.server.active_requests", Unit: "{request}", Description: "Active.", LabelKeys: []string{"my.surface"}})
	m.Gauge(kernel.Instrument{Name: "my.queue.dead", Unit: "{job}", Description: "Dead jobs."},
		func(_ context.Context, o kernel.GaugeObserver) error { o.Observe(7, kernel.Labels{}); return nil })

	ctx := t.Context()
	h.Record(ctx, 0.02, kernel.NewLabels("http.route", "/api/v1/statuses/{id}", "my.surface", "client_api"))
	// An undeclared label is dropped, not exported.
	c.Add(ctx, 2, kernel.NewLabels("my.outcome", "accepted", "account_id", "secret"))
	u.Add(ctx, 1, kernel.NewLabels("my.surface", "client_api"))

	out := scrape(t, m)
	for _, want := range []string{
		`http_server_request_duration_seconds_bucket{http_route="/api/v1/statuses/{id}",my_surface="client_api",le="0.025"} 1`,
		`my_inbox_activities_total{my_outcome="accepted"} 2`,
		`http_server_active_requests{my_surface="client_api"} 1`,
		`my_queue_dead 7`,
		`service_version="1.2.3"`,
		`my_role="web"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("scrape lacks %s\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Errorf("scrape exports an undeclared label\n%s", out)
	}
	if err := m.Err(); err != nil {
		t.Error(err)
	}
}

func TestOTLPPushesOnShutdown(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.Method == http.MethodPost && r.URL.Path == "/v1/metrics" {
			posts.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
	m, err := otel.New(t.Context(), otel.Options{Exporter: otel.ExporterOTLP, Role: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Handler() != nil {
		t.Error("Handler() != nil for OTLP")
	}
	m.Counter(kernel.Instrument{Name: "my.job.runs", Unit: "{job}"}).Add(t.Context(), 1, kernel.Labels{})
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if posts.Load() == 0 {
		t.Error("no OTLP export on shutdown")
	}
}

func TestUnknownExporter(t *testing.T) {
	if _, err := otel.New(t.Context(), otel.Options{Exporter: "emf"}); err == nil {
		t.Error("New() with emf succeeded")
	}
}

// BenchmarkRecord measures a measurement with metrics_exporter = prometheus, for comparison
// with none's BenchmarkRecord.
func BenchmarkRecord(b *testing.B) {
	m, err := otel.New(b.Context(), otel.Options{Exporter: otel.ExporterPrometheus})
	if err != nil {
		b.Fatal(err)
	}
	h := m.Histogram(kernel.Instrument{Name: "http.server.request.duration", Unit: "s", LabelKeys: []string{"http.route", "my.surface"}})
	c := m.Counter(kernel.Instrument{Name: "my.job.runs", Unit: "{job}", LabelKeys: []string{"my.outcome"}})
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		ctx, end := m.Begin(ctx)
		h.Record(ctx, 0.01, kernel.NewLabels("http.route", "/api/v1/statuses", "my.surface", "client_api"))
		c.Add(ctx, 1, kernel.NewLabels("my.outcome", "success"))
		end()
	}
}
