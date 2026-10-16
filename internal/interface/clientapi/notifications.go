// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appaccount "github.com/best-friends/my/internal/app/account"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/domain/notification"
)

// Notifications runs the use cases of the viewer's own notifications (FR-205).
// *appnotification.Service implements it.
type Notifications interface {
	List(ctx context.Context, accountID string, q appnotification.Query) ([]notification.Notification, error)
	ReadMarker(ctx context.Context, accountID string) (notification.ReadMarker, error)
	UnreadCount(ctx context.Context, accountID string) (int64, error)
	MarkRead(ctx context.Context, accountID, lastReadID string) (notification.ReadMarker, error)
	Dismiss(ctx context.Context, accountID, notificationID string) error
}

// Page sizes of collections: the default and maximum of the spec's Limit parameter.
const (
	defaultPageLimit = 20
	maxPageLimit     = 40
)

// pageLimit returns the page size that the limit parameter asks for, or false when it is out of
// the spec's range (the generated code does not check it).
func pageLimit(limit *Limit) (int, bool) {
	if limit == nil {
		return defaultPageLimit, true
	}
	return *limit, *limit >= 1 && *limit <= maxPageLimit
}

// invalidParameter returns the 400 problem of a parameter whose value the handler rejects.
func (s *Server) invalidParameter(ctx context.Context, name string) (Problem, ProblemResponseHeaders) {
	l := s.Localizer(ctx)
	return s.NewProblem(ctx, http.StatusBadRequest, ProblemTypeBlank, l.ServerErrorsBadRequest(), l.ServerErrorsInvalidParameter(name))
}

// notFound returns the 404 problem of a resource that does not exist or is not the viewer's.
func (s *Server) notFound(ctx context.Context) (Problem, ProblemResponseHeaders) {
	return s.NewProblem(ctx, http.StatusNotFound, ProblemTypeBlank, s.Localizer(ctx).ServerErrorsNotFound(), "")
}

// ListNotifications serves GET /api/v1/notifications: a page of the viewer's notifications,
// newest first. The cursor is the ID of the last notification of the previous page.
func (s *Server) ListNotifications(ctx context.Context, req ListNotificationsRequestObject) (ListNotificationsResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	limit, ok := pageLimit(req.Params.Limit)
	if !ok {
		body, headers := s.invalidParameter(ctx, "limit")
		return ListNotificationsdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusBadRequest}, nil
	}
	q := appnotification.Query{Limit: limit + 1} // one more tells whether a next page exists
	if req.Params.Cursor != nil {
		q.Before = *req.Params.Cursor
	}
	if req.Params.Types != nil {
		for _, t := range *req.Params.Types {
			q.Types = append(q.Types, notification.Type(t))
		}
	}
	ns, err := s.o.Notifications.List(ctx, v.AccountID, q)
	if errors.Is(err, notification.ErrInvalidType) {
		body, headers := s.invalidParameter(ctx, "types")
		return ListNotificationsdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusBadRequest}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: list notifications of %s: %w", v.AccountID, err)
	}
	page := NotificationPage{Items: []Notification{}}
	if len(ns) > limit {
		ns = ns[:limit]
		next := ns[limit-1].ID
		page.NextCursor = &next
	}
	accounts := map[string]*Account{}
	for _, n := range ns {
		item, err := s.newNotification(ctx, n, accounts)
		if err != nil {
			return nil, err
		}
		page.Items = append(page.Items, item)
	}
	cache := cacheNoStore
	return ListNotifications200JSONResponse{Body: page, Headers: ListNotifications200ResponseHeaders{CacheControl: &cache}}, nil
}

// newNotification presents n as the Notification schema. accounts caches the source accounts of
// one page by ID; a source account that no longer exists is left out.
func (s *Server) newNotification(ctx context.Context, n notification.Notification, accounts map[string]*Account) (Notification, error) {
	out := Notification{Id: n.ID, Type: NotificationType(n.Type), CreatedAt: n.CreatedAt}
	if n.StatusID != "" {
		id := n.StatusID
		out.StatusId = &id
	}
	if n.SourceAccountID == "" {
		return out, nil
	}
	a, ok := accounts[n.SourceAccountID]
	if !ok {
		actor, err := s.o.Accounts.FindByID(ctx, n.SourceAccountID)
		switch {
		case errors.Is(err, appaccount.ErrNotFound):
		case err != nil:
			return Notification{}, fmt.Errorf("clientapi: the source account %s of notification %s: %w", n.SourceAccountID, n.ID, err)
		default:
			acc := s.newAccount(actor)
			a = &acc
		}
		accounts[n.SourceAccountID] = a
	}
	out.Account = a
	return out, nil
}

// GetNotificationMarker serves GET /api/v1/notifications/marker: the viewer's read marker and
// unread count.
func (s *Server) GetNotificationMarker(ctx context.Context, _ GetNotificationMarkerRequestObject) (GetNotificationMarkerResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	m, err := s.o.Notifications.ReadMarker(ctx, v.AccountID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the notification marker of %s: %w", v.AccountID, err)
	}
	body, err := s.newNotificationMarker(ctx, v.AccountID, m)
	if err != nil {
		return nil, err
	}
	cache := cacheNoStore
	return GetNotificationMarker200JSONResponse{Body: body, Headers: GetNotificationMarker200ResponseHeaders{CacheControl: &cache}}, nil
}

// UpdateNotificationMarker serves PUT /api/v1/notifications/marker: it moves the viewer's read
// marker forward to last_read_id, never back.
func (s *Server) UpdateNotificationMarker(ctx context.Context, req UpdateNotificationMarkerRequestObject) (UpdateNotificationMarkerResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		body, headers := s.invalidParameter(ctx, "last_read_id")
		return UpdateNotificationMarkerdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusBadRequest}, nil
	}
	m, err := s.o.Notifications.MarkRead(ctx, v.AccountID, req.Body.LastReadId)
	if errors.Is(err, appnotification.ErrNotFound) {
		body, headers := s.notFound(ctx)
		return UpdateNotificationMarkerdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: mark the notifications of %s read: %w", v.AccountID, err)
	}
	body, err := s.newNotificationMarker(ctx, v.AccountID, m)
	if err != nil {
		return nil, err
	}
	cache := cacheNoStore
	return UpdateNotificationMarker200JSONResponse{Body: body, Headers: UpdateNotificationMarker200ResponseHeaders{CacheControl: &cache}}, nil
}

// newNotificationMarker presents accountID's marker m, with its unread count, as the
// NotificationMarker schema.
func (s *Server) newNotificationMarker(ctx context.Context, accountID string, m notification.ReadMarker) (NotificationMarker, error) {
	n, err := s.o.Notifications.UnreadCount(ctx, accountID)
	if err != nil {
		return NotificationMarker{}, fmt.Errorf("clientapi: the unread notifications of %s: %w", accountID, err)
	}
	out := NotificationMarker{UnreadCount: n}
	if m.LastReadID != "" {
		id := m.LastReadID
		out.LastReadId = &id
	}
	return out, nil
}

// DismissNotification serves POST /api/v1/notifications/{id}/dismiss.
func (s *Server) DismissNotification(ctx context.Context, req DismissNotificationRequestObject) (DismissNotificationResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	err = s.o.Notifications.Dismiss(ctx, v.AccountID, req.Id)
	if errors.Is(err, appnotification.ErrNotFound) {
		body, headers := s.notFound(ctx)
		return DismissNotificationdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: dismiss notification %s of %s: %w", req.Id, v.AccountID, err)
	}
	return DismissNotification204Response{}, nil
}
