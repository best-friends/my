// SPDX-License-Identifier: AGPL-3.0-only

// Package emf is the kernel.Meter adapter of metrics_exporter = emf (ADR-0055 §2): it writes
// the measurements as JSON lines in the CloudWatch Embedded Metric Format, which CloudWatch
// Logs turns into metrics, so that the process aggregates nothing and a frozen Lambda execution
// environment loses nothing.
//
// The measurements made with the context of a unit of work (kernel.Meter.Begin: an HTTP request
// or a job) are written when it ends: one line per distinct label set, holding every metric
// recorded with those labels (a counter's sum, a histogram's values). A measurement outside a
// unit of work is written at once. UpDownCounters record nothing, since they are values of one
// process that the platform's own metrics stand in for (Lambda's concurrency). Gauges are not
// collected: WriteGauges (kernel.GaugeWriter) writes them when the periodic job of the metrics
// snapshot asks (ADR-0055 §4).
package emf

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/best-friends/my/internal/app/kernel"
)

// maxValues is the most values of one metric in one EMF document.
const maxValues = 100

// Options configure New.
type Options struct {
	// Writer receives the lines; the process passes its standard output.
	Writer io.Writer
	// Namespace is the CloudWatch namespace of the metrics (metrics_emf_namespace).
	Namespace string
	// Dimensions are the label keys that become CloudWatch dimensions when a line carries
	// them (metrics_emf_dimensions); the other labels stay properties of the line, which
	// CloudWatch Logs Insights can still query. Each combination of dimension values is a
	// metric that CloudWatch bills.
	Dimensions []string
	// Properties are written on every line: the resource (service.name, service.version,
	// my.role, ADR-0055 §6).
	Properties map[string]string
	Clock      kernel.Clock
}

// Meter is a kernel.Meter that writes EMF lines.
type Meter struct {
	opts Options
	mu   sync.Mutex // serializes the writes, one line each

	gaugesMu sync.Mutex
	gauges   []gauge
}

type gauge struct {
	i kernel.Instrument
	f kernel.GaugeFunc
}

var (
	_ kernel.Meter       = (*Meter)(nil)
	_ kernel.GaugeWriter = (*Meter)(nil)
)

// New returns the Meter on opts. Writer and Clock are required.
func New(opts Options) *Meter {
	if opts.Writer == nil || opts.Clock == nil {
		panic("emf: Options.Writer and Options.Clock are required")
	}
	if opts.Namespace == "" {
		opts.Namespace = "my"
	}
	return &Meter{opts: opts}
}

type kind int

const (
	kindSum kind = iota + 1
	kindValues
)

// Counter implements kernel.Meter.
func (m *Meter) Counter(i kernel.Instrument) kernel.Counter {
	return instrument{m: m, i: i, k: kindSum}
}

// UpDownCounter implements kernel.Meter; it records nothing.
func (m *Meter) UpDownCounter(i kernel.Instrument) kernel.UpDownCounter {
	return kernel.NopMeter{}.UpDownCounter(i)
}

// Histogram implements kernel.Meter.
func (m *Meter) Histogram(i kernel.Instrument) kernel.Histogram {
	return instrument{m: m, i: i, k: kindValues}
}

// Gauge implements kernel.Meter. EMF has no collection: WriteGauges calls f.
func (m *Meter) Gauge(i kernel.Instrument, f kernel.GaugeFunc) {
	m.gaugesMu.Lock()
	defer m.gaugesMu.Unlock()
	m.gauges = append(m.gauges, gauge{i: i, f: f})
}

// WriteGauges implements kernel.GaugeWriter: it calls the gauge functions and writes their
// values, one line per distinct label set, and returns the errors of the functions.
func (m *Meter) WriteGauges(ctx context.Context) error {
	m.gaugesMu.Lock()
	gauges := slices.Clone(m.gauges)
	m.gaugesMu.Unlock()
	b := &batch{}
	bctx := context.WithValue(ctx, batchKey{}, b)
	var errs []error
	for _, g := range gauges {
		errs = append(errs, g.f(ctx, gaugeObserver{m: m, ctx: bctx, i: g.i}))
	}
	m.flush(b)
	return errors.Join(errs...)
}

type gaugeObserver struct {
	m   *Meter
	ctx context.Context
	i   kernel.Instrument
}

func (o gaugeObserver) Observe(v float64, l kernel.Labels) {
	o.m.record(o.ctx, o.i, kindValues, v, 0, l)
}

type batchKey struct{}

// Begin implements kernel.Meter: the measurements made with the returned context are written
// when end is called.
func (m *Meter) Begin(ctx context.Context) (context.Context, func()) {
	b := &batch{}
	return context.WithValue(ctx, batchKey{}, b), func() { m.flush(b) }
}

// batch is the measurements of one unit of work.
type batch struct {
	mu     sync.Mutex
	groups []*group
	ended  bool
}

