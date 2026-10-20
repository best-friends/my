// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/best-friends/my/internal/app/kernel"
)

// Meter is a kernel.Meter that keeps every measurement, for tests of the code that measures.
// A label whose key the instrument did not declare, an instrument declared twice with
// different label keys or units, or an observable counter whose total goes down between two
// collections, is a programming error that Err reports (ADR-0055 §1).
// It is safe for concurrent use.
type Meter struct {
	mu          sync.Mutex
	instruments map[string]kernel.Instrument
	points      []Measurement
	gauges      map[string]kernel.GaugeFunc
	// counters are the observable counters, and totals their totals at the previous Collect
	// by series.
	counters map[string]bool
	totals   map[string]float64
	errs     []error
}

var _ kernel.Meter = (*Meter)(nil)

// Measurement is one recorded value.
type Measurement struct {
	Name   string
	Value  float64
	Labels map[string]string
}

// NewMeter returns an empty Meter.
func NewMeter() *Meter { return &Meter{} }

func (m *Meter) declare(i kernel.Instrument) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.instruments == nil {
		m.instruments = map[string]kernel.Instrument{}
	}
	if len(i.LabelKeys) > kernel.MaxLabels {
		m.errs = append(m.errs, fmt.Errorf("metric %s declares %d labels, more than %d", i.Name, len(i.LabelKeys), kernel.MaxLabels))
	}
	if prev, ok := m.instruments[i.Name]; ok && (prev.Unit != i.Unit || !slices.Equal(prev.LabelKeys, i.LabelKeys)) {
		m.errs = append(m.errs, fmt.Errorf("metric %s declared twice differently: %+v and %+v", i.Name, prev, i))
	}
	m.instruments[i.Name] = i
}

func (m *Meter) record(i kernel.Instrument, v float64, l kernel.Labels) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := i.Check(l); err != nil {
		m.errs = append(m.errs, err)
	}
	labels := map[string]string{}
	for k, val := range l.All() {
		labels[k] = val
	}
	m.points = append(m.points, Measurement{Name: i.Name, Value: v, Labels: labels})
}

// Counter implements kernel.Meter.
func (m *Meter) Counter(i kernel.Instrument) kernel.Counter {
	m.declare(i)
	return fakeInstrument{m: m, i: i, monotonic: true}
}

// UpDownCounter implements kernel.Meter.
func (m *Meter) UpDownCounter(i kernel.Instrument) kernel.UpDownCounter {
	m.declare(i)
	return fakeInstrument{m: m, i: i}
}

// Histogram implements kernel.Meter.
func (m *Meter) Histogram(i kernel.Instrument) kernel.Histogram {
	m.declare(i)
	return fakeInstrument{m: m, i: i}
}

// Gauge implements kernel.Meter. Collect calls f.
func (m *Meter) Gauge(i kernel.Instrument, f kernel.GaugeFunc) {
	m.declare(i)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauges == nil {
		m.gauges = map[string]kernel.GaugeFunc{}
	}
	m.gauges[i.Name] = f
}

// ObservableCounter implements kernel.Meter. Collect calls f and records the totals it
// observes; a total lower than at the previous Collect is reported by Err.
func (m *Meter) ObservableCounter(i kernel.Instrument, f kernel.GaugeFunc) {
	m.Gauge(i, f)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = map[string]bool{}
	}
	m.counters[i.Name] = true
}

// Begin implements kernel.Meter.
func (m *Meter) Begin(ctx context.Context) (context.Context, func()) { return ctx, func() {} }

// Collect calls the functions of the gauges and the observable counters, recording what they
// observe, and returns their errors.
func (m *Meter) Collect(ctx context.Context) error {
	m.mu.Lock()
	gauges := maps.Clone(m.gauges)
	instruments := maps.Clone(m.instruments)
	counters := maps.Clone(m.counters)
	m.mu.Unlock()
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(gauges)) {
		i := instruments[name]
		errs = append(errs, gauges[name](ctx, observerFunc(func(v float64, l kernel.Labels) {
			if counters[name] {
				m.checkTotal(i, v, l)
			}
			m.record(i, v, l)
		})))
	}
	return errors.Join(errs...)
}

// checkTotal reports a total of an observable counter that is negative or lower than at the
// previous Collect.
func (m *Meter) checkTotal(i kernel.Instrument, v float64, l kernel.Labels) {
	key := i.Name
	for k, val := range l.All() {
		key += fmt.Sprintf("{%s=%s}", k, val)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.totals == nil {
		m.totals = map[string]float64{}
	}
	if prev, ok := m.totals[key]; v < 0 || ok && v < prev {
		m.errs = append(m.errs, fmt.Errorf("observable counter %s went down to %v", key, v))
	}
	m.totals[key] = v
}

// Err returns the programming errors seen so far: undeclared labels, negative counter
// increments, observable counters that went down, and conflicting declarations.
func (m *Meter) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return errors.Join(m.errs...)
}

// Instrument returns the declaration of the metric name.
func (m *Meter) Instrument(name string) (kernel.Instrument, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.instruments[name]
	return i, ok
}

// Measurements returns the values recorded for the metric name whose labels include every
// label of match, in order.
func (m *Meter) Measurements(name string, match map[string]string) []Measurement {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Measurement
	for _, p := range m.points {
		if p.Name != name {
			continue
		}
		ok := true
		for k, v := range match {
			if p.Labels[k] != v {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}

// Sum returns the sum of the values that Measurements returns.
func (m *Meter) Sum(name string, match map[string]string) float64 {
	var sum float64
	for _, p := range m.Measurements(name, match) {
		sum += p.Value
	}
	return sum
}

type fakeInstrument struct {
	m         *Meter
	i         kernel.Instrument
	monotonic bool
}

func (f fakeInstrument) Add(_ context.Context, n int64, l kernel.Labels) {
	if f.monotonic && n < 0 {
		f.m.mu.Lock()
		f.m.errs = append(f.m.errs, fmt.Errorf("counter %s decremented by %d", f.i.Name, n))
		f.m.mu.Unlock()
		return
	}
	f.m.record(f.i, float64(n), l)
}

func (f fakeInstrument) Record(_ context.Context, v float64, l kernel.Labels) { f.m.record(f.i, v, l) }

type observerFunc func(v float64, l kernel.Labels)

func (o observerFunc) Observe(v float64, l kernel.Labels) { o(v, l) }
