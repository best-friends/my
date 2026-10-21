// SPDX-License-Identifier: AGPL-3.0-only

// Package otlptest is an OTLP/HTTP trace receiver for tests: it decodes the spans that the
// otel Tracer exports, so that tests outside internal/adapter/telemetry check what leaves the
// process without importing OpenTelemetry (ADR-0063 §1).
package otlptest

import (
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// Span is one exported span. IDs are lower-case hex; a root has no ParentSpanID.
type Span struct {
	Name                          string
	TraceID, SpanID, ParentSpanID string
	// Kind is server, client, internal, producer, or consumer.
	Kind string
	// Links are the span IDs that the span links to.
	Links []string
	// Attrs are the attributes, integers in decimal.
	Attrs map[string]string
	// Resource are the attributes of the resource of the span.
	Resource map[string]string
	// Gzip reports whether the request that carried the span was gzip-compressed.
	Gzip bool
}

// Collector is an http.Handler that takes OTLP/HTTP trace exports (protobuf, gzip or not) and
// keeps their spans. It is safe for concurrent use.
type Collector struct {
	mu    sync.Mutex
	spans []Span
}

// ServeHTTP implements http.Handler.
func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body io.Reader = r.Body
	gz := r.Header.Get("Content-Encoding") == "gzip"
	if gz {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body = zr
	}
	b, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(b, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var spans []Span
	for _, rs := range req.GetResourceSpans() {
		res := attrs(rs.GetResource().GetAttributes())
		for _, ss := range rs.GetScopeSpans() {
			for _, s := range ss.GetSpans() {
				out := Span{
					Name: s.GetName(), TraceID: hex.EncodeToString(s.GetTraceId()), SpanID: hex.EncodeToString(s.GetSpanId()),
					ParentSpanID: hex.EncodeToString(s.GetParentSpanId()), Kind: kind(s.GetKind()),
					Attrs: attrs(s.GetAttributes()), Resource: res, Gzip: gz,
				}
				for _, l := range s.GetLinks() {
					out.Links = append(out.Links, hex.EncodeToString(l.GetSpanId()))
				}
				spans = append(spans, out)
			}
		}
	}
	c.mu.Lock()
	c.spans = append(c.spans, spans...)
	c.mu.Unlock()
	w.Header().Set("Content-Type", "application/x-protobuf")
}

// Spans returns the spans received so far, in the order they arrived.
func (c *Collector) Spans() []Span {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.spans)
}

func attrs(kvs []*commonpb.KeyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		v := kv.GetValue()
		switch v.GetValue().(type) {
		case *commonpb.AnyValue_IntValue:
			m[kv.GetKey()] = fmt.Sprint(v.GetIntValue())
		default:
			m[kv.GetKey()] = v.GetStringValue()
		}
	}
	return m
}

func kind(k tracepb.Span_SpanKind) string {
	switch k {
	case tracepb.Span_SPAN_KIND_SERVER:
		return "server"
	case tracepb.Span_SPAN_KIND_CLIENT:
		return "client"
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return "producer"
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return "consumer"
	}
	return "internal"
}
