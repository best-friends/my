// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// Event types of the content module's statuses, for uow.Projectors.
const (
	EventStatusCreated = "content.status_created"
	EventStatusRevised = "content.status_revised"
	EventStatusDeleted = "content.status_deleted"
)

// StatusCreated is emitted when a status and its first revision are inserted.
type StatusCreated struct {
	Status   content.Status
	Revision content.Revision
}

// EventType implements uow.Event.
func (StatusCreated) EventType() string { return EventStatusCreated }

// StatusRevised is emitted when a later revision of a status is inserted.
type StatusRevised struct {
	Revision content.Revision
}

// EventType implements uow.Event.
func (StatusRevised) EventType() string { return EventStatusRevised }

// StatusDeleted is emitted when the deletion of a status is inserted.
type StatusDeleted struct {
	Deletion content.Deletion
	Status   content.Status
}

// EventType implements uow.Event.
func (StatusDeleted) EventType() string { return EventStatusDeleted }

// RegisterProjectors registers the synchronous projectors of the content module's statuses
// (data-model §9). Every status projection covers only statuses that are not deleted:
//   - status_heads points at the revision with the highest Seq;
//   - status_mention_index holds the mentions of that revision;
//   - status_reply_links links a reply to its parent once both are known;
//   - status_counters counts the replies whose visibility CountsAsReply (the boosts and
//     favourites are counted by RegisterInteractionProjectors);
//   - account_post_counters counts each account's statuses and their latest PublishedAt.
//
// The rebuild queries of the Repository state the same rules in SQL; the property test in
// testkit.StatusRepositoryContract checks that both agree.
func RegisterProjectors(r *uow.Projectors, repo Repository) {
	r.Register(EventStatusCreated, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(StatusCreated)
		return projectCreated(ctx, tx, repo, ev.Status, ev.Revision)
	})
	r.Register(EventStatusRevised, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		rev := e.(StatusRevised).Revision
		deleted, err := repo.IsDeleted(ctx, tx, rev.StatusID)
		if err != nil || deleted {
			return err
		}
		return projectRevision(ctx, tx, repo, rev)
	})
	r.Register(EventStatusDeleted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(StatusDeleted)
		return projectDeleted(ctx, tx, repo, ev.Status)
	})
}

func projectRevision(ctx context.Context, tx uow.Tx, repo Repository, rev content.Revision) error {
	head, err := repo.Head(ctx, tx, rev.StatusID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if !rev.Supersedes(head) {
		return nil
	}
	if err := repo.UpsertHead(ctx, tx, content.HeadOf(rev)); err != nil {
		return err
	}
	return repo.ReplaceMentions(ctx, tx, rev.StatusID, rev.MentionedAccountIDs)
}

func projectCreated(ctx context.Context, tx uow.Tx, repo Repository, s content.Status, rev content.Revision) error {
	if err := projectRevision(ctx, tx, repo, rev); err != nil {
		return err
	}
	// Replies that arrived before this status now have a known parent.
	replies, err := repo.ListLiveRepliesTo(ctx, tx, s.URI)
	if err != nil {
		return err
	}
	counters := content.Counters{StatusID: s.ID}
	for _, reply := range replies {
		if reply.ID == s.ID {
			continue // a status that replies to itself is linked below, as a reply
		}
		if err := repo.UpsertReplyLink(ctx, tx, linkTo(reply, s)); err != nil {
			return err
		}
		if reply.Visibility.CountsAsReply() {
			counters.RepliesCount++
		}
	}
	if err := repo.InsertCounters(ctx, tx, counters); err != nil {
		return err
	}
	if s.InReplyToURI != "" {
		parent, err := repo.FindLiveStatusByURI(ctx, tx, s.InReplyToURI)
		switch {
		case errors.Is(err, ErrNotFound):
			// The parent is unknown or deleted; a parent that arrives later links this reply.
		case err != nil:
			return err
		default:
			if err := repo.UpsertReplyLink(ctx, tx, linkTo(s, parent)); err != nil {
				return err
			}
			if s.Visibility.CountsAsReply() {
				if err := repo.AddReplies(ctx, tx, parent.ID, 1); err != nil {
					return err
				}
			}
		}
	}
	return repo.AddPost(ctx, tx, s.AuthorAccountID, s.PublishedAt)
}

func projectDeleted(ctx context.Context, tx uow.Tx, repo Repository, s content.Status) error {
	link, err := repo.ReplyLink(ctx, tx, s.ID)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return err
	case link.ParentStatusID != s.ID && s.Visibility.CountsAsReply():
		if err := repo.AddReplies(ctx, tx, link.ParentStatusID, -1); err != nil {
			return err
		}
	}
	if err := repo.DeleteStatusProjections(ctx, tx, s.ID); err != nil {
		return err
	}
	return repo.RemovePost(ctx, tx, s.AuthorAccountID)
}

func linkTo(reply, parent content.Status) content.ReplyLink {
	return content.ReplyLink{StatusID: reply.ID, ParentStatusID: parent.ID, ParentAuthorAccountID: parent.AuthorAccountID}
}

// RebuildProjections empties the projections of the content module's statuses and derives
// them again from their sources (data-model §9), in one transaction per projection.
func RebuildProjections(ctx context.Context, u uow.UnitOfWork, repo Repository) error {
	steps := []struct {
		name string
		fn   func(context.Context, uow.Tx) error
	}{
		{"status_heads", repo.RebuildHeads},
		{"status_mention_index", repo.RebuildMentions},
		{"status_reply_links", repo.RebuildReplyLinks},
		{"status_counters", repo.RebuildCounters},
		{"account_post_counters", repo.RebuildPostCounters},
	}
	for _, s := range steps {
		if err := u.Tx(ctx, func(tx uow.Tx) error { return s.fn(ctx, tx) }); err != nil {
			return fmt.Errorf("rebuild %s: %w", s.name, err)
		}
	}
	return nil
}
