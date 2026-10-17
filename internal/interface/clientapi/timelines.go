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
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/domain/timeline"
)

// Timelines reads the timelines (apptimeline.Service, FR-201 to FR-204).
type Timelines interface {
	Home(ctx context.Context, viewer appcontent.Viewer, q apptimeline.Query) (apptimeline.Page, error)
	Local(ctx context.Context, viewer appcontent.Viewer, q apptimeline.Query) (apptimeline.Page, error)
	Public(ctx context.Context, viewer appcontent.Viewer, q apptimeline.Query) (apptimeline.Page, error)
	Hashtag(ctx context.Context, viewer appcontent.Viewer, tagID string, q apptimeline.Query) (apptimeline.Page, error)
	Profile(ctx context.Context, viewer appcontent.Viewer, authorAccountID string, f apptimeline.ProfileFilter, q apptimeline.Query) (apptimeline.Page, error)
}

// TimelineReader reads the statuses that timelines list. *appcontent.Reader implements it.
type TimelineReader interface {
	// Views returns the statuses of ids that the viewer may see, in order.
	Views(ctx context.Context, viewer appcontent.Viewer, ids []string) ([]appcontent.StatusView, error)
	// Boosts returns the boosts of ids, in order, leaving out missing ones.
	Boosts(ctx context.Context, ids []string) ([]content.Boost, error)
	// Tag returns a hashtag by name, failing with appcontent.ErrNotFound when no status used it.
	Tag(ctx context.Context, name string) (content.Tag, error)
	// Pinned returns the statuses that accountID pins and the viewer may see, newest pin first.
	Pinned(ctx context.Context, viewer appcontent.Viewer, accountID string) ([]appcontent.StatusView, error)
}

// timelineViewer returns the viewer of the request in ctx for the content module, and the
// authenticated Viewer, nil for a visitor.
func timelineViewer(ctx context.Context) (appcontent.Viewer, *Viewer) {
	v, ok := ViewerFrom(ctx)
	if !ok {
		return contentViewer(ctx), nil
	}
	return contentViewer(ctx), &v
}

// statusItem is one item of a page of statuses: a post (BoostID "") or a boost of StatusID.
type statusItem struct {
	StatusID string
	BoostID  string
}

// statuses presents items in order as viewer sees them. An item whose status, boost, or
// booster is gone or hidden (since the page was read) is left out.
func (s *Server) statuses(ctx context.Context, viewer appcontent.Viewer, items []statusItem) ([]Status, error) {
	var statusIDs, boostIDs []string
	for _, it := range items {
		statusIDs = append(statusIDs, it.StatusID)
		if it.BoostID != "" {
			boostIDs = append(boostIDs, it.BoostID)
		}
	}
	views, err := s.o.TimelineReader.Views(ctx, viewer, statusIDs)
	if err != nil {
		return nil, err
	}
	boosts := map[string]content.Boost{}
	// The boosters, read once each; nil for one that no longer exists.
	boosters := map[string]*appaccount.Actor{}
	var found []appaccount.Actor
	if len(boostIDs) > 0 {
		bs, err := s.o.TimelineReader.Boosts(ctx, boostIDs)
		if err != nil {
			return nil, err
		}
		for _, b := range bs {
			boosts[b.ID] = b
			if _, ok := boosters[b.BoosterAccountID]; ok {
				continue
			}
			a, err := s.o.Accounts.FindByID(ctx, b.BoosterAccountID)
			switch {
			case err == nil:
				boosters[b.BoosterAccountID] = &a
				found = append(found, a)
			case errors.Is(err, appaccount.ErrNotFound):
				boosters[b.BoosterAccountID] = nil
			default:
				return nil, err
			}
		}
	}
	parts, err := s.statusParts(ctx, views, found...)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Status, len(views))
	for _, v := range views {
		byID[v.Status.ID] = s.newStatus(v, parts)
	}
	out := make([]Status, 0, len(items))
	for _, it := range items {
		st, ok := byID[it.StatusID]
		if !ok {
			continue
		}
		if it.BoostID == "" {
			out = append(out, st)
			continue
		}
		b, ok := boosts[it.BoostID]
		if !ok {
			continue
		}
		booster := boosters[b.BoosterAccountID]
		if booster == nil {
			continue
		}
		boosted := st
		out = append(out, Status{
			Id: b.ID, Uri: b.URI, CreatedAt: b.BoostedAt, Account: s.newAccount(*booster, parts),
			Visibility: st.Visibility, Language: st.Language, Boost: &boosted,
			MediaAttachments: []MediaAttachment{}, Mentions: []StatusMention{}, Tags: []StatusTag{},
		})
	}
	return out, nil
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
	cache := cachePublic
	if v != nil {
		cache = cacheNoStore
	}
	statuses, err := s.statuses(ctx, contentViewer(ctx), items)
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
	cv, v := timelineViewer(ctx)
	if v == nil {
		return nil, errNoViewer
	}
	if _, ok := pageLimit(req.Params.Limit); !ok {
		return GetHomeTimelinedefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "limit")), nil
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
	cv, v := timelineViewer(ctx)
	if _, ok := pageLimit(req.Params.Limit); !ok {
		return GetLocalTimelinedefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "limit")), nil
	}
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

