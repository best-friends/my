// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	apptimeline "github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/domain/timeline"
)

// Timelines reads the timelines (apptimeline.Service, FR-201 to FR-204).
type Timelines interface {
	Home(ctx context.Context, viewer appcontent.Viewer, q apptimeline.Query) (apptimeline.Page, error)
	Local(ctx context.Context, viewer appcontent.Viewer, q apptimeline.Query) (apptimeline.Page, error)
	Hashtag(ctx context.Context, viewer appcontent.Viewer, tagID string, q apptimeline.Query) (apptimeline.Page, error)
	Profile(ctx context.Context, viewer appcontent.Viewer, authorAccountID string, f apptimeline.ProfileFilter, q apptimeline.Query) (apptimeline.Page, error)
}

// contentViewer returns the viewer of the visibility policy for the request in ctx.
func contentViewer(ctx context.Context) (appcontent.Viewer, *Viewer) {
	v, ok := ViewerFrom(ctx)
	if !ok {
		return appcontent.Viewer{Kind: appcontent.ViewerAnonymous}, nil
	}
	return appcontent.Viewer{Kind: appcontent.ViewerLocal, AccountID: v.AccountID}, &v
}

// timelineQuery is the query of a page; languages applies the viewer's timeline languages
// (FR-208).
func (s *Server) timelineQuery(ctx context.Context, v *Viewer, cursor *Cursor, limit *Limit, languages bool) (apptimeline.Query, error) {
	var q apptimeline.Query
	if cursor != nil {
		q.Before = *cursor
	}
	if limit != nil {
		q.Limit = *limit
	}
	if languages && v != nil && s.o.TimelineLanguages != nil {
		langs, err := s.o.TimelineLanguages(ctx, v.UserID)
		if err != nil {
			return q, fmt.Errorf("clientapi: the timeline languages of %s: %w", v.UserID, err)
		}
		q.Languages = langs
	}
	return q, nil
}

// statusPage presents a page of a timeline. An anonymous page is the same for every visitor,
// so shared caches may keep it briefly (ADR-0016); a viewer's page is not stored.
func (s *Server) statusPage(ctx context.Context, v *Viewer, items []statusItem, next string) (StatusPageJSONResponse, error) {
	var viewerAccountID string
	cache := cachePublic
	if v != nil {
		viewerAccountID, cache = v.AccountID, cacheNoStore
	}
	statuses, err := s.newStatusRenderer(viewerAccountID).statuses(ctx, items)
	if err != nil {
		return StatusPageJSONResponse{}, err
	}
	body := StatusPage{Items: statuses}
	if next != "" {
		body.NextCursor = &next
	}
	return StatusPageJSONResponse{Body: body, Headers: StatusPageResponseHeaders{CacheControl: &cache}}, nil
}

// entryItems returns the items of the entries of a timeline page.
func entryItems(entries []apptimeline.Entry) []statusItem {
	items := make([]statusItem, 0, len(entries))
	for _, e := range entries {
		it := statusItem{StatusID: e.StatusID}
		if e.Kind == timeline.KindBoost {
			it.BoostID = e.ID
		}
		items = append(items, it)
	}
	return items
}

// GetHomeTimeline serves GET /api/v1/timelines/home.
func (s *Server) GetHomeTimeline(ctx context.Context, req GetHomeTimelineRequestObject) (GetHomeTimelineResponseObject, error) {
	cv, v := contentViewer(ctx)
	if v == nil {
		return nil, errNoViewer
	}
	q, err := s.timelineQuery(ctx, v, req.Params.Cursor, req.Params.Limit, true)
	if err != nil {
		return nil, err
	}
	p, err := s.o.Timelines.Home(ctx, cv, q)
	if err != nil {
		return nil, err
	}
	res, err := s.statusPage(ctx, v, entryItems(p.Entries), p.Next)
	return GetHomeTimeline200JSONResponse{res}, err
}

// GetLocalTimeline serves GET /api/v1/timelines/local. Visitors who are not logged in see only
// public posts; local posts need a logged-in viewer (ADR-0017).
func (s *Server) GetLocalTimeline(ctx context.Context, req GetLocalTimelineRequestObject) (GetLocalTimelineResponseObject, error) {
	cv, v := contentViewer(ctx)
	q, err := s.timelineQuery(ctx, v, req.Params.Cursor, req.Params.Limit, true)
	if err != nil {
		return nil, err
	}
	p, err := s.o.Timelines.Local(ctx, cv, q)
	if err != nil {
		return nil, err
	}
	res, err := s.statusPage(ctx, v, entryItems(p.Entries), p.Next)
	return GetLocalTimeline200JSONResponse{res}, err
}

// GetHashtagTimeline serves GET /api/v1/timelines/tag/{hashtag}. A hashtag that no status used
// has an empty timeline.
func (s *Server) GetHashtagTimeline(ctx context.Context, req GetHashtagTimelineRequestObject) (GetHashtagTimelineResponseObject, error) {
	cv, v := contentViewer(ctx)
	q, err := s.timelineQuery(ctx, v, req.Params.Cursor, req.Params.Limit, true)
	if err != nil {
		return nil, err
	}
	var p apptimeline.Page
	tag, err := s.o.StatusViews.Tag(ctx, req.Hashtag)
	switch {
	case err == nil:
		if p, err = s.o.Timelines.Hashtag(ctx, cv, tag.ID, q); err != nil {
			return nil, err
		}
	case !errors.Is(err, appcontent.ErrNotFound):
		return nil, err
	}
	res, err := s.statusPage(ctx, v, entryItems(p.Entries), p.Next)
	return GetHashtagTimeline200JSONResponse{res}, err
}

// ListAccountStatuses serves GET /api/v1/accounts/{id}/statuses: the account's profile
// timeline, or its pinned statuses in one page. An unknown account is a 404. The viewer's
// timeline languages do not apply: a profile shows the account in every language.
func (s *Server) ListAccountStatuses(ctx context.Context, req ListAccountStatusesRequestObject) (ListAccountStatusesResponseObject, error) {
	cv, v := contentViewer(ctx)
	if _, err := s.o.Accounts.FindByID(ctx, req.Id); errors.Is(err, appaccount.ErrNotFound) {
		l := s.Localizer(ctx)
		body, headers := s.NewProblem(ctx, http.StatusNotFound, ProblemTypeBlank, l.ServerErrorsNotFound(), "")
		return ListAccountStatusesdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
	} else if err != nil {
		return nil, err
	}
	params := req.Params
	if params.Pinned != nil && *params.Pinned {
		var items []statusItem
		// Every pinned status fits in the first page, so a cursor asks for an empty one.
		if params.Cursor == nil {
			ids, err := s.o.StatusViews.Pinned(ctx, cv, req.Id)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				items = append(items, statusItem{StatusID: id})
			}
		}
		res, err := s.statusPage(ctx, v, items, "")
		return ListAccountStatuses200JSONResponse{res}, err
	}
	q, err := s.timelineQuery(ctx, v, params.Cursor, params.Limit, false)
	if err != nil {
		return nil, err
	}
	f := apptimeline.ProfileFilter{
		ExcludeReplies: params.ExcludeReplies != nil && *params.ExcludeReplies,
		ExcludeBoosts:  params.ExcludeBoosts != nil && *params.ExcludeBoosts,
	}
	p, err := s.o.Timelines.Profile(ctx, cv, req.Id, f, q)
	if err != nil {
		return nil, err
	}
	res, err := s.statusPage(ctx, v, entryItems(p.Entries), p.Next)
	return ListAccountStatuses200JSONResponse{res}, err
}
