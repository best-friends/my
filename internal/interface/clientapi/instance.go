// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/domain/instance"
)

// InstanceInfo reads the instance settings and texts. *appinstance.Service implements it.
type InstanceInfo interface {
	// Settings returns the current value of every setting, defaults included.
	Settings(ctx context.Context) (instance.Settings, error)
	// Text returns the text of key for a reader of locale along the locale fallback chain,
	// and false when no locale of the chain has it.
	Text(ctx context.Context, key instance.TextKey, locale string) (instance.Text, bool, error)
}

// cachePublic is the Cache-Control of anonymous public GETs, which shared caches may store
// (ADR-0016).
const cachePublic = "public, max-age=60"

// cachePrivate is the Cache-Control of public resources fetched with an access token, which
// depend on the viewer's locale.
const cachePrivate = "private, max-age=60"

// GetInstance serves GET /api/v1/instance. The texts and rules are in the locale of the request
// (ADR-0009 §2): the response varies with Accept-Language (localize) for anonymous requests
// and is private for authenticated ones, whose locale is the user's.
func (s *Server) GetInstance(ctx context.Context, _ GetInstanceRequestObject) (GetInstanceResponseObject, error) {
	var settings instance.Settings
	out := Instance{Domain: s.o.Host, Name: s.o.Host, Version: s.o.SoftwareVersion, SourceUrl: s.o.SourceURL}
	locale := s.Localizer(ctx).RequestedLocale()
	if s.o.Instance != nil {
		var err error
		if settings, err = s.o.Instance.Settings(ctx); err != nil {
			return nil, fmt.Errorf("clientapi: instance settings: %w", err)
		}
		for _, t := range []struct {
			key instance.TextKey
			set func(string)
		}{
			{instance.TextName, func(v string) { out.Name = v }},
			{instance.TextShortDescription, func(v string) { out.ShortDescription = &v }},
			{instance.TextDescription, func(v string) { out.Description = &v }},
		} {
			text, ok, err := s.o.Instance.Text(ctx, t.key, locale)
			if err != nil {
				return nil, fmt.Errorf("clientapi: instance text %s: %w", t.key, err)
			}
			if ok {
				t.set(text.Body)
			}
		}
		if err := s.documentURLs(ctx, &out, locale); err != nil {
			return nil, err
		}
	}
	out.Registrations = InstanceRegistrations{Mode: InstanceRegistrationsMode(settings.RegistrationMode())}
	out.Limits = InstanceLimits{MaxPostCharacters: settings.MaxStatusLength()}
	out.Timelines = InstanceTimelines{Local: settings.LocalTimelineEnabled(), Federated: settings.FederatedTimelineEnabled()}
	out.SendsEmail = s.o.SendsEmail
	if key := s.pushServerKey(); key != "" {
		out.PushServerKey = &key
	}
	rules, err := s.instanceRules(ctx, locale)
	if err != nil {
		return nil, err
	}
	out.Rules = rules
	if out.Theme, err = s.instanceTheme(ctx); err != nil {
		return nil, err
	}

	cache := cachePublic
	if _, ok := ViewerFrom(ctx); ok {
		cache = cachePrivate
	}
	return GetInstance200JSONResponse{Body: out, Headers: GetInstance200ResponseHeaders{CacheControl: &cache}}, nil
}
