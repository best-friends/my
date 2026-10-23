// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mycroblog/my/internal/domain/content"
)

// AutoDeletePolicies reads and replaces the viewer's automatic deletion of their old posts
// (FR-118); *content.AutoDelete implements it.
type AutoDeletePolicies interface {
	AutoDeletePolicy(ctx context.Context, accountID string) (content.AutoDeletePolicy, error)
	// UpdateAutoDeletePolicy fails with a *content.AutoDeletePolicyError for invalid input.
	UpdateAutoDeletePolicy(ctx context.Context, accountID string, p content.AutoDeletePolicy) (content.AutoDeletePolicy, error)
}

func newAutoDeletePolicy(p content.AutoDeletePolicy) AutoDeletePolicy {
	kept := make([]Visibility, 0, len(p.KeepVisibilities))
	for _, v := range p.KeepVisibilities {
		kept = append(kept, Visibility(v))
	}
	return AutoDeletePolicy{
		Enabled: p.Enabled, MinAgeDays: p.MinAgeDays,
		KeepPinned: p.KeepPinned, KeepBookmarked: p.KeepBookmarked, KeepFavourited: p.KeepFavourited,
		KeepMedia: p.KeepMedia, KeepPolls: p.KeepPolls, KeepVisibilities: kept,
		KeepMinBoosts: p.KeepMinBoosts, KeepMinFavourites: p.KeepMinFavourites,
	}
}

// GetAutoDeletePolicy serves GET /api/v1/preferences/auto_delete.
func (s *Server) GetAutoDeletePolicy(ctx context.Context, _ GetAutoDeletePolicyRequestObject) (GetAutoDeletePolicyResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.AutoDelete == nil {
		body, headers := s.notFound(ctx)
		return GetAutoDeletePolicydefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
	}
	p, err := s.o.AutoDelete.AutoDeletePolicy(ctx, v.AccountID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the auto delete policy of %s: %w", v.AccountID, err)
	}
	cache := cacheNoStore
	return GetAutoDeletePolicy200JSONResponse{Body: newAutoDeletePolicy(p), Headers: GetAutoDeletePolicy200ResponseHeaders{CacheControl: &cache}}, nil
}

// UpdateAutoDeletePolicy serves PUT /api/v1/preferences/auto_delete.
func (s *Server) UpdateAutoDeletePolicy(ctx context.Context, req UpdateAutoDeletePolicyRequestObject) (UpdateAutoDeletePolicyResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.AutoDelete == nil {
		body, headers := s.notFound(ctx)
		return UpdateAutoDeletePolicydefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
	}
	if req.Body == nil {
		return UpdateAutoDeletePolicydefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "body")), nil
	}
	b := req.Body
	in := content.AutoDeletePolicy{
		Enabled: b.Enabled, MinAgeDays: b.MinAgeDays,
		KeepPinned: b.KeepPinned, KeepBookmarked: b.KeepBookmarked, KeepFavourited: b.KeepFavourited,
		KeepMedia: b.KeepMedia, KeepPolls: b.KeepPolls,
		KeepMinBoosts: b.KeepMinBoosts, KeepMinFavourites: b.KeepMinFavourites,
	}
	for _, vis := range b.KeepVisibilities {
		in.KeepVisibilities = append(in.KeepVisibilities, content.Visibility(vis))
	}
	p, err := s.o.AutoDelete.UpdateAutoDeletePolicy(ctx, v.AccountID, in)
	var invalid *content.AutoDeletePolicyError
	if errors.As(err, &invalid) {
		l := s.Localizer(ctx)
		body, headers := s.NewProblem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), l.ServerErrorsInvalidValue(invalid.Property))
		return UpdateAutoDeletePolicydefaultApplicationProblemPlusJSONResponse{
			Body: body, Headers: headers, StatusCode: http.StatusUnprocessableEntity,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: update the auto delete policy of %s: %w", v.AccountID, err)
	}
	cache := cacheNoStore
	return UpdateAutoDeletePolicy200JSONResponse{Body: newAutoDeletePolicy(p), Headers: UpdateAutoDeletePolicy200ResponseHeaders{CacheControl: &cache}}, nil
}
