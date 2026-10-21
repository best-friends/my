// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/adapter/telemetry/otel/otlptest"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/platform/config"
	"github.com/best-friends/my/schema"
)

// The span attributes that ADR-0063 §4 allows.
var allowedSpanAttributes = map[string]bool{
	"http.request.method": true, "http.route": true, "http.response.status_code": true, "my.surface": true,
	"my.job.type": true, "my.job.attempt": true, "my.outcome": true, "activity.type": true,
	"db.system.name": true, "db.operation.name": true, "error.type": true, "server.address": true,
}

const (
	incomingTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	incomingSpanID  = "00f067aa0ba902b7"
)

// newTracedApp returns an App on env whose route POST /test/traced enqueues a test.traced job,
// which handler runs.
func newTracedApp(t *testing.T, env map[string]string, handler queue.Handler) (*App, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "my.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), schema.SQLite); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	kek := make([]byte, 32)
	_, _ = rand.Read(kek)
	full := map[string]string{
		"MY_HOST": "example.test", "MY_KEK": base64.StdEncoding.EncodeToString(kek), "MY_BLOB_FS_DIR": t.TempDir(),
		"MY_DATABASE_URL": path, "MY_TRUSTED_PROXIES": "192.0.2.10",
	}
	for k, v := range env {
		full[k] = v
	}
	cfg, _, err := config.Load(config.Source{LookupEnv: func(k string) (string, bool) { v, ok := full[k]; return v, ok }})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	a, err := NewApp(t.Context(), cfg, AppOptions{Logger: NewLogger(&logs, slog.LevelInfo)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	a.Handlers.Register("test.traced", handler)
	a.handle("POST /test/traced", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.Jobs.Enqueue(r.Context(), queue.NewJob{Type: "test.traced"}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}), surfaceClientAPI)
	return a, &logs
}

func post(t *testing.T, h http.Handler, remote string) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test/traced?token=secret", nil)
	req.RemoteAddr = remote
	req.Header.Set("Traceparent", "00-"+incomingTraceID+"-"+incomingSpanID+"-01")
	req.Header.Set("Tracestate", "vendor=value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /test/traced = %d %s", rec.Code, rec.Body)
	}
}

