// SPDX-License-Identifier: AGPL-3.0-only

package emf_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/telemetry/emf"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/pkg/testkit"
)

// buffer collects the written lines.
type buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *buffer) lines(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for line := range strings.Lines(b.buf.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

var start = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func newMeter(w *buffer) *emf.Meter {
	return emf.New(emf.Options{
		Writer: w, Namespace: "my", Dimensions: []string{"my.surface", "my.role"},
		Properties: map[string]string{"service.name": "my", "my.role": "web"}, Clock: testkit.NewClock(start),
	})
}

func TestMeterContract(t *testing.T) {
	testkit.MeterContract(t, func(*testing.T) kernel.Meter { return newMeter(&buffer{}) })
}

func TestWritesOneLinePerLabelSetAtTheEnd(t *testing.T) {
	var w buffer
	m := newMeter(&w)
	h := m.Histogram(kernel.Instrument{Name: "http.server.request.duration", Unit: "s", LabelKeys: []string{"http.route", "my.surface"}})
	c := m.Counter(kernel.Instrument{Name: "my.statuses.created", Unit: "{status}", LabelKeys: []string{"my.visibility"}})
	u := m.UpDownCounter(kernel.Instrument{Name: "http.server.active_requests", Unit: "{request}", LabelKeys: []string{"my.surface"}})

	ctx, end := m.Begin(t.Context())
	l := kernel.NewLabels("http.route", "/api/v1/statuses", "my.surface", "client_api")
	h.Record(ctx, 0.5, l)
	h.Record(ctx, 0.25, l)
	c.Add(ctx, 1, kernel.NewLabels("my.visibility", "public", "account_id", "x"))
	c.Add(ctx, 1, kernel.NewLabels("my.visibility", "public"))
	u.Add(ctx, 1, kernel.NewLabels("my.surface", "client_api"))
	if got := w.lines(t); len(got) != 0 {
		t.Fatalf("wrote %d lines before the end", len(got))
	}
	end()

	lines := w.lines(t)
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want 2: %v", len(lines), lines)
	}
	first := lines[0]
	if first["http.route"] != "/api/v1/statuses" || first["my.surface"] != "client_api" || first["my.role"] != "web" {
		t.Errorf("labels and properties: %v", first)
	}
	if vals, _ := first["http.server.request.duration"].([]any); len(vals) != 2 || vals[0] != 0.5 || vals[1] != 0.25 {
		t.Errorf("histogram values = %v", first["http.server.request.duration"])
	}
	aws, _ := first["_aws"].(map[string]any)
	if aws["Timestamp"] != float64(start.UnixMilli()) {
		t.Errorf("Timestamp = %v", aws["Timestamp"])
	}
	directive := aws["CloudWatchMetrics"].([]any)[0].(map[string]any)
	if directive["Namespace"] != "my" {
		t.Errorf("Namespace = %v", directive["Namespace"])
	}
	if dims, _ := json.Marshal(directive["Dimensions"]); string(dims) != `[["my.surface","my.role"]]` {
		t.Errorf("Dimensions = %s", dims)
	}
	if metrics, _ := json.Marshal(directive["Metrics"]); string(metrics) != `[{"Name":"http.server.request.duration","Unit":"Seconds"}]` {
		t.Errorf("Metrics = %s", metrics)
	}

	second := lines[1]
	if second["my.statuses.created"] != float64(2) || second["my.visibility"] != "public" {
		t.Errorf("counter line = %v", second)
	}
	if _, ok := second["account_id"]; ok {
		t.Errorf("an undeclared label was written: %v", second)
	}
	if dims, _ := json.Marshal(second["_aws"].(map[string]any)["CloudWatchMetrics"].([]any)[0].(map[string]any)["Dimensions"]); string(dims) != `[["my.role"]]` {
		t.Errorf("Dimensions = %s", dims)
	}
	if _, ok := second["http.server.active_requests"]; ok {
		t.Error("an UpDownCounter was written")
	}

	// Outside a unit of work, and after it ended, a measurement is written at once.
	c.Add(ctx, 1, kernel.NewLabels("my.visibility", "direct"))
	c.Add(t.Context(), 1, kernel.NewLabels("my.visibility", "local"))
	if got := w.lines(t); len(got) != 4 {
		t.Errorf("wrote %d lines, want 4", len(got))
	}
}

func TestSplitsLongHistograms(t *testing.T) {
	var w buffer
	m := newMeter(&w)
	h := m.Histogram(kernel.Instrument{Name: "my.job.duration", Unit: "s"})
	ctx, end := m.Begin(t.Context())
	for range 150 {
		h.Record(ctx, 1, kernel.Labels{})
	}
	end()
	lines := w.lines(t)
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want 2", len(lines))
	}
	if n := len(lines[0]["my.job.duration"].([]any)); n != 100 {
		t.Errorf("first line holds %d values, want 100", n)
	}
	if n := len(lines[1]["my.job.duration"].([]any)); n != 50 {
		t.Errorf("second line holds %d values, want 50", n)
	}
}

func TestUnit(t *testing.T) {
	for in, want := range map[string]string{"s": "Seconds", "ms": "Milliseconds", "By": "Bytes", "{request}": "Count", "1": "Count", "m": "None"} {
		if got := emf.Unit(in); got != want {
			t.Errorf("Unit(%q) = %q, want %q", in, got, want)
		}
	}
}
