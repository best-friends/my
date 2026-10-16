// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"context"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/notification"
)

// Accounts is the part of the account module's public API that the projectors read: whether a
// recipient is a local account. Only local accounts receive notifications here; remote ones are
// notified by their own servers. *appaccount.Service implements it.
type Accounts interface {
	IsLocal(ctx context.Context, tx uow.Tx, accountID string) (bool, error)
}

// Relationships is the part of the graph module's public API that the projectors read: whether
// a block between the recipient and the source, or the recipient's mute of the source with
// hide_notifications, keeps the notification from being created. *appgraph.Service implements it.
type Relationships interface {
	HidesNotifications(ctx context.Context, tx uow.Tx, recipientID, sourceID string) (bool, error)
}

// ProjectorDeps are the ports the notification projectors use.
type ProjectorDeps struct {
	Repository    Repository
	Accounts      Accounts
	Relationships Relationships
	IDs           kernel.IDGenerator
}

// RegisterProjectors registers the synchronous projectors that create notifications from the
// graph and content events, in the transaction of the command that emits them (data-model
// §2.1, §4.7):
//
//   - graph.followed: follow, to the followee.
//   - graph.follow_requested: follow_request, to the followee.
//   - graph.follow_accepted: follow_accepted, to the follower.
//   - content.status_created and content.status_revised: mention, to each account the revision
//     mentions; an edit notifies only the accounts it newly mentions.
//   - content.boosted: boost, to the status's author.
//   - content.favourited: favourite, to the status's author.
//
// A notification goes only to a local recipient other than its source, and not when either
// blocks the other or the recipient mutes the source with hide_notifications. Each notification
// has a dedupe key unique per recipient (notification.FollowKey, MentionKey, InteractionKey),
// so projecting an event again, or a repeated boost or favourite, creates nothing more.
//
// The exclusion projectors keep notification_exclusions, which the list and the unread count
// apply at read time, so that a later block or mute also hides the notifications that came
// before it:
//
//   - graph.blocked: block from the blocker to the blocked account, and blocked_by back.
//   - graph.unblocked: deletes both.
//   - graph.muted: mute from the account to the target with the mute's expiry when the mute
//     hides notifications; deletes it otherwise.
//   - graph.unmuted: deletes the mute.
//
// The projector of content.status_deleted records the status in notification_deleted_statuses,
// which the list and the unread count also apply, so that the mention, boost, and favourite
// notifications about a deleted status no longer appear. Notifications are events and stay
// until the erasure job deletes them.
//
// Register them after the graph and content projectors.
func RegisterProjectors(r *uow.Projectors, d ProjectorDeps) {
	p := &projector{d: d}
	r.Register(appgraph.EventFollowed, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appgraph.Followed)
		f := ev.Follow
		return p.notify(ctx, tx, notification.Notification{
			RecipientAccountID: f.FolloweeAccountID, SourceAccountID: f.FollowerAccountID, Type: notification.TypeFollow,
			DedupeKey: notification.FollowKey(notification.TypeFollow, f.ID), CreatedAt: ev.Acceptance.AcceptedAt,
		})
	})
	r.Register(appgraph.EventFollowRequested, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		f := e.(appgraph.FollowRequested).Follow
		return p.notify(ctx, tx, notification.Notification{
			RecipientAccountID: f.FolloweeAccountID, SourceAccountID: f.FollowerAccountID, Type: notification.TypeFollowRequest,
			DedupeKey: notification.FollowKey(notification.TypeFollowRequest, f.ID), CreatedAt: f.RequestedAt,
		})
	})
	r.Register(appgraph.EventFollowAccepted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appgraph.FollowAccepted)
		f := ev.Follow
		return p.notify(ctx, tx, notification.Notification{
			RecipientAccountID: f.FollowerAccountID, SourceAccountID: f.FolloweeAccountID, Type: notification.TypeFollowAccepted,
			DedupeKey: notification.FollowKey(notification.TypeFollowAccepted, f.ID), CreatedAt: ev.Acceptance.AcceptedAt,
		})
	})
	r.Register(appgraph.EventBlocked, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		b := e.(appgraph.Blocked).Block
		for _, x := range blockExclusions(b.BlockerAccountID, b.BlockedAccountID) {
			if err := d.Repository.UpsertExclusion(ctx, tx, x); err != nil {
				return err
			}
		}
		return nil
	})
	r.Register(appgraph.EventUnblocked, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		b := e.(appgraph.Unblocked).Block
		for _, x := range blockExclusions(b.BlockerAccountID, b.BlockedAccountID) {
			if err := d.Repository.DeleteExclusion(ctx, tx, x.ViewerAccountID, x.TargetAccountID, x.Reason); err != nil {
				return err
			}
		}
		return nil
	})
	r.Register(appgraph.EventMuted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		m := e.(appgraph.Muted).Mute
		if !m.HideNotifications {
			return d.Repository.DeleteExclusion(ctx, tx, m.AccountID, m.TargetAccountID, notification.ExcludedMute)
		}
		return d.Repository.UpsertExclusion(ctx, tx, notification.Exclusion{
			ViewerAccountID: m.AccountID, TargetAccountID: m.TargetAccountID, Reason: notification.ExcludedMute, ExpiresAt: m.ExpiresAt,
		})
	})
	r.Register(appgraph.EventUnmuted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appgraph.Unmuted)
		return d.Repository.DeleteExclusion(ctx, tx, ev.AccountID, ev.TargetAccountID, notification.ExcludedMute)
	})
	r.Register(appcontent.EventStatusCreated, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appcontent.StatusCreated)
		return p.mentions(ctx, tx, ev.Status.ID, ev.Status.AuthorAccountID, ev.Revision.MentionedAccountIDs, ev.Revision.RevisedAt)
	})
	r.Register(appcontent.EventStatusRevised, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appcontent.StatusRevised)
		return p.mentions(ctx, tx, ev.Status.ID, ev.Status.AuthorAccountID, ev.Revision.MentionedAccountIDs, ev.Revision.RevisedAt)
	})
	r.Register(appcontent.EventStatusDeleted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		return d.Repository.InsertDeletedStatus(ctx, tx, e.(appcontent.StatusDeleted).Deletion.StatusID)
	})
	r.Register(appcontent.EventBoosted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appcontent.Boosted)
		return p.interaction(ctx, tx, notification.TypeBoost, ev.Status, ev.Boost.BoosterAccountID, ev.Boost.BoostedAt)
	})
	r.Register(appcontent.EventFavourited, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appcontent.Favourited)
		return p.interaction(ctx, tx, notification.TypeFavourite, ev.Status, ev.Favourite.AccountID, ev.Favourite.CreatedAt)
	})
}