// group is the metrics recorded with one label set.
type group struct {
	labels  []label
	metrics []*measure
}

type label struct{ key, value string }

type measure struct {
	name, unit string
	sum        int64
	values     []float64
}

type instrument struct {
	m *Meter
	i kernel.Instrument
	k kind
}

func (in instrument) Add(ctx context.Context, n int64, l kernel.Labels) {
	if n < 0 {
		return
	}
	in.m.record(ctx, in.i, in.k, float64(n), n, l)
}

func (in instrument) Record(ctx context.Context, v float64, l kernel.Labels) {
	in.m.record(ctx, in.i, in.k, v, 0, l)
}

func (m *Meter) record(ctx context.Context, i kernel.Instrument, k kind, v float64, n int64, l kernel.Labels) {
	labels := make([]label, 0, l.Len())
	for key, value := range l.All() {
		if i.Accepts(key) {
			labels = append(labels, label{key, value})
		}
	}
	slices.SortFunc(labels, func(a, b label) int { return strings.Compare(a.key, b.key) })

	b, _ := ctx.Value(batchKey{}).(*batch)
	if b == nil {
		m.write(&group{labels: labels, metrics: []*measure{newMeasure(i, k, v, n)}})
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ended {
		m.write(&group{labels: labels, metrics: []*measure{newMeasure(i, k, v, n)}})
		return
	}
	g := b.group(labels)
	for _, ms := range g.metrics {
		if ms.name != i.Name {
			continue
		}
		if k == kindSum {
			ms.sum += n
			return
		}
		if len(ms.values) == maxValues {
			// A document holds at most maxValues values of a metric: write those so far.
			m.write(&group{labels: g.labels, metrics: []*measure{ms}})
			ms.values = nil
		}
		ms.values = append(ms.values, v)
		return
	}
	g.metrics = append(g.metrics, newMeasure(i, k, v, n))
}

func newMeasure(i kernel.Instrument, k kind, v float64, n int64) *measure {
	ms := &measure{name: i.Name, unit: i.Unit}
	if k == kindSum {
		ms.sum = n
	} else {
		ms.values = []float64{v}
	}
	return ms
}

func (b *batch) group(labels []label) *group {
	for _, g := range b.groups {
		if slices.Equal(g.labels, labels) {
			return g
		}
	}
	g := &group{labels: labels}
	b.groups = append(b.groups, g)
	return g
}

func (m *Meter) flush(b *batch) {
	b.mu.Lock()
	groups := b.groups
	b.groups, b.ended = nil, true
	b.mu.Unlock()
	for _, g := range groups {
		m.write(g)
	}
}

type document struct {
	Timestamp         int64       `json:"Timestamp"`
	CloudWatchMetrics []directive `json:"CloudWatchMetrics"`
}

type directive struct {
	Namespace  string      `json:"Namespace"`
	Dimensions [][]string  `json:"Dimensions"`
	Metrics    []metricDef `json:"Metrics"`
}

type metricDef struct {
	Name string `json:"Name"`
	Unit string `json:"Unit"`
}

// write writes g as one EMF line.
func (m *Meter) write(g *group) {
	if len(g.metrics) == 0 {
		return
	}
	line := make(map[string]any, len(m.opts.Properties)+len(g.labels)+len(g.metrics)+1)
	for k, v := range m.opts.Properties {
		line[k] = v
	}
	for _, l := range g.labels {
		line[l.key] = l.value
	}
	dims := []string{}
	for _, d := range m.opts.Dimensions {
		if _, ok := line[d]; ok {
			dims = append(dims, d)
		}
	}
	defs := make([]metricDef, 0, len(g.metrics))
	for _, ms := range g.metrics {
		defs = append(defs, metricDef{Name: ms.name, Unit: Unit(ms.unit)})
		switch len(ms.values) {
		case 0:
			line[ms.name] = ms.sum
		case 1:
			line[ms.name] = ms.values[0]
		default:
			line[ms.name] = ms.values
		}
	}
	line["_aws"] = document{
		Timestamp: m.opts.Clock.Now().UnixMilli(),
		CloudWatchMetrics: []directive{{
			Namespace: m.opts.Namespace, Dimensions: [][]string{dims}, Metrics: defs,
		}},
	}
	out, err := json.Marshal(line)
	if err != nil {
		return // only a NaN or an infinity fails, which no instrument records
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, _ = m.opts.Writer.Write(append(out, '\n'))
}

// Unit returns the CloudWatch unit of a UCUM unit: Seconds, Milliseconds, Bytes, Count for the
// annotations such as {request} and 1, and None otherwise.
func Unit(ucum string) string {
	switch {
	case ucum == "s":
		return "Seconds"
	case ucum == "ms":
		return "Milliseconds"
	case ucum == "By":
		return "Bytes"
	case ucum == "1", strings.HasPrefix(ucum, "{") && strings.HasSuffix(ucum, "}"):
		return "Count"
	default:
		return "None"
	}
}
