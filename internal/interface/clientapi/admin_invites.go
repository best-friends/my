// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	appidentity "github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/domain/identity"
)

// The invites (FR-001, manage_invites): the admin operations that create, list, and revoke the
// codes that let people sign up while the registration mode is invite. The CLI's `my invite`
// runs the same use cases.

// Invites runs the use cases of the invites. *appidentity.Service implements it.
type Invites interface {
	ListInvites(ctx context.Context, before string, limit int) ([]appidentity.InviteStatus, error)
	CreateInvite(ctx context.Context, in appidentity.CreateInviteInput) (identity.Invite, error)
	RevokeInviteByID(ctx context.Context, id, revokedByUserID string) (appidentity.InviteStatus, error)
}

// maxInviteExpiry bounds CreateInviteRequest.expires_in: ten years.
const maxInviteExpiry = 10 * 365 * 24 * time.Hour

func (s *Server) newAdminInvite(st appidentity.InviteStatus) AdminInvite {
	out := AdminInvite{
		Id: st.ID, Code: st.Code, Url: s.origin() + s.o.SignUpPath + "?invite=" + url.QueryEscape(st.Code),
		Uses: st.Uses, State: AdminInviteState(st.State), CreatedAt: st.CreatedAt.UTC(),
	}
	if st.MaxUses > 0 {
		out.MaxUses = &st.MaxUses
	}
	if st.ExpiresAt != nil {
		t := st.ExpiresAt.UTC()
		out.ExpiresAt = &t
	}
	return out
}

func (s *Server) adminInviteResponse(st appidentity.InviteStatus) AdminInviteJSONResponse {
	cache := cacheNoStore
	return AdminInviteJSONResponse{Body: s.newAdminInvite(st), Headers: AdminInviteResponseHeaders{CacheControl: &cache}}
}

// ListInvites serves GET /api/v1/admin/invites, newest first. The cursor is the ID of the last
// invite of the previous page.
func (s *Server) ListInvites(ctx context.Context, req ListInvitesRequestObject) (ListInvitesResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageInvites); err != nil {
		return nil, err
	} else if p != nil {
		return ListInvitesdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	q := s.parsePage(ctx, req.Params.Cursor, req.Params.Limit)
	if q.problem != nil {
		return ListInvitesdefaultApplicationProblemPlusJSONResponse(*q.problem), nil
	}
	list, err := s.o.Invites.ListInvites(ctx, q.before, q.limit+1)
	if err != nil {
		return nil, fmt.Errorf("clientapi: list invites: %w", err)
	}
	list, last := trim(list, q.limit)
	page := AdminInvitePage{Items: make([]AdminInvite, 0, len(list))}
	for _, st := range list {
		page.Items = append(page.Items, s.newAdminInvite(st))
	}
	if last >= 0 {
		page.NextCursor = encodeCursor(list[last].ID)
	}
	cache := cacheNoStore
	return ListInvites200JSONResponse{Body: page, Headers: ListInvites200ResponseHeaders{CacheControl: &cache}}, nil
}

// CreateInvite serves POST /api/v1/admin/invites.
func (s *Server) CreateInvite(ctx context.Context, req CreateInviteRequestObject) (CreateInviteResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageInvites); err != nil {
		return nil, err
	} else if p != nil {
		return CreateInvitedefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	l := s.Localizer(ctx)
	unprocessable := func(name string) CreateInviteResponseObject {
		return CreateInvitedefaultApplicationProblemPlusJSONResponse(
			s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), l.ServerErrorsInvalidValue(name)))
	}
	in := appidentity.CreateInviteInput{CreatedByUserID: v.UserID}
	if b := req.Body; b != nil {
		if b.MaxUses != nil {
			if *b.MaxUses < 1 || *b.MaxUses > appidentity.MaxInviteUses {
				return unprocessable("max_uses"), nil
			}
			in.MaxUses = *b.MaxUses
		}
		if b.ExpiresIn != nil {
			if *b.ExpiresIn < 1 || *b.ExpiresIn > int(maxInviteExpiry/time.Second) {
				return unprocessable("expires_in"), nil
			}
			in.ExpiresIn = time.Duration(*b.ExpiresIn) * time.Second
		}
	}
	inv, err := s.o.Invites.CreateInvite(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("clientapi: create invite: %w", err)
	}
	return CreateInvite200JSONResponse{s.adminInviteResponse(appidentity.InviteStatus{Invite: inv, State: appidentity.InviteUsable})}, nil
}

// RevokeInvite serves POST /api/v1/admin/invites/{id}/revoke.
func (s *Server) RevokeInvite(ctx context.Context, req RevokeInviteRequestObject) (RevokeInviteResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageInvites); err != nil {
		return nil, err
	} else if p != nil {
		return RevokeInvitedefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.o.Invites.RevokeInviteByID(ctx, req.Id, v.UserID)
	switch {
	case errors.Is(err, appidentity.ErrNotFound):
		return RevokeInvitedefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	case err != nil:
		return nil, fmt.Errorf("clientapi: revoke invite %s: %w", req.Id, err)
	}
	return RevokeInvite200JSONResponse{s.adminInviteResponse(st)}, nil
}
