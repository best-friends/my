// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
)

// UserPreferences reads and changes the viewer's preferences (FR-007, FR-208).
// *appidentity.Service implements it.
type UserPreferences interface {
	Preferences(ctx context.Context, userID string) (appidentity.UserPreferences, error)
	// UpdatePreferences fails with a *appidentity.PreferenceError for invalid input.
	UpdatePreferences(ctx context.Context, userID string, in appidentity.PreferencesUpdate) (appidentity.UserPreferences, error)
}

func newPreferences(p appidentity.UserPreferences) Preferences {
	langs := p.TimelineLanguages
	if langs == nil {
		langs = []string{}
	}
	return Preferences{
		Locale: p.Locale, TimeZone: p.TimeZone, TimeZoneChosen: p.TimeZoneChosen, DefaultVisibility: Visibility(p.DefaultVisibility),
		DefaultPostLanguage: p.DefaultPostLanguage, RequireMediaDescription: p.RequireMediaDescription,
		EmailModeratorNotifications: p.EmailModeratorNotifications, TimelineLanguages: langs,
	}
}

// GetPreferences serves GET /api/v1/preferences.
func (s *Server) GetPreferences(ctx context.Context, _ GetPreferencesRequestObject) (GetPreferencesResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.o.Preferences.Preferences(ctx, v.UserID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the preferences of %s: %w", v.UserID, err)
	}
	cache := cacheNoStore
	return GetPreferences200JSONResponse{Body: newPreferences(p), Headers: GetPreferences200ResponseHeaders{CacheControl: &cache}}, nil
}

// UpdatePreferences serves PATCH /api/v1/preferences.
func (s *Server) UpdatePreferences(ctx context.Context, req UpdatePreferencesRequestObject) (UpdatePreferencesResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	var in appidentity.PreferencesUpdate
	if b := req.Body; b != nil {
		in = appidentity.PreferencesUpdate{
			Locale: b.Locale, TimeZone: b.TimeZone, DefaultPostLanguage: b.DefaultPostLanguage,
			RequireMediaDescription: b.RequireMediaDescription, TimelineLanguages: b.TimelineLanguages,
			EmailModeratorNotifications: b.EmailModeratorNotifications,
		}
		if b.DefaultVisibility != nil {
			vis := identity.Visibility(*b.DefaultVisibility)
			in.DefaultVisibility = &vis
		}
	}
	p, err := s.o.Preferences.UpdatePreferences(ctx, v.UserID, in)
	var invalid *appidentity.PreferenceError
	if errors.As(err, &invalid) {
		l := s.Localizer(ctx)
		detail := l.ServerErrorsInvalidValue(invalid.Preference)
		if errors.Is(invalid.Err, identity.ErrTooManyLanguages) {
			detail = l.ServerErrorsTooMany(identity.MaxTimelineLanguageCount, invalid.Preference)
		}
		body, headers := s.NewProblem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), detail)
		return UpdatePreferencesdefaultApplicationProblemPlusJSONResponse{
			Body: body, Headers: headers, StatusCode: http.StatusUnprocessableEntity,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: update the preferences of %s: %w", v.UserID, err)
	}
	cache := cacheNoStore
	return UpdatePreferences200JSONResponse{Body: newPreferences(p), Headers: UpdatePreferences200ResponseHeaders{CacheControl: &cache}}, nil
}
