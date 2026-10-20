// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
)

// The instance theme (ADR-0059, FR-605): the admin operations that read and replace it
// (manage_settings), and Instance.theme of GET /instance.

// ProblemTypeInvalidTheme is the problem type of a theme that breaks a rule (422), whose
// theme_errors list every reason.
const ProblemTypeInvalidTheme = "/api/v1/problems/invalid-theme"

// Themes runs the use cases of the instance theme. *appinstance.Service implements it.
type Themes interface {
	// Theme returns the current theme revision, and false when it changes nothing (never saved:
	// the zero revision; reset: an empty one).
	Theme(ctx context.Context) (instance.ThemeRevision, bool, error)
	// SetTheme records a new theme revision, audited; it fails with an *instance.ThemeError.
	SetTheme(ctx context.Context, in appinstance.SetThemeInput) (instance.ThemeRevision, error)
}

func themeTokenValues(tokens []instance.ThemeTokenValue) []ThemeTokenValue {
	out := make([]ThemeTokenValue, len(tokens))
	for i, v := range tokens {
		out[i] = ThemeTokenValue{Token: v.Token}
		if t, ok := instance.LookupThemeToken(v.Token); ok && t.Kind == instance.ThemeTokenColor {
			out[i].Light, out[i].Dark = &v.Light, &v.Dark
		} else {
			out[i].Value = &v.Value
		}
	}
	return out
}

// themeStylesheetURL is the URL of a revision's stylesheet, or "" without Options.ThemeStylesheetPath.
func (s *Server) themeStylesheetURL(revisionID string) string {
	if s.o.ThemeStylesheetPath == nil {
		return ""
	}
	return s.origin() + s.o.ThemeStylesheetPath(revisionID)
}

func (s *Server) adminTheme(rev instance.ThemeRevision, inEffect bool) AdminTheme {
	out := AdminTheme{Tokens: themeTokenValues(rev.Tokens), CustomCss: rev.CustomCSS}
	if rev.ID != "" {
		id, at := rev.ID, rev.RevisedAt.UTC()
		out.Revision, out.RevisedAt = &id, &at
	}
	if u := s.themeStylesheetURL(rev.ID); inEffect && u != "" {
		out.StylesheetUrl = &u
	}
	return out
}

// instanceTheme returns Instance.theme, or nil when the instance has no theme.
func (s *Server) instanceTheme(ctx context.Context) (*InstanceTheme, error) {
	if s.o.Themes == nil {
		return nil, nil
	}
	rev, ok, err := s.o.Themes.Theme(ctx)
	if err != nil {
		return nil, fmt.Errorf("clientapi: instance theme: %w", err)
	}
	u := s.themeStylesheetURL(rev.ID)
	if !ok || u == "" {
		return nil, nil
	}
	return &InstanceTheme{Revision: rev.ID, StylesheetUrl: u, Tokens: themeTokenValues(rev.Tokens)}, nil
}

// GetAdminTheme serves GET /api/v1/admin/theme.
func (s *Server) GetAdminTheme(ctx context.Context, _ GetAdminThemeRequestObject) (GetAdminThemeResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageSettings); err != nil {
		return nil, err
	} else if p != nil {
		return GetAdminThemedefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	if s.o.Themes == nil {
		return GetAdminThemedefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	rev, ok, err := s.o.Themes.Theme(ctx)
	if err != nil {
		return nil, fmt.Errorf("clientapi: admin theme: %w", err)
	}
	cache := cacheNoStore
	return GetAdminTheme200JSONResponse{AdminThemeJSONResponse{
		Body: s.adminTheme(rev, ok), Headers: AdminThemeResponseHeaders{CacheControl: &cache},
	}}, nil
}

// SetAdminTheme serves PUT /api/v1/admin/theme.
func (s *Server) SetAdminTheme(ctx context.Context, req SetAdminThemeRequestObject) (SetAdminThemeResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageSettings); err != nil {
		return nil, err
	} else if p != nil {
		return SetAdminThemedefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	if s.o.Themes == nil {
		return SetAdminThemedefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return SetAdminThemedefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "body")), nil
	}
	tokens := make([]instance.ThemeTokenValue, len(req.Body.Tokens))
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	for i, t := range req.Body.Tokens {
		tokens[i] = instance.ThemeTokenValue{Token: t.Token, Light: deref(t.Light), Dark: deref(t.Dark), Value: deref(t.Value)}
	}
	rev, err := s.o.Themes.SetTheme(ctx, appinstance.SetThemeInput{Tokens: tokens, CustomCSS: req.Body.CustomCss, RevisedByUserID: v.UserID})
	var te *instance.ThemeError
	switch {
	case errors.As(err, &te):
		return s.themeProblem(ctx, te), nil
	case err != nil:
		return nil, fmt.Errorf("clientapi: set theme: %w", err)
	}
	cache := cacheNoStore
	return SetAdminTheme200JSONResponse{AdminThemeJSONResponse{
		Body: s.adminTheme(rev, !rev.Empty()), Headers: AdminThemeResponseHeaders{CacheControl: &cache},
	}}, nil
}

func (s *Server) themeProblem(ctx context.Context, te *instance.ThemeError) SetAdminTheme422ApplicationProblemPlusJSONResponse {
	l := s.Localizer(ctx)
	body, headers := s.NewProblem(ctx, http.StatusUnprocessableEntity, ProblemTypeInvalidTheme, l.ServerErrorsInvalidTheme(), "")
	out := ThemeProblem{Type: body.Type, Title: body.Title, Status: body.Status, Detail: body.Detail, RequestId: body.RequestId}
	out.ThemeErrors = make([]ThemeError, len(te.Problems))
	num := func(f float64) *float32 { v := float32(f); return &v }
	for i, p := range te.Problems {
		e := ThemeError{Code: ThemeErrorCode(p.Code)}
		if p.Token != "" {
			e.Token = &p.Token
		}
		if p.Scheme == instance.SchemeLight || p.Scheme == instance.SchemeDark {
			scheme := ThemeErrorScheme(p.Scheme)
			e.Scheme = &scheme
		}
		switch p.Code {
		case instance.ThemeLowContrast:
			e.Background, e.Ratio, e.Minimum = &p.Background, num(p.Ratio), num(p.Minimum)
		case instance.ThemeOutOfRange:
			e.Min, e.Max = num(p.Min), num(p.Max)
		case instance.ThemeCSSTooLong:
			e.Max = num(p.Max)
		}
		out.ThemeErrors[i] = e
	}
	return SetAdminTheme422ApplicationProblemPlusJSONResponse{
		Body:    out,
		Headers: SetAdminTheme422ResponseHeaders{CacheControl: headers.CacheControl, ContentLanguage: headers.ContentLanguage},
	}
}
