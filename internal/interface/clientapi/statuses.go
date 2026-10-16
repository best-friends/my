// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
)

var errNotImplemented = errors.New("clientapi: not implemented yet")

func (s *Server) ListBookmarks(context.Context, ListBookmarksRequestObject) (ListBookmarksResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) CreateStatus(context.Context, CreateStatusRequestObject) (CreateStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) DeleteStatus(context.Context, DeleteStatusRequestObject) (DeleteStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) GetStatus(context.Context, GetStatusRequestObject) (GetStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) EditStatus(context.Context, EditStatusRequestObject) (EditStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UnbookmarkStatus(context.Context, UnbookmarkStatusRequestObject) (UnbookmarkStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) BookmarkStatus(context.Context, BookmarkStatusRequestObject) (BookmarkStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UnboostStatus(context.Context, UnboostStatusRequestObject) (UnboostStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) BoostStatus(context.Context, BoostStatusRequestObject) (BoostStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) GetStatusContext(context.Context, GetStatusContextRequestObject) (GetStatusContextResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UnfavouriteStatus(context.Context, UnfavouriteStatusRequestObject) (UnfavouriteStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) FavouriteStatus(context.Context, FavouriteStatusRequestObject) (FavouriteStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) GetStatusHistory(context.Context, GetStatusHistoryRequestObject) (GetStatusHistoryResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UnpinStatus(context.Context, UnpinStatusRequestObject) (UnpinStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) PinStatus(context.Context, PinStatusRequestObject) (PinStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) GetStatusSource(context.Context, GetStatusSourceRequestObject) (GetStatusSourceResponseObject, error) {
	return nil, errNotImplemented
}
