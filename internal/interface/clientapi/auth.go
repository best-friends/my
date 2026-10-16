// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
)

// Authentication and authorization of the client API (ADR-0019 §4, docs/architecture/client-api.md §4).
//
// Register wraps every operation's handler in guard, which runs before the generated code
// parses the request:
//
//  1. Authentication reads only the Authorization header, never a cookie. Without one the
//     request is anonymous; with one it must be a valid Bearer access token, whose Viewer goes
//     into the context. A credential that is sent is always checked, also for operations that
//     allow anonymous requests: an invalid token is a 401, never an anonymous request.
//  2. Rate limiting takes the request from the account's limit, or from the client IP's limit
//     when there is no valid token.
//  3. Authorization checks the operation's security from security.gen.go, which `make generate`
//     derives from the security requirements of api/openapi.yaml: 401 without a token when the
//     operation needs one, 403 insufficient_scope when the token does not grant every scope.
//  4. The request body is bounded (limitBody in media.go): a declared length over the
//     operation's limit is a 413 before any of the body is read.

// Problem types of authentication and authorization; api/openapi.yaml documents them.
const (
	ProblemTypeUnauthenticated    = "/api/v1/problems/unauthenticated"
	ProblemTypeInvalidToken       = "/api/v1/problems/invalid-token"
	ProblemTypeInsufficientScope  = "/api/v1/problems/insufficient-scope"
	ProblemTypeAccountUnavailable = "/api/v1/problems/account-unavailable"
)

// Authenticator authenticates the bearer access tokens of client API requests.
type Authenticator interface {
	// AuthenticateBearer fails with appidentity.ErrInvalidAccessToken for a token that is
	// unknown, revoked, or expired, and with appidentity.ErrAccountUnavailable for a valid
	// token whose user cannot use the app.
	AuthenticateBearer(ctx context.Context, accessToken string) (appidentity.Bearer, error)
}

// noTokens is the Authenticator of a Server without one: every token is invalid.
type noTokens struct{}

func (noTokens) AuthenticateBearer(context.Context, string) (appidentity.Bearer, error) {
	return appidentity.Bearer{}, appidentity.ErrInvalidAccessToken
}

// Viewer is who an authenticated request acts as. Handlers of operations that need an access
// token get it with requireViewer; those that also serve anonymous requests use ViewerFrom.
type Viewer struct {
	UserID string
	// AccountID is the local account the user logs in as.
	AccountID string
	// TokenID and ClientID identify the access token and its OAuth client (the row IDs).
	TokenID  string
	ClientID string
	// Scope is the token's scope. The operation's scopes are checked before its handler runs;
	// a handler checks Scope.Grants itself only for what depends on more than the operation.
	Scope identity.Scope
}

type viewerKey struct{}

// WithViewer returns ctx with v as its Viewer.
func WithViewer(ctx context.Context, v Viewer) context.Context {
	return context.WithValue(ctx, viewerKey{}, v)
}

// ViewerFrom returns the Viewer of the request that ctx belongs to; ok is false for an
// anonymous request. The platform's i18n.Resolver finds the user's locale through it.
func ViewerFrom(ctx context.Context) (v Viewer, ok bool) {
	v, ok = ctx.Value(viewerKey{}).(Viewer)
	return v, ok
}

// errNoViewer reports a handler that needs a Viewer running without one, which means that its
// operation allows anonymous requests in the spec but the handler does not serve them.
var errNoViewer = errors.New("clientapi: the operation needs an authenticated viewer")

// requireViewer returns the Viewer of an operation that needs an access token.
func requireViewer(ctx context.Context) (Viewer, error) {
	v, ok := ViewerFrom(ctx)
	if !ok {
		return Viewer{}, errNoViewer
	}
	return v, nil
}

// operationAccess is the security of one operation (security.gen.go).
type operationAccess struct {
	// Operation is the operationId.
	Operation string
	// Anonymous operations serve requests without an access token.
	Anonymous bool
	// Scopes must all be granted by the access token of an authenticated request.
	Scopes []string
}

