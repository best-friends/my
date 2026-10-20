// SPDX-License-Identifier: AGPL-3.0-only

package emf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/best-friends/my/internal/app/kernel"
)

func TestWritesGaugesWhenAsked(t *testing.T) {
	var w buffer
	m := newMeter(&w)
	m.Gauge(kernel.Instrument{Name: "my.queue.ready", Unit: "{job}", LabelKeys: []string{"my.job.type"}},
		func(_ context.Context, o kernel.GaugeObserver) error {
			o.Observe(3, kernel.NewLabels("my.job.type", "delivery.deliver"))
			o.Observe(1, kernel.NewLabels("my.job.type", "media.process"))
			return nil
		})
	m.Gauge(kernel.Instrument{Name: "my.queue.dead", Unit: "{job}"},
		func(_ context.Context, o kernel.GaugeObserver) error { o.Observe(2, kernel.Labels{}); return nil })
	m.Gauge(kernel.Instrument{Name: "my.instance.users", Unit: "{account}"},
		func(_ context.Context, o kernel.GaugeObserver) error {
			o.Observe(5, kernel.Labels{})
			return context.Canceled
		})
	if got := w.lines(t); len(got) != 0 {
		t.Fatalf("wrote %d lines before WriteGauges", len(got))
	}

	if err := m.WriteGauges(t.Context()); !errors.Is(err, context.Canceled) {
		t.Errorf("WriteGauges = %v, want the error of the gauge", err)
	}
	lines := w.lines(t)
	if len(lines) != 3 {
		t.Fatalf("wrote %d lines, want 3: %v", len(lines), lines)
	}
	if lines[0]["my.job.type"] != "delivery.deliver" || lines[0]["my.queue.ready"] != 3.0 {
		t.Errorf("first line = %v", lines[0])
	}
	if lines[1]["my.job.type"] != "media.process" || lines[1]["my.queue.ready"] != 1.0 {
		t.Errorf("second line = %v", lines[1])
	}
	if lines[2]["my.queue.dead"] != 2.0 || lines[2]["my.instance.users"] != 5.0 {
		t.Errorf("unlabeled line = %v", lines[2])
	}
}
