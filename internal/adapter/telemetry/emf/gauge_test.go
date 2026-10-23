// SPDX-License-Identifier: AGPL-3.0-only

package emf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mycroblog/my/internal/app/kernel"
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

func TestWritesTheIncreaseOfObservableCounters(t *testing.T) {
	var w buffer
	m := newMeter(&w)
	user, system := 1.5, 0.5
	m.ObservableCounter(kernel.Instrument{Name: "process.cpu.time", Unit: "s", LabelKeys: []string{"cpu.mode"}},
		func(_ context.Context, o kernel.GaugeObserver) error {
			o.Observe(user, kernel.NewLabels("cpu.mode", "user"))
			o.Observe(system, kernel.NewLabels("cpu.mode", "system"))
			return nil
		})
	values := func() map[string]any {
		t.Helper()
		if err := m.WriteGauges(t.Context()); err != nil {
			t.Fatal(err)
		}
		got := map[string]any{}
		for _, line := range w.lines(t) {
			got[line["cpu.mode"].(string)] = line["process.cpu.time"]
		}
		w.reset()
		return got
	}
	if got := values(); got["user"] != 1.5 || got["system"] != 0.5 {
		t.Errorf("first write = %v, want the totals", got)
	}
	user, system = 2.25, 0.5
	if got := values(); got["user"] != 0.75 || got["system"] != 0.0 {
		t.Errorf("second write = %v, want the increases", got)
	}
	// A total that went back to 0 (its source restarted) counts from 0.
	user = 0.25
	if got := values(); got["user"] != 0.25 {
		t.Errorf("write after a restart = %v, want the new total", got)
	}
}
