// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/content"
)

// The part of this module's public API that the federation module reads to deliver local
// statuses and boosts (docs/architecture/federation.md §7): the statuses, revisions, and boosts
// that outbound activities refer to, and what the visibility policy needs about their authors.
type (
	// Status is content.Status.
	Status = content.Status
	// Revision is content.Revision.
	Revision = content.Revision
	// Boost is content.Boost.
	Boost = content.Boost
	// Favourite is content.Favourite.
	Favourite = content.Favourite
)

// ErrNotBoostable is content.ErrNotBoostable: a status the account may not boost.
var ErrNotBoostable = content.ErrNotBoostable

// The values of the visibility policy's types that delivery and ActivityPub fetches use.
const (
	ViewerRemote       = content.ViewerRemote
	ContextDelivery    = content.ContextDelivery
	ContextObjectFetch = content.ContextObjectFetch
	FollowPending      = content.FollowPending
)

// StatusURI returns the ActivityPub id of a local status on host (ADR-0018).
func StatusURI(host, statusID string) string { return content.StatusURI(host, statusID) }

// ActivityURI returns the IRI of an activity of this instance other than Create (ADR-0018).
func ActivityURI(host, id string) string { return content.ActivityURI(host, id) }

// FindStatus returns a status, deleted or not. It fails with ErrNotFound when there is none.
func (s *Service) FindStatus(ctx context.Context, id string) (Status, error) {
	return s.d.Repository.FindStatus(ctx, id)
}

// LiveRevision returns a status that is not deleted with one of its revisions, the revision an
// outbound Create or Update presents. A missing or deleted status, and a revision of another
// status, fail with ErrNotFound.
func (s *Service) LiveRevision(ctx context.Context, statusID, revisionID string) (Status, Revision, error) {
	st, err := s.d.Repository.FindStatus(ctx, statusID)
	if err != nil {
		return Status{}, Revision{}, err
	}
	switch _, err := s.d.Repository.FindDeletion(ctx, statusID); {
	case err == nil:
		return Status{}, Revision{}, fmt.Errorf("status %s: %w", statusID, ErrNotFound)
	case !errors.Is(err, ErrNotFound):
		return Status{}, Revision{}, err
	}
	rev, err := s.d.Repository.FindRevision(ctx, revisionID)
	if err != nil {
		return Status{}, Revision{}, err
	}
	if rev.StatusID != statusID {
		return Status{}, Revision{}, fmt.Errorf("revision %s of status %s: %w", revisionID, statusID, ErrNotFound)
	}
	return st, rev, nil
}

// AuthorRestrictions returns the restrictions of an author that bear on who sees its statuses,
// as CanView takes them: the moderation restrictions in effect, and profile_view for an author
// that is missing, suspended, or deleted (data-model §5.1).
func (s *Service) AuthorRestrictions(ctx context.Context, accountID string) (AuthorRestrictions, error) {
	r, err := s.d.Restrictions.Restrictions(ctx, accountID)
	if err != nil {
		return AuthorRestrictions{}, err
	}
	out := r.Author
	author, err := s.d.Accounts.FindByID(ctx, accountID)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		out.ProfileView = true
	case err != nil:
		return AuthorRestrictions{}, err
	default:
		out.ProfileView = out.ProfileView || !author.Lifecycle.Active()
	}
	return out, nil
}

// FindBoost returns a boost, retracted or not. It fails with ErrNotFound when there is none.
func (i *Interactions) FindBoost(ctx context.Context, id string) (Boost, error) {
	return i.d.Interactions.FindBoost(ctx, id)
}

// EffectiveVisibility is content.EffectiveVisibility: the visibility a status is judged and
// addressed by, min(original, the author's ceiling) (data-model §5.4).
func EffectiveVisibility(original Visibility, author AuthorRestrictions) Visibility {
	return content.EffectiveVisibility(original, author)
}

// ObjectStatus returns a status with the revision its ActivityPub object presents, and whether
// it is deleted (federation.md §10). The revision of a live status is its current one; that of
// a deleted status is its last one, which still tells whom the status mentioned, and is the zero
// Revision once the purge has removed it. It fails with ErrNotFound when there is no status.
func (s *Service) ObjectStatus(ctx context.Context, id string) (Status, Revision, bool, error) {
	st, err := s.d.Repository.FindStatus(ctx, id)
	if err != nil {
		return Status{}, Revision{}, false, err
	}
	switch _, err := s.d.Repository.FindDeletion(ctx, id); {
	case errors.Is(err, ErrNotFound):
		rev, err := s.d.Repository.FindCurrentRevision(ctx, id)
		if err != nil {
			return Status{}, Revision{}, false, err
		}
		return st, rev, false, nil
	case err != nil:
		return Status{}, Revision{}, false, err
	}
	revs, err := s.d.Repository.ListRevisions(ctx, id)
	if err != nil || len(revs) == 0 {
		return st, Revision{}, true, err
	}
	rev, err := s.d.Repository.FindRevision(ctx, revs[len(revs)-1].ID)
	if errors.Is(err, ErrNotFound) {
		return st, Revision{}, true, nil
	}
	return st, rev, true, err
}

// PostCount returns the number of statuses of an account that are not deleted, as its outbox
// reports it.
func (s *Service) PostCount(ctx context.Context, accountID string) (int64, error) {
	c, err := s.d.Repository.FindPostCounter(ctx, accountID)
	if errors.Is(err, ErrNotFound) {
		return 0, nil
	}
	return c.StatusesCount, err
}
