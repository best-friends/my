// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/content"
)

// ViewDeps are the ports Views uses.
type ViewDeps struct {
	Statuses      Repository
	Interactions  InteractionRepository
	Accounts      Accounts
	Relationships Relationships
	// Restrictions are the authors' moderation restrictions; nil means NoRestrictions.
	Restrictions RestrictionPolicy
}

// Views reads statuses as the client API shows them: with their counters, hashtags, reply
// targets, and what the viewer has done with them. Statuses and Boosts do not decide who may
// see a status; their callers have done that, as the timeline module does with CanView. Pinned
// decides it itself.
type Views struct {
	d ViewDeps
}

// NewViews returns Views on d.
func NewViews(d ViewDeps) *Views {
	if d.Restrictions == nil {
		d.Restrictions = NoRestrictions{}
	}
	return &Views{d: d}
}

// StatusView is a status that is not deleted with what clients show of it.
type StatusView struct {
	Post
	Counters content.Counters
	// Tags are the hashtags of the current revision, in the order of Revision.TagIDs.
	Tags []content.Tag
	// InReplyTo links a reply to its parent; nil for a status that is not a reply or whose
	// parent is unknown or deleted.
	InReplyTo *content.ReplyLink
	// Viewer is what the viewer has done with the status, nil for an anonymous viewer.
	Viewer *ViewerState
}

// ViewerState is what a local account has done with a status.
type ViewerState struct {
	Favourited bool
	Boosted    bool
	Bookmarked bool
	// Pinned is true when the status is the viewer's own and pinned to the viewer's profile.
	Pinned bool
}

