// SPDX-License-Identifier: AGPL-3.0-only

package timeline

import (
	"context"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/app/uow"
)

// RegisterCommitHooks signals every new home timeline entry to pub once the command that adds
// it commits (FR-207): a status as posted (content.status_created) and a boost
// (content.boosted). The signal carries only the entry ID; each stream asks HomeEntry whether
// its viewer's home timeline lists the entry.
func RegisterCommitHooks(r *uow.Projectors, pub stream.Publisher) {
	r.OnCommit(appcontent.EventStatusCreated, func(_ context.Context, e uow.Event) {
		pub.Publish(stream.Event{Kind: stream.KindHomeEntry, EntryID: e.(appcontent.StatusCreated).Status.ID})
	})
	r.OnCommit(appcontent.EventBoosted, func(_ context.Context, e uow.Event) {
		pub.Publish(stream.Event{Kind: stream.KindHomeEntry, EntryID: e.(appcontent.Boosted).Boost.ID})
	})
}

// HomeEntry returns the entry id when the home timeline of a logged-in viewer lists it, as Home
// would, in the timeline languages q.Languages (q.Before and q.Limit are ignored); false when it
// does not, because the viewer does not follow its author, may not see it, or it is in another
// language. It reads the entries from id up, so it is meant for an entry that has just been
// added, as the streaming endpoint asks.
func (s *Service) HomeEntry(ctx context.Context, viewer appcontent.Viewer, id string, q Query) (Entry, bool, error) {
	if viewer.Kind != appcontent.ViewerLocal || viewer.AccountID == "" {
		return Entry{}, false, ErrLoginRequired
	}
	w := s.window(viewer, Query{Languages: q.Languages, Limit: MaxLimit})
	rows, err := s.d.Repository.ListHome(ctx, HomeQuery{Window: w, Floor: id})
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range rows {
		if e.ID != id {
			continue
		}
		p, err := s.page(ctx, appcontent.ContextHome, viewer, w, []Entry{e})
		if err != nil || len(p.Entries) == 0 {
			return Entry{}, false, err
		}
		return p.Entries[0], true, nil
	}
	return Entry{}, false, nil
}
