// SPDX-License-Identifier: AGPL-3.0-only

package none_test

import (
	"testing"

	"github.com/best-friends/my/internal/adapter/telemetry/none"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/pkg/testkit"
)

func TestMeterContract(t *testing.T) {
	testkit.MeterContract(t, func(*testing.T) kernel.Meter { return none.New() })
}

// BenchmarkRecord measures what a measurement costs with metrics_exporter = none: one call on
// a method that does nothing, without an allocation (ADR-0055 §1).
func BenchmarkRecord(b *testing.B) {
	m := none.New()
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
