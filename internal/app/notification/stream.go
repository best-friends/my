// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"context"

	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/app/uow"
)

// RegisterCommitHooks signals to pub, once the command commits, the accounts that the events
// of RegisterProjectors may have notified (FR-207). The hooks name every candidate recipient of
// an event: the projectors may still have created nothing for it (a remote or blocking
// recipient, a repeated dedupe key), so a stream reads the notifications after the last one it
// sent and sends only those.
func RegisterCommitHooks(r *uow.Projectors, pub stream.Publisher) {
	signal := func(accountIDs ...string) {
		for _, id := range accountIDs {
			pub.Publish(stream.Event{Kind: stream.KindNotifications, AccountID: id})
		}
	}
	r.OnCommit(appgraph.EventFollowed, func(_ context.Context, e uow.Event) {
		signal(e.(appgraph.Followed).Follow.FolloweeAccountID)
	})
	r.OnCommit(appgraph.EventFollowRequested, func(_ context.Context, e uow.Event) {
		signal(e.(appgraph.FollowRequested).Follow.FolloweeAccountID)
	})
	r.OnCommit(appgraph.EventFollowAccepted, func(_ context.Context, e uow.Event) {
		signal(e.(appgraph.FollowAccepted).Follow.FollowerAccountID)
	})
	r.OnCommit(appcontent.EventStatusCreated, func(_ context.Context, e uow.Event) {
		signal(e.(appcontent.StatusCreated).Revision.MentionedAccountIDs...)
	})
	r.OnCommit(appcontent.EventStatusRevised, func(_ context.Context, e uow.Event) {
		signal(e.(appcontent.StatusRevised).Revision.MentionedAccountIDs...)
	})
	r.OnCommit(appcontent.EventBoosted, func(_ context.Context, e uow.Event) {
		signal(e.(appcontent.Boosted).Status.AuthorAccountID)
	})
	r.OnCommit(appcontent.EventFavourited, func(_ context.Context, e uow.Event) {
		signal(e.(appcontent.Favourited).Status.AuthorAccountID)
	})
}