// bearerToken parses the Authorization header (RFC 6750 §2.1). present is false when there
// is none; ok is false when it is not exactly one "Bearer <token>".
func bearerToken(h http.Header) (token string, present, ok bool) {
	values := h.Values("Authorization")
	if len(values) == 0 {
		return "", false, false
	}
	if len(values) > 1 {
		return "", true, false
	}
	scheme, token, found := strings.Cut(values[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t") {
		return "", true, false
	}
	return token, true, true
}

// guard authenticates, rate-limits, and authorizes the requests of the operation registered
// with pattern before next parses them. It panics when the operation has no security
// metadata, so that a stale security.gen.go fails at start-up instead of serving the
// operation unguarded.
func (s *Server) guard(pattern string, next http.Handler) http.Handler {
	a, ok := operationSecurity[pattern]
	if !ok {
		panic(fmt.Sprintf("clientapi: %s has no security metadata; run make generate", pattern))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(withLocale(r))
		v, ok := s.authenticate(w, r)
		if !ok {
			return
		}
		if v == nil {
			ok = s.limit(w, r, policyIP, s.o.AnonymousLimit, s.clientKey(r))
		} else {
			// The Viewer goes in first, so that every answer from here on is in the user's locale.
			r = r.WithContext(WithViewer(r.Context(), *v))
			ok = s.limit(w, r, policyAccount, s.o.AccountLimit, v.AccountID)
		}
		if ok && s.authorize(w, r, a, v) && s.limitBody(w, r, pattern) {
			next.ServeHTTP(w, r)
		}
	})
}

// authenticate returns the Viewer of the request's access token, or nil for a request
// without an Authorization header. It returns false when it has answered the request: an
// invalid credential or an unusable account, which count against the client IP's rate limit
// like anonymous requests.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*Viewer, bool) {
	token, present, ok := bearerToken(r.Header)
	if !present {
		return nil, true
	}
	err := appidentity.ErrInvalidAccessToken
	var b appidentity.Bearer
	if ok {
		b, err = s.o.Authenticator.AuthenticateBearer(r.Context(), token)
	}
	switch {
	case err == nil:
		return &Viewer{UserID: b.UserID, AccountID: b.AccountID, TokenID: b.TokenID, ClientID: b.ClientID, Scope: b.Scope}, true
	case errors.Is(err, appidentity.ErrInvalidAccessToken):
		if s.limit(w, r, policyIP, s.o.AnonymousLimit, s.clientKey(r)) {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			s.writeProblem(w, r, http.StatusUnauthorized, ProblemTypeInvalidToken, s.localizer(r).ServerErrorsInvalidToken(), "")
		}
	case errors.Is(err, appidentity.ErrAccountUnavailable):
		if s.limit(w, r, policyIP, s.o.AnonymousLimit, s.clientKey(r)) {
			s.writeProblem(w, r, http.StatusForbidden, ProblemTypeAccountUnavailable, s.localizer(r).ServerErrorsAccountUnavailable(), "")
		}
	default:
		s.responseError(w, r, err)
	}
	return nil, false
}

// authorize reports whether the request may run an operation with access a, answering it
// with a 401 or 403 problem when it may not. v is nil for an anonymous request.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request, a operationAccess, v *Viewer) bool {
	if v == nil {
		if a.Anonymous {
			return true
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		s.writeProblem(w, r, http.StatusUnauthorized, ProblemTypeUnauthenticated, s.localizer(r).ServerErrorsUnauthenticated(), "")
		return false
	}
	for _, scope := range a.Scopes {
		if v.Scope.Grants(scope) {
			continue
		}
		l := s.localizer(r)
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="insufficient_scope", scope="%s"`, strings.Join(a.Scopes, " ")))
		s.writeProblem(w, r, http.StatusForbidden, ProblemTypeInsufficientScope,
			l.ServerErrorsInsufficientScope(), l.ServerErrorsInsufficientScopeDetail(scope))
		return false
	}
	return true
}