// timelineEnabled reports whether the instance setting enabled turns a timeline on (FR-202).
func (s *Server) timelineEnabled(ctx context.Context, enabled func(instance.Settings) bool) (bool, error) {
	var settings instance.Settings
	if s.o.Instance != nil {
		var err error
		if settings, err = s.o.Instance.Settings(ctx); err != nil {
			return false, fmt.Errorf("clientapi: instance settings: %w", err)
		}
	}
	return enabled(settings), nil
}

// GetPublicTimeline serves GET /api/v1/timelines/public: the public posts of local and remote
// accounts. It is a 404 while the instance setting federated_timeline_enabled is off (FR-202).
func (s *Server) GetPublicTimeline(ctx context.Context, req GetPublicTimelineRequestObject) (GetPublicTimelineResponseObject, error) {
	if on, err := s.timelineEnabled(ctx, instance.Settings.FederatedTimelineEnabled); err != nil {
		return nil, err
	} else if !on {
		return GetPublicTimelinedefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	cv, v := timelineViewer(ctx)
	if _, ok := pageLimit(req.Params.Limit); !ok {
		return GetPublicTimelinedefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "limit")), nil
	}
	q, err := s.timelineQuery(ctx, v, req.Params.Cursor, req.Params.Limit, true)
	if err != nil {
		return nil, err
	}
	p, err := s.o.Timelines.Public(ctx, cv, q)
	if err != nil {
		return nil, err
	}
	res, err := s.statusPage(ctx, v, entryItems(p.Entries), p.Next)
	return GetPublicTimeline200JSONResponse{res}, err
}

// GetHashtagTimeline serves GET /api/v1/timelines/tag/{hashtag}. A hashtag that no status used
// has an empty timeline.
func (s *Server) GetHashtagTimeline(ctx context.Context, req GetHashtagTimelineRequestObject) (GetHashtagTimelineResponseObject, error) {
	cv, v := timelineViewer(ctx)
	if _, ok := pageLimit(req.Params.Limit); !ok {
		return GetHashtagTimelinedefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "limit")), nil
	}
	q, err := s.timelineQuery(ctx, v, req.Params.Cursor, req.Params.Limit, true)
	if err != nil {
		return nil, err
	}
	var p apptimeline.Page
	tag, err := s.o.TimelineReader.Tag(ctx, req.Hashtag)
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
	cv, v := timelineViewer(ctx)
	if _, ok := pageLimit(req.Params.Limit); !ok {
		return ListAccountStatusesdefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "limit")), nil
	}
	if _, err := s.o.Accounts.FindByID(ctx, req.Id); errors.Is(err, appaccount.ErrNotFound) {
		l := s.Localizer(ctx)
		body, headers := s.NewProblem(ctx, http.StatusNotFound, ProblemTypeBlank, l.ServerErrorsNotFound(), "")
		return ListAccountStatusesdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
	} else if err != nil {
		return nil, err
	}
	params := req.Params
	if params.Pinned != nil && *params.Pinned {
		cache := cachePublic
		if v != nil {
			cache = cacheNoStore
		}
		body := StatusPage{Items: []Status{}}
		// Every pinned status fits in the first page, so a cursor asks for an empty one.
		if params.Cursor == nil {
			views, err := s.o.TimelineReader.Pinned(ctx, cv, req.Id)
			if err != nil {
				return nil, err
			}
			if body.Items, err = s.presentStatuses(ctx, views); err != nil {
				return nil, err
			}
		}
		return ListAccountStatuses200JSONResponse{StatusPageJSONResponse{Body: body, Headers: StatusPageResponseHeaders{CacheControl: &cache}}}, nil
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
