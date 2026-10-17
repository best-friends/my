// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/domain/moderation"
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
	items, err := s.newNotifications(ctx, v.AccountID, ns)
	if err != nil {
		return nil, err
	}
	page.Items = append(page.Items, items...)
	cache := cacheNoStore
	return ListNotifications200JSONResponse{Body: page, Headers: ListNotifications200ResponseHeaders{CacheControl: &cache}}, nil
}

// newNotifications presents accountID's notifications ns, in order, as the Notification schema.
// The posts that they are about are read together through the content module, which applies the
// visibility policy: a post that the user may not see is left out of its notification. The source
// accounts are read once each; a source account that no longer exists is left out. A moderation
// notice carries its action as the user sees it (moderation.go), and a report_resolved the
// user's report, which tells only that it is resolved.
func (s *Server) newNotifications(ctx context.Context, accountID string, ns []notification.Notification) ([]Notification, error) {
	var statusIDs []string
	seen := map[string]bool{}
	for _, n := range ns {
		if n.StatusID != "" && !seen[n.StatusID] {
			seen[n.StatusID] = true
			statusIDs = append(statusIDs, n.StatusID)
		}
	}
	var views []appcontent.StatusView
	if len(statusIDs) > 0 {
		var err error
		views, err = s.o.TimelineReader.Views(ctx, appcontent.Viewer{Kind: appcontent.ViewerLocal, AccountID: accountID}, statusIDs)
		if err != nil {
			return nil, fmt.Errorf("clientapi: the statuses of the notifications of %s: %w", accountID, err)
		}
	}
	// The source accounts, read once each; nil for one that no longer exists.
	sources := map[string]*appaccount.Actor{}
	var found []appaccount.Actor
	for _, n := range ns {
		if _, ok := sources[n.SourceAccountID]; ok || n.SourceAccountID == "" {
			continue
		}
		actor, err := s.o.Accounts.FindByID(ctx, n.SourceAccountID)
		switch {
		case err == nil:
			sources[n.SourceAccountID] = &actor
			found = append(found, actor)
		case errors.Is(err, appaccount.ErrNotFound):
			sources[n.SourceAccountID] = nil
		default:
			return nil, fmt.Errorf("clientapi: the source account %s of notification %s: %w", n.SourceAccountID, n.ID, err)
		}
	}
	reports, err := s.notifiedReports(ctx, accountID, ns)
	if err != nil {
		return nil, err
	}
	var actionIDs []string
	for _, n := range ns {
		if n.ModerationActionID != "" {
			actionIDs = append(actionIDs, n.ModerationActionID)
		}
	}
	actions, err := s.noticeActions(ctx, accountID, actionIDs)
	if err != nil {
		return nil, err
	}
	parts, err := s.statusParts(ctx, views, found...)
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]Status, len(views))
	for _, v := range views {
		statuses[v.Status.ID] = s.newStatus(v, parts)
	}
	accounts := make(map[string]Account, len(found))
	for id, a := range sources {
		if a != nil {
			accounts[id] = s.newAccount(*a, parts)
		}
	}
	out := make([]Notification, 0, len(ns))
	for _, n := range ns {
		item := Notification{Id: n.ID, Type: NotificationType(n.Type), CreatedAt: n.CreatedAt}
		if st, ok := statuses[n.StatusID]; ok {
			item.Status = &st
		}
		if a, ok := accounts[n.SourceAccountID]; ok {
			item.Account = &a
		}
		if r, ok := reports[n.ReportID]; ok {
			item.Report = &r
		}
		if a, ok := actions[n.ModerationActionID]; ok {
			item.ModerationAction = &a
		}
		out = append(out, item)
	}
	return out, nil
}

// notifiedReports presents, by ID, accountID's own reports that the report_resolved
// notifications ns tell of; a report that no longer exists is left out.
func (s *Server) notifiedReports(ctx context.Context, accountID string, ns []notification.Notification) (map[string]Report, error) {
	if s.o.FiledReports == nil {
		return nil, nil
	}
	var recs []moderation.ReportRecord
	seen := map[string]bool{}
	for _, n := range ns {
		if n.ReportID == "" || seen[n.ReportID] {
			continue
		}
		seen[n.ReportID] = true
		rec, err := s.o.FiledReports.ReportFiledBy(ctx, accountID, n.ReportID)
		switch {
		case errors.Is(err, appmoderation.ErrNotFound):
			continue
		case err != nil:
			return nil, fmt.Errorf("clientapi: the report %s of notification %s: %w", n.ReportID, n.ID, err)
		}
		recs = append(recs, rec)
	}
	items, err := s.newReports(ctx, recs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Report, len(items))
	for _, r := range items {
		out[r.Id] = r
	}
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
