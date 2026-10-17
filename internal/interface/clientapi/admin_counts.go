// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"fmt"
	"slices"

	"github.com/best-friends/my/internal/domain/identity"
)

// ReportCounts counts the open reports. *appmoderation.Service implements it.
type ReportCounts interface {
	CountOpenReports(ctx context.Context) (int64, error)
}

// RegistrationCounts counts the registrations that await approval. *appidentity.Service
// implements it.
type RegistrationCounts interface {
	CountPendingRegistrations(ctx context.Context) (int64, error)
}

// GetAdminCounts serves GET /api/v1/admin/counts: what awaits the moderators (FR-712). Each
// count needs its own permission, and a count that the viewer's roles do not allow is omitted
// rather than refused, so a single request fills the badges of whatever admin menu the viewer
// has.
func (s *Server) GetAdminCounts(ctx context.Context, _ GetAdminCountsRequestObject) (GetAdminCountsResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	var perms []identity.Permission
	if s.o.Permissions != nil {
		if perms, err = s.o.Permissions.Permissions(ctx, v.UserID); err != nil {
			return nil, fmt.Errorf("clientapi: permissions of %s: %w", v.UserID, err)
		}
	}
	var body AdminCounts
	if slices.Contains(perms, identity.PermManageReports) && s.o.ReportCounts != nil {
		n, err := s.o.ReportCounts.CountOpenReports(ctx)
		if err != nil {
			return nil, fmt.Errorf("clientapi: count the open reports: %w", err)
		}
		body.OpenReports = &n
	}
	if slices.Contains(perms, identity.PermManageAccounts) && s.o.RegistrationCounts != nil {
		n, err := s.o.RegistrationCounts.CountPendingRegistrations(ctx)
		if err != nil {
			return nil, fmt.Errorf("clientapi: count the pending registrations: %w", err)
		}
		body.PendingRegistrations = &n
	}
	cache := cacheNoStore
	return GetAdminCounts200JSONResponse{Body: body, Headers: GetAdminCounts200ResponseHeaders{CacheControl: &cache}}, nil
}
