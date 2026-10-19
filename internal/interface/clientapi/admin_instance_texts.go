// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
)

// The instance texts (manage_settings): the display name of the instance, its descriptions, its
// terms and privacy policy, and the alt text of its preview image, each in any locale. GET
// /instance reads them in the reader's locale (instance.go).

// InstanceTexts runs the use cases of the instance texts. *appinstance.Service implements it.
type InstanceTexts interface {
	// Texts returns every current, non-empty text, by key and locale.
	Texts(ctx context.Context) ([]instance.Text, error)
	// SetText records a new revision of a text in one locale, audited.
	SetText(ctx context.Context, in appinstance.SetTextInput) (instance.TextRevision, error)
}

func newAdminInstanceText(key instance.TextKey, locale, body string, at time.Time) AdminInstanceText {
	return AdminInstanceText{Key: AdminInstanceTextKey(key), Locale: locale, Text: body, RevisedAt: at.UTC()}
}

// ListInstanceTexts serves GET /api/v1/admin/instance/texts.
func (s *Server) ListInstanceTexts(ctx context.Context, _ ListInstanceTextsRequestObject) (ListInstanceTextsResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageSettings); err != nil {
		return nil, err
	} else if p != nil {
		return ListInstanceTextsdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	texts, err := s.o.Texts.Texts(ctx)
	if err != nil {
		return nil, fmt.Errorf("clientapi: list instance texts: %w", err)
	}
	list := AdminInstanceTextList{Items: make([]AdminInstanceText, 0, len(texts))}
	for _, t := range texts {
		list.Items = append(list.Items, newAdminInstanceText(t.Key, t.Locale, t.Body, t.RevisedAt))
	}
	cache := cacheNoStore
	return ListInstanceTexts200JSONResponse{Body: list, Headers: ListInstanceTexts200ResponseHeaders{CacheControl: &cache}}, nil
}

// SetInstanceText serves PUT /api/v1/admin/instance/texts/{key}/{locale}.
func (s *Server) SetInstanceText(ctx context.Context, req SetInstanceTextRequestObject) (SetInstanceTextResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageSettings); err != nil {
		return nil, err
	} else if p != nil {
		return SetInstanceTextdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return SetInstanceTextdefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "text")), nil
	}
	key := instance.TextKey(req.Key)
	def, err := instance.LookupText(key)
	if err != nil {
		return SetInstanceTextdefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	l := s.Localizer(ctx)
	unprocessable := func(detail string) SetInstanceTextResponseObject {
		return SetInstanceTextdefaultApplicationProblemPlusJSONResponse(s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), detail))
	}
	// The text is checked here, so that the use case's refusal of a valid text names the locale.
	if err := def.Validate(req.Body.Text); err != nil {
		if utf8.RuneCountInString(req.Body.Text) > def.MaxLength {
			return unprocessable(l.ServerErrorsTooLong(def.MaxLength, "text")), nil
		}
		return unprocessable(l.ServerErrorsInvalidValue("text")), nil
	}
	rev, err := s.o.Texts.SetText(ctx, appinstance.SetTextInput{Key: key, Locale: req.Locale, Body: req.Body.Text, RevisedByUserID: v.UserID})
	if errors.Is(err, instance.ErrInvalidValue) {
		return unprocessable(l.ServerErrorsInvalidValue("locale")), nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: set instance text %s [%s]: %w", key, req.Locale, err)
	}
	cache := cacheNoStore
	return SetInstanceText200JSONResponse{
		Body:    newAdminInstanceText(rev.Key, rev.Locale, rev.Body, rev.RevisedAt),
		Headers: SetInstanceText200ResponseHeaders{CacheControl: &cache},
	}, nil
}