// Statuses returns the statuses of ids that are not deleted, in the order of ids, as
// viewerAccountID ("" for an anonymous viewer) sees them. Missing and deleted statuses are left
// out.
func (v *Views) Statuses(ctx context.Context, viewerAccountID string, ids []string) ([]StatusView, error) {
	out := make([]StatusView, 0, len(ids))
	tags := map[string]content.Tag{}
	for _, id := range ids {
		sv, err := v.status(ctx, viewerAccountID, id, tags)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, nil
}

func (v *Views) status(ctx context.Context, viewerAccountID, id string, tags map[string]content.Tag) (StatusView, error) {
	repo := v.d.Statuses
	st, err := repo.FindStatus(ctx, id)
	if err != nil {
		return StatusView{}, err
	}
	// A deleted status has no current revision.
	rev, err := repo.FindCurrentRevision(ctx, id)
	if err != nil {
		return StatusView{}, err
	}
	sv := StatusView{Post: Post{Status: st, Revision: rev}, Counters: content.Counters{StatusID: id}}
	switch c, err := repo.FindCounters(ctx, id); {
	case err == nil:
		sv.Counters = c
	case !errors.Is(err, ErrNotFound):
		return StatusView{}, err
	}
	for _, tagID := range rev.TagIDs {
		t, ok := tags[tagID]
		if !ok {
			if t, err = repo.FindTag(ctx, tagID); err != nil {
				return StatusView{}, fmt.Errorf("tag of status %s: %w", id, err)
			}
			tags[tagID] = t
		}
		sv.Tags = append(sv.Tags, t)
	}
	if st.InReplyToURI != "" {
		switch l, err := repo.FindReplyLink(ctx, id); {
		case err == nil:
			sv.InReplyTo = &l
		case !errors.Is(err, ErrNotFound):
			return StatusView{}, err
		}
	}
	if viewerAccountID != "" {
		if sv.Viewer, err = v.viewerState(ctx, viewerAccountID, st); err != nil {
			return StatusView{}, err
		}
	}
	return sv, nil
}

func (v *Views) viewerState(ctx context.Context, accountID string, st content.Status) (*ViewerState, error) {
	repo := v.d.Interactions
	var s ViewerState
	var err error
	if s.Favourited, err = found(repo.FindFavourite(ctx, accountID, st.ID)); err != nil {
		return nil, err
	}
	if s.Boosted, err = found(repo.FindBoostEdge(ctx, accountID, st.ID)); err != nil {
		return nil, err
	}
	if s.Bookmarked, err = found(repo.FindBookmark(ctx, accountID, st.ID)); err != nil {
		return nil, err
	}
	if st.AuthorAccountID == accountID {
		if s.Pinned, err = found(repo.FindPin(ctx, accountID, st.ID)); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

// found turns a lookup into whether it found a row.
func found[T any](_ T, err error) (bool, error) {
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Boosts returns the boosts of ids, in their order; missing ones are left out. A retracted
// boost is still returned: the timeline module drops it from its projections.
func (v *Views) Boosts(ctx context.Context, ids []string) ([]content.Boost, error) {
	out := make([]content.Boost, 0, len(ids))
	for _, id := range ids {
		b, err := v.d.Interactions.FindBoost(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// Tag returns the hashtag name, matched under NormalizeTag, failing with ErrNotFound when no
// status ever used it.
func (v *Views) Tag(ctx context.Context, name string) (content.Tag, error) {
	return v.d.Statuses.FindTagByName(ctx, NormalizeTag(name))
}

// Pinned returns the IDs of the statuses that accountID pins to its profile and that viewer may
// see there (content.CanView in ContextProfile), newest pin first.
func (v *Views) Pinned(ctx context.Context, viewer Viewer, accountID string) ([]string, error) {
	pins, err := v.d.Interactions.ListPins(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range pins {
		_, err := statusVisibility{v.d.Statuses, v.d.Interactions, v.d.Accounts, v.d.Relationships, v.d.Restrictions}.
			visible(ctx, content.ContextProfile, viewer, p.StatusID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, p.StatusID)
	}
	return out, nil
}

// statusVisibility decides with content.CanView whether a viewer may see one status.
type statusVisibility struct {
	statuses      Repository
	interactions  InteractionRepository
	accounts      Accounts
	relationships Relationships
	restrictions  RestrictionPolicy
}

// visible returns statusID when viewer may see it in vctx. A status that is missing, deleted,
// or hidden from the viewer fails with ErrNotFound.
func (sv statusVisibility) visible(ctx context.Context, vctx content.ViewContext, viewer content.Viewer, statusID string) (content.Status, error) {
	hidden := fmt.Errorf("status %s: %w", statusID, ErrNotFound)
	st, err := sv.statuses.FindStatus(ctx, statusID)
	if err != nil {
		return content.Status{}, err
	}
	switch _, err := sv.statuses.FindDeletion(ctx, statusID); {
	case err == nil:
		return content.Status{}, hidden
	case !errors.Is(err, ErrNotFound):
		return content.Status{}, err
	}

	r, err := sv.restrictions.Restrictions(ctx, st.AuthorAccountID)
	if err != nil {
		return content.Status{}, err
	}
	restrictions := r.Author
	author, err := sv.accounts.FindByID(ctx, st.AuthorAccountID)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		restrictions.ProfileView = true
	case err != nil:
		return content.Status{}, err
	default:
		// A suspended or deleted author is shown to no one else (data-model §5.1).
		restrictions.ProfileView = restrictions.ProfileView || !author.Lifecycle.Active()
	}

	var rel content.Relation
	if viewer.Kind == content.ViewerLocal && viewer.AccountID != "" && st.AuthorAccountID != viewer.AccountID {
		r, err := sv.relationships.Relationship(ctx, viewer.AccountID, st.AuthorAccountID)
		if err != nil {
			return content.Status{}, err
		}
		rel.Blocking, rel.BlockedBy = r.Blocking, r.BlockedBy
		switch {
		case r.ActivelyFollowing():
			rel.Follow = content.FollowActive
		case r.RequestedFollow():
			rel.Follow = content.FollowPending
		}
		if rel.Mentioned, err = sv.interactions.Mentioned(ctx, st.ID, viewer.AccountID); err != nil {
			return content.Status{}, err
		}
	}

	facts := content.StatusFacts{AuthorAccountID: st.AuthorAccountID, AuthorIsLocal: st.IsLocal, Visibility: st.Visibility}
	if !content.CanView(vctx, viewer, facts, restrictions, rel) {
		return content.Status{}, hidden
	}
	return st, nil
}
