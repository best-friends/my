// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/i18n"
)

// BasePath is the path prefix of every client API endpoint, the server URL of api/openapi.yaml.
const BasePath = "/api/v1"

// Options configure a Server.
type Options struct {
	// Host is the domain that serves the instance.
	Host string
	// Name is the display name of the instance.
	Name string
	// SoftwareVersion is the server version.
	SoftwareVersion string
	// SourceURL is where users get the server's source code (ADR-0011).
	SourceURL string
	// MaxPostCharacters is the maximum number of characters in the text of a post.
	MaxPostCharacters int

	// Authenticator authenticates bearer access tokens; nil accepts none.
	Authenticator Authenticator
	// Accounts reads accounts.
	Accounts Accounts
	// RateLimiter applies AnonymousLimit and AccountLimit; nil applies no limit.
	RateLimiter *kernel.RateLimiter
	// AnonymousLimit and AccountLimit override the package's limits of the same names when
	// their Max is set.
	AnonymousLimit kernel.Limit
	AccountLimit   kernel.Limit
	// ClientIP returns the client IP of a request for its rate limit; nil means the
	// connection's peer.
	ClientIP func(r *http.Request) string

	// Locales resolves the locale of problem documents (ADR-0009 §2). Its UserLocale step
	// finds the authenticated user with ViewerFrom.
	Locales *i18n.Resolver
	// RequestID returns the ID of the request in ctx, echoed in problem documents; nil omits it.
	RequestID func(ctx context.Context) string
	// Logger logs unexpected errors; nil means slog.Default().
	Logger *slog.Logger
}

// Server implements the generated StrictServerInterface by calling application use cases.
type Server struct {
	o Options
}

var _ StrictServerInterface = (*Server)(nil)

// New returns a Server on o.
func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.RequestID == nil {
		o.RequestID = func(context.Context) string { return "" }
	}
	if o.AnonymousLimit.Max == 0 {
		o.AnonymousLimit = AnonymousLimit
	}
	if o.AccountLimit.Max == 0 {
		o.AccountLimit = AccountLimit
	}
	if o.ClientIP == nil {
		o.ClientIP = remoteIP
	}
	if o.Authenticator == nil {
		o.Authenticator = noTokens{}
	}
	return &Server{o: o}
}

// Router registers handlers for http.ServeMux patterns.
type Router interface {
	Handle(pattern string, h http.Handler)
}

// Register registers every operation of the spec on r under BasePath, behind authentication
// and rate limiting, with the security of each operation enforced (auth.go). Requests that
// match no operation are left to r, which answers them with its own 404 or 405 problem.
func (s *Server) Register(r Router) {
	strict := NewStrictHandlerWithOptions(s, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  s.requestError,
		ResponseErrorHandlerFunc: s.responseError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL:          BasePath,
		BaseRouter:       routerMux{r: r, wrap: s.guard},
		Middlewares:      []MiddlewareFunc{s.localize},
		ErrorHandlerFunc: s.requestError,
	})
}

// routerMux adapts a Router to the ServeMux that the generated code registers on, wrapping
// every operation's handler by its pattern.
type routerMux struct {
	r    Router
	wrap func(pattern string, h http.Handler) http.Handler
}

func (m routerMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	m.r.Handle(pattern, m.wrap(pattern, http.HandlerFunc(h)))
}

// ServeHTTP is never called: the Router serves the registered handlers.
func (routerMux) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

// cachePublic is the Cache-Control of anonymous public GETs, which shared caches may store
// (ADR-0016).
const cachePublic = "public, max-age=60"

// GetInstance serves GET /api/v1/instance.
func (s *Server) GetInstance(context.Context, GetInstanceRequestObject) (GetInstanceResponseObject, error) {
	cache := cachePublic
	return GetInstance200JSONResponse{
		Body: Instance{
			Domain:    s.o.Host,
			Name:      s.o.Name,
			Version:   s.o.SoftwareVersion,
			SourceUrl: s.o.SourceURL,
			Limits:    InstanceLimits{MaxPostCharacters: s.o.MaxPostCharacters},
		},
		Headers: GetInstance200ResponseHeaders{CacheControl: &cache},
	}, nil
}