// blockExclusions are the exclusions of blocker's block of blocked: each hides the other's
// notifications.
func blockExclusions(blocker, blocked string) []notification.Exclusion {
	return []notification.Exclusion{
		{ViewerAccountID: blocker, TargetAccountID: blocked, Reason: notification.ExcludedBlock},
		{ViewerAccountID: blocked, TargetAccountID: blocker, Reason: notification.ExcludedBlockedBy},
	}
}

type projector struct {
	d ProjectorDeps
}

func (p *projector) mentions(ctx context.Context, tx uow.Tx, statusID, authorID string, mentioned []string, at time.Time) error {
	for _, id := range mentioned {
		err := p.notify(ctx, tx, notification.Notification{
			RecipientAccountID: id, SourceAccountID: authorID, Type: notification.TypeMention, StatusID: statusID,
			DedupeKey: notification.MentionKey(statusID), CreatedAt: at,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *projector) interaction(ctx context.Context, tx uow.Tx, t notification.Type, st appcontent.StatusRef, sourceID string, at time.Time) error {
	if !st.IsLocal {
		return nil // a remote author is notified by its own server
	}
	return p.notify(ctx, tx, notification.Notification{
		RecipientAccountID: st.AuthorAccountID, SourceAccountID: sourceID, Type: t, StatusID: st.ID,
		DedupeKey: notification.InteractionKey(t, st.ID, sourceID), CreatedAt: at,
	})
}

// notify inserts n unless its recipient is its source or not local, or the relationship
// between them hides it. A notification with the same dedupe key leaves n out.
func (p *projector) notify(ctx context.Context, tx uow.Tx, n notification.Notification) error {
	if n.RecipientAccountID == n.SourceAccountID {
		return nil
	}
	local, err := p.d.Accounts.IsLocal(ctx, tx, n.RecipientAccountID)
	if err != nil || !local {
		return err
	}
	if n.SourceAccountID != "" {
		hidden, err := p.d.Relationships.HidesNotifications(ctx, tx, n.RecipientAccountID, n.SourceAccountID)
		if err != nil || hidden {
			return err
		}
	}
	n.ID = p.d.IDs.NewID()
	if err := n.Validate(); err != nil {
		return err
	}
	_, err = p.d.Repository.InsertNotification(ctx, tx, n)
	return err
}
