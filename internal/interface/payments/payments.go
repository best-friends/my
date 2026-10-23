// SPDX-License-Identifier: AGPL-3.0-only

// Package payments serves the endpoint that the payment provider calls (ADR-0074 §3):
// POST /payments/webhooks/{provider}. It is not part of the client API: no user calls it. The
// body is read up to supporters.WebhookMaxBodyBytes before anything else, and the supporters
// module verifies, records, and queues the delivery.
package payments

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/internal/app/supporters"
)

// PathPrefix is the path of the webhook endpoints, followed by the provider's name.
const PathPrefix = "/payments/webhooks/"

// Webhooks is what the endpoint needs from the supporters module; *supporters.Payments
// implements it.
type Webhooks interface {
	ReceiveWebhook(ctx context.Context, req supporters.WebhookRequest) (supporters.WebhookResult, error)
}

// ProblemWriter writes the error response of an HTTP status, the same one the rest of the
// server uses for that status.
type ProblemWriter func(w http.ResponseWriter, r *http.Request, status int)

// Options configure a Handler.
type Options struct {
	Webhooks Webhooks
	// Problem writes error responses; nil writes a plain-text body.
	Problem ProblemWriter
	// Logger logs unexpected errors; nil means slog.Default().
	Logger *slog.Logger
}

// Handler serves the webhook endpoint.
type Handler struct {
	o Options
}

// New returns a Handler on o.
func New(o Options) *Handler {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Problem == nil {
		o.Problem = func(w http.ResponseWriter, _ *http.Request, status int) {
			http.Error(w, http.StatusText(status), status)
		}
	}
	return &Handler{o: o}
}

// Router registers handlers for http.ServeMux patterns.
type Router interface {
	Handle(pattern string, h http.Handler)
}

// Register registers the endpoint on r.
func (h *Handler) Register(r Router) {
	r.Handle("POST "+PathPrefix+"{provider}", h)
}

// ServeHTTP implements http.Handler: 200 for a delivery that is recorded, a duplicate, or of a
// type that is ignored; 400 for a wrong signature, time, mode, or body; 404 for another
// provider; 413 beyond the body's limit; 429 beyond the rate limit; and 503 when the delivery
// could not be recorded, for the provider to send it again.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, supporters.WebhookMaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		status := http.StatusBadRequest
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		h.o.Problem(w, r, status)
		return
	}
	res, err := h.o.Webhooks.ReceiveWebhook(r.Context(), supporters.WebhookRequest{
		Provider: r.PathValue("provider"), Header: r.Header, Body: body, ClientIP: kernel.RequestClientIP(r),
	})
	if err != nil {
		h.o.Logger.ErrorContext(r.Context(), "payment webhook not recorded", slog.Any("error", err))
		h.o.Problem(w, r, http.StatusServiceUnavailable)
		return
	}
	switch res.Outcome {
	case supporters.WebhookRecorded, supporters.WebhookDuplicate, supporters.WebhookIgnored:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"received":true}` + "\n"))
	case supporters.WebhookRejected:
		// The kind, never the body or the signature (ADR-0074 §5).
		h.o.Logger.WarnContext(r.Context(), "payment webhook refused", slog.String("provider", r.PathValue("provider")),
			slog.String("error_kind", string(res.ErrorKind)))
		h.o.Problem(w, r, http.StatusBadRequest)
	case supporters.WebhookRateLimited:
		w.Header().Set("Retry-After", strconv.Itoa(int((res.RetryAfter+time.Second-1)/time.Second)))
		h.o.Problem(w, r, http.StatusTooManyRequests)
	default:
		h.o.Problem(w, r, http.StatusNotFound)
	}
}