// TestTracesFromHTTPToJobs checks ADR-0063 with traces_exporter = otlp: a request from a trusted
// proxy continues its traceparent, its queries are its children, the job it enqueued runs in a
// new trace linked to it, every span reaches the OTLP receiver, no attribute is outside the
// allowlist of §4, and the log lines carry the trace and span IDs.
func TestTracesFromHTTPToJobs(t *testing.T) {
	c := &otlptest.Collector{}
	srv := httptest.NewServer(c)
	defer srv.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", srv.URL+"/v1/traces")
	a, logs := newTracedApp(t, map[string]string{
		"MY_TRACES_EXPORTER": "otlp", "MY_TRACES_SAMPLE_RATIO": "1", "MY_TRACES_FLUSH": "unit",
	}, func(context.Context, queue.Job) queue.Result { return queue.Done() })
	h := a.Handler()
	post(t, h, "192.0.2.10:4000")   // a trusted proxy
	post(t, h, "203.0.113.50:4000") // anyone else
	if n, err := a.Runner.Drain(t.Context()); err != nil || n != 2 {
		t.Fatalf("Drain = %d, %v", n, err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	spans := c.Spans()
	var trusted, other *otlptest.Span
	var jobs []otlptest.Span
	for i, s := range spans {
		switch {
		case s.Name == "POST /test/traced" && s.TraceID == incomingTraceID:
			trusted = &spans[i]
		case s.Name == "POST /test/traced":
			other = &spans[i]
		case s.Name == "test.traced":
			jobs = append(jobs, s)
		}
		for k, v := range s.Attrs {
			if !allowedSpanAttributes[k] {
				t.Errorf("span %s has the attribute %s = %q outside the allowlist", s.Name, k, v)
			}
			if strings.Contains(v, "secret") || strings.Contains(v, "192.0.2") || strings.Contains(v, "203.0.113") {
				t.Errorf("span %s attribute %s = %q", s.Name, k, v)
			}
		}
	}
	if trusted == nil || len(jobs) != 2 {
		t.Fatalf("no request span in the trusted proxy's trace, or %d job spans, want 2: %+v", len(jobs), spans)
	}
	if trusted.ParentSpanID != incomingSpanID || trusted.Kind != "server" || trusted.Attrs["http.route"] != "/test/traced" {
		t.Errorf("the trusted proxy's request span = %+v, want a server span under %s", *trusted, incomingSpanID)
	}
	if other == nil || other.ParentSpanID != "" {
		t.Errorf("an untrusted client's request span = %+v, want the root of a new trace", other)
	}

	// The job enqueued by the trusted request: a new trace, linked to the request's span.
	var job *otlptest.Span
	for i, j := range jobs {
		if slices.Equal(j.Links, []string{trusted.SpanID}) {
			job = &jobs[i]
		}
	}
	if job == nil {
		t.Fatalf("no job span links to the request span %s: %+v", trusted.SpanID, jobs)
	}
	if job.TraceID == incomingTraceID || job.ParentSpanID != "" || job.Kind != "consumer" || job.Attrs["my.outcome"] != "success" {
		t.Errorf("job span = %+v, want a consumer root of a new trace", *job)
	}

	// The enqueue's query is in the request's trace, and the job's result in the job's.
	inTrace := func(traceID, name string) bool {
		return slices.ContainsFunc(spans, func(s otlptest.Span) bool { return s.TraceID == traceID && s.Name == name })
	}
	if !inTrace(trusted.TraceID, "InsertJob") || !inTrace(trusted.TraceID, "my.db.transaction") {
		t.Error("the request's trace lacks its enqueue's transaction and query")
	}
	if !inTrace(job.TraceID, "DeleteClaimedJob") {
		t.Error("the job's trace lacks the transaction that completes it")
	}

	// The access log line names the request's span.
	found := false
	for line := range strings.Lines(logs.String()) {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) != nil || m["msg"] != "http request" {
			continue
		}
		if m["trace_id"] == incomingTraceID && m["span_id"] == trusted.SpanID {
			found = true
		}
	}
	if !found {
		t.Errorf("no access log line carries the request's trace_id and span_id:\n%s", logs)
	}
}

// TestNoneTracerCorrelatesLogs checks that traces_exporter = none records no span and sends
// nothing, yet the log lines of a request from a trusted proxy carry its traceparent's IDs
// (ADR-0063 §6), and a job enqueued then carries no TraceParent.
func TestNoneTracerCorrelatesLogs(t *testing.T) {
	var traceParents []string
	a, logs := newTracedApp(t, nil, func(_ context.Context, j queue.Job) queue.Result {
		traceParents = append(traceParents, j.TraceParent)
		return queue.Done()
	})
	h := a.Handler()
	post(t, h, "192.0.2.10:4000")
	post(t, h, "203.0.113.50:4000")
	if n, err := a.Runner.Drain(t.Context()); err != nil || n != 2 {
		t.Fatalf("Drain = %d, %v", n, err)
	}
	if !slices.Equal(traceParents, []string{"", ""}) {
		t.Errorf("TraceParents = %q, want none", traceParents)
	}
	var correlated, plain int
	for line := range strings.Lines(logs.String()) {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) != nil || m["msg"] != "http request" {
			continue
		}
		switch m["trace_id"] {
		case incomingTraceID:
			correlated++
			if m["span_id"] != incomingSpanID {
				t.Errorf("span_id = %v, want the proxy's %s", m["span_id"], incomingSpanID)
			}
		case nil:
			plain++
		default:
			t.Errorf("trace_id = %v", m["trace_id"])
		}
	}
	if correlated != 1 || plain != 1 {
		t.Errorf("access log lines with the trace ID %d, without %d; want 1 each:\n%s", correlated, plain, logs)
	}
}
