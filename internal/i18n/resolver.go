// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"context"
	"net/http"
)

// Resolver resolves the locale of each request in the order of ADR-0009 §2: the user's
// saved locale, then the request's Accept-Language, then the instance's default locale,
// then SourceLocale. Every HTTP surface (client API, sign-in screens, HTML shell, platform
// problem documents) resolves through one Resolver so that they agree. It is safe for
// concurrent use when its functions are.
type Resolver struct {
	// Bundle holds the catalogs that a resolved locale must have.
	Bundle *Bundle
	// UserLocale returns the saved locale of the user that the request in ctx is
	// authenticated as, or "" for an anonymous request. nil means every request is anonymous.
	UserLocale func(ctx context.Context) string
	// InstanceDefault returns the instance's current default locale, or "" when it cannot be
	// read. It is called per resolution, and only when neither the user's locale nor
	// Accept-Language matches a catalog, so a changed setting applies without a restart.
	// nil means SourceLocale.
	InstanceDefault func(ctx context.Context) string
}

// Locale returns the locale for a request in ctx with the Accept-Language header value
// acceptLanguage.
func (r *Resolver) Locale(ctx context.Context, acceptLanguage string) string {
	var user string
	if r.UserLocale != nil {
		user = r.UserLocale(ctx)
	}
	if l, ok := r.Bundle.match(append([]string{user}, ParseAcceptLanguage(acceptLanguage)...)); ok {
		return l
	}
	if r.InstanceDefault != nil {
		if l, ok := r.Bundle.match([]string{r.InstanceDefault(ctx)}); ok {
			return l
		}
	}
	return SourceLocale
}

// Localizer returns the Localizer of Locale(ctx, acceptLanguage).
func (r *Resolver) Localizer(ctx context.Context, acceptLanguage string) *Localizer {
	return r.Bundle.Localizer(r.Locale(ctx, acceptLanguage))
}

// RequestLocalizer returns the Localizer of req, from its context and Accept-Language header.
func (r *Resolver) RequestLocalizer(req *http.Request) *Localizer {
	return r.Localizer(req.Context(), req.Header.Get("Accept-Language"))
}
