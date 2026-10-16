// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	appaccount "github.com/best-friends/my/internal/app/account"
)

// Accounts is what the handlers need from the account module; *account.Service implements it.
type Accounts interface {
	FindByID(ctx context.Context, id string) (appaccount.Actor, error)
	FindByHandle(ctx context.Context, username, domain string) (appaccount.Actor, error)
	InstanceActor(ctx context.Context) (appaccount.Actor, error)
	CountLocalUsers(ctx context.Context) (int, error)
}

// ProblemWriter writes the error response of an HTTP status, the same one the rest of the
// server uses for that status.
type ProblemWriter func(w http.ResponseWriter, r *http.Request, status int)

// Options configure a Handler.
type Options struct {
	// Host is the serving domain of every ActivityPub id (ADR-0018).
	Host string
	// AccountDomain is the domain of handles; empty means Host.
	AccountDomain string
	// SoftwareVersion is the server version that NodeInfo reports.
	SoftwareVersion string
	// SourceURL is where users get the server's source code (NodeInfo software.repository).
	SourceURL string

	Accounts Accounts
	// HTML serves the web client's page for profile pages (/@alice) and other single-segment
	// paths when the request does not ask for ActivityPub; nil answers 404.
	HTML http.Handler
	// Problem writes error responses; nil writes a plain-text body.
	Problem ProblemWriter
	// Logger logs unexpected errors; nil means slog.Default().
	Logger *slog.Logger
}

// Handler serves the federation endpoints.
type Handler struct {
	o Options
}

// New returns a Handler on o.
func New(o Options) *Handler {
	if o.AccountDomain == "" {
		o.AccountDomain = o.Host
	}
	if o.Problem == nil {
		o.Problem = func(w http.ResponseWriter, _ *http.Request, status int) {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, http.StatusText(status), status)
		}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Handler{o: o}
}

// Router registers handlers for http.ServeMux patterns.
type Router interface {
	Handle(pattern string, h http.Handler)
}

// Paths that the handlers serve, besides the ADR-0018 URIs under /ap/ and /@.
const (
	PathWebFinger     = "/.well-known/webfinger"
	PathHostMeta      = "/.well-known/host-meta"
	PathHostMetaJSON  = "/.well-known/host-meta.json"
	PathNodeInfoIndex = "/.well-known/nodeinfo"
	PathNodeInfo      = "/nodeinfo/2.1"
)

// Register registers every endpoint on r.
func (h *Handler) Register(r Router) {
	r.Handle("GET "+PathWebFinger, h.wellKnown(h.webFinger))
	r.Handle("GET "+PathHostMeta, h.wellKnown(h.hostMeta))
	r.Handle("GET "+PathHostMetaJSON, h.wellKnown(h.hostMetaJSON))
	r.Handle("GET "+PathNodeInfoIndex, h.wellKnown(h.nodeInfoIndex))
	r.Handle("GET "+PathNodeInfo, http.HandlerFunc(h.nodeInfo))
	r.Handle("GET "+actorsPrefix+"{id}", http.HandlerFunc(h.actor))
	r.Handle("GET "+actorsPrefix+"{id}/{collection}", http.HandlerFunc(h.collection))
	// A wildcard cannot share a path segment with "@", so every single-segment GET that no
	// more specific pattern serves comes here; profilePage passes those that are not profile
	// pages to the HTML handler.
	r.Handle("GET /{name}", http.HandlerFunc(h.profilePage))
}

// Response headers.
const (
	cachePublic = "public, max-age=60"

	typeActivityJSON = "application/activity+json"
	typeJRD          = "application/jrd+json; charset=utf-8"
	typeXRD          = "application/xrd+xml; charset=utf-8"
	typeJSON         = "application/json; charset=utf-8"
	typeNodeInfo     = `application/json; profile="` + nodeInfoSchema + `#"`
)

// notFound answers with the one 404 response that missing and hidden resources share.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.o.Problem(w, r, http.StatusNotFound)
}

// fail answers a lookup error: 404 for ErrNotFound, otherwise 500 after logging it.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, appaccount.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	h.o.Logger.ErrorContext(r.Context(), "activitypub lookup failed", slog.Any("error", err))
	h.o.Problem(w, r, http.StatusInternalServerError)
}

// visible reports whether an actor may be presented: only active local actors are.
// Authorized fetch (FR-408) will add the requester's signature here.
func visible(a appaccount.Actor) bool {
	return a.Account.IsLocal() && a.Lifecycle.Active()
}

// findVisible looks up an actor and hides the ones that may not be presented as missing.
func findVisible(a appaccount.Actor, err error) (appaccount.Actor, error) {
	if err != nil {
		return appaccount.Actor{}, err
	}
	if !visible(a) {
		return appaccount.Actor{}, appaccount.ErrNotFound
	}
	return a, nil
}

// writeJSON writes v as a cacheable public document of the content type ct. Federation
// documents are fetched by other servers' backends and by browsers on other origins, so they
// are cross-origin resources (ADR-0019 §9).
func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, ct string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		h.o.Logger.ErrorContext(r.Context(), "activitypub encode failed", slog.Any("error", err))
		h.o.Problem(w, r, http.StatusInternalServerError)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", ct)
	hd.Set("Cache-Control", cachePublic)
	hd.Set("Cross-Origin-Resource-Policy", "cross-origin")
	_, _ = w.Write(append(body, '\n'))
}

// redirect answers with a redirect to location that may be cached like the document it
// points at.
func redirect(w http.ResponseWriter, r *http.Request, location string, status int) {
	w.Header().Set("Cache-Control", cachePublic)
	http.Redirect(w, r, location, status)
}

// wellKnown wraps a /.well-known endpoint: every response allows any origin to read it
// (RFC 7033 §5), and requests to a separate account domain are redirected to the host
// (ADR-0018).
func (h *Handler) wellKnown(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if h.o.AccountDomain != h.o.Host && strings.EqualFold(r.Host, h.o.AccountDomain) {
			// The target is always on the configured host; only the path and query are the request's.
			target := url.URL{Scheme: "https", Host: h.o.Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
			http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
			return
		}
		next(w, r)
	})
}
