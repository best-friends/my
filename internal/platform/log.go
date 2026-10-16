// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"io"
	"log/slog"
)

// NewLogger returns the process logger: JSON lines on w at level and above. Every record
// logged with a request's context carries that request's ID as request_id.
func NewLogger(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(requestIDHandler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

// requestIDHandler adds the request ID found in the context to every record.
type requestIDHandler struct{ slog.Handler }

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{h.Handler.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{h.Handler.WithGroup(name)}
}

type requestIDKey struct{}

// RequestID returns the ID of the HTTP request that ctx belongs to, or "" outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}
