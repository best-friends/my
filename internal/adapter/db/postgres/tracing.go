// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/jackc/pgx/v5"

	"github.com/mycroblog/my/internal/adapter/telemetry/dbtrace"
	"github.com/mycroblog/my/internal/app/kernel"
)

// tracerRef holds the Tracer of a DB, which SetTracer sets after the pool's QueryTracer exists.
type tracerRef struct{ t atomic.Pointer[kernel.Tracer] }

func (r *tracerRef) set(t kernel.Tracer) {
	t = kernel.TracerOrNop(t)
	r.t.Store(&t)
}

func (r *tracerRef) get() kernel.Tracer {
	if r == nil {
		return kernel.NopTracer{}
	}
	if t := r.t.Load(); t != nil {
		return *t
	}
	return kernel.NopTracer{}
}

// queryTracer is the pgx QueryTracer of the pool: a span of every query, named after the sqlc
// query (dbtrace.QuerySpan).
type queryTracer struct{ ref *tracerRef }

var _ pgx.QueryTracer = queryTracer{}

type querySpanKey struct{}

func (q queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	t := q.ref.get()
	if kernel.IsNopTracer(t) {
		return ctx
	}
	ctx, s := dbtrace.StartQuery(ctx, t, dbtrace.SystemPostgreSQL, data.SQL)
	return context.WithValue(ctx, querySpanKey{}, s)
}

func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if s, ok := ctx.Value(querySpanKey{}).(kernel.Span); ok {
		err := data.Err
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		dbtrace.End(s, err)
	}
}
