// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// ErrActorUnavailable reports an acting account that does not exist, is not local, or is not
// active: interactions are commands of local accounts, and federated ones go through the
// federation module.
var ErrActorUnavailable = errors.New("content: the acting account does not exist, is not local, or is not active")

// InteractionDeps are the ports and settings an Interactions service uses.
type InteractionDeps struct {
	UnitOfWork    uow.UnitOfWork
	Statuses      Repository
	Interactions  InteractionRepository
	Accounts      Accounts
	Relationships Relationships
	// Restrictions are the authors' moderation restrictions; nil means NoRestrictions.
	Restrictions RestrictionPolicy
	Clock        kernel.Clock
	IDs          kernel.IDGenerator
	// Host is the serving domain of the Announce and Like activity URIs (ADR-0018).
	Host string
}

// Interactions runs the use cases of local accounts acting on statuses (FR-107, FR-113):
// boosting, favouriting, bookmarking, and pinning, and undoing each. Every command is safe to
// repeat: doing what is already done, or undoing what is not, changes nothing and emits no
// event, so the counters never move twice.
//
// An account may act only on a status it can see: each command except the undoing ones checks
// the status with content.CanView in ContextStatus, and a status the account cannot see, or a
// deleted one, fails with ErrNotFound, the same as a missing one, so that its existence does
// not leak (docs/architecture/visibility.md).
type Interactions struct {
	d InteractionDeps
}

// NewInteractions returns an Interactions service on d.
func NewInteractions(d InteractionDeps) *Interactions {
	if d.Restrictions == nil {
		d.Restrictions = NoRestrictions{}
	}
	return &Interactions{d: d}
}

// Boost makes accountID boost statusID and returns the live boost. Boosting a status that the
// account already boosts returns the existing boost. The boost copies the status's visibility;
// a boost of a local status never federates (content.Boost.Federates, ADR-0017). A status whose
// visibility does not allow the account to boost it fails with content.ErrNotBoostable.
func (s *Interactions) Boost(ctx context.Context, accountID, statusID string) (content.Boost, error) {
	st, err := s.visibleStatus(ctx, accountID, statusID)
	if err != nil {
		return content.Boost{}, err
	}
	if !st.Visibility.Boostable(st.AuthorAccountID == accountID) {
		return content.Boost{}, fmt.Errorf("boost %s: %w", statusID, content.ErrNotBoostable)
	}
	id := s.d.IDs.NewID()
	b := content.Boost{
		ID: id, URI: content.ActivityURI(s.d.Host, id), BoosterAccountID: accountID, StatusID: st.ID,
		Visibility: st.Visibility, IsLocal: true, BoostedAt: s.d.Clock.Now(),
	}
	var out content.Boost
	repo := s.d.Interactions
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		out = content.Boost{}
		if err := s.ensureLive(ctx, tx, st.ID); err != nil {
			return err
		}
		edge, err := repo.BoostEdge(ctx, tx, accountID, st.ID)
		switch {
		case err == nil:
			out, err = repo.Boost(ctx, tx, edge.BoostID)
			return err
		case !errors.Is(err, ErrNotFound):
			return err
		}
		if err := repo.InsertBoost(ctx, tx, b); err != nil {
			return err
		}
		out = b
		return tx.Emit(ctx, Boosted{Boost: b, Status: RefOf(st)})
	})
	if err != nil {
		return content.Boost{}, err
	}
	return out, nil
}

// Unboost retracts accountID's live boost of statusID. Without one it changes nothing.
func (s *Interactions) Unboost(ctx context.Context, accountID, statusID string) error {
	st, err := s.d.Statuses.FindStatus(ctx, statusID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := s.d.Clock.Now()
	repo := s.d.Interactions
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		edge, err := repo.BoostEdge(ctx, tx, accountID, statusID)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		b, err := repo.Boost(ctx, tx, edge.BoostID)
		if err != nil {
			return err
		}
		r := content.BoostRetraction{BoostID: b.ID, Origin: content.RetractedByBooster, RetractedAt: now}
		err = repo.InsertBoostRetraction(ctx, tx, r)
		if errors.Is(err, ErrBoostRetracted) {
			// A concurrent Unboost won; a new attempt finds the edge gone.
			return uow.MarkRetryable(err)
		}
		if err != nil {
			return err
		}
		return tx.Emit(ctx, BoostRetracted{Boost: b, Retraction: r, Status: RefOf(st)})
	})
}

// Favourite makes accountID favourite statusID and returns the favourite. Favouriting a status
// that the account already favourites returns the existing favourite.
func (s *Interactions) Favourite(ctx context.Context, accountID, statusID string) (content.Favourite, error) {
	st, err := s.visibleStatus(ctx, accountID, statusID)
	if err != nil {
		return content.Favourite{}, err
	}
	id := s.d.IDs.NewID()
	f := content.Favourite{ID: id, AccountID: accountID, StatusID: st.ID, URI: content.ActivityURI(s.d.Host, id), CreatedAt: s.d.Clock.Now()}
	inserted := false
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		inserted = false
		if err := s.ensureLive(ctx, tx, st.ID); err != nil {
			return err
		}
		ok, err := s.d.Interactions.InsertFavourite(ctx, tx, f)
		if err != nil || !ok {
			return err
		}
		inserted = true
		return tx.Emit(ctx, Favourited{Favourite: f, Status: RefOf(st)})
	})
	if err != nil {
		return content.Favourite{}, err
	}
	if inserted {
		return f, nil
	}
	return s.d.Interactions.FindFavourite(ctx, accountID, st.ID)
}

// Unfavourite removes accountID's favourite of statusID. Without one it changes nothing.
func (s *Interactions) Unfavourite(ctx context.Context, accountID, statusID string) error {
	st, err := s.d.Statuses.FindStatus(ctx, statusID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		f, deleted, err := s.d.Interactions.DeleteFavourite(ctx, tx, accountID, statusID)
		if err != nil || !deleted {
			return err
		}
		return tx.Emit(ctx, Unfavourited{Favourite: f, Status: RefOf(st)})
	})
}

// Bookmark makes accountID bookmark statusID (FR-113). Bookmarking a status that the account
// already bookmarks changes nothing.
func (s *Interactions) Bookmark(ctx context.Context, accountID, statusID string) error {
	st, err := s.visibleStatus(ctx, accountID, statusID)
	if err != nil {
		return err
	}
	b := content.Bookmark{ID: s.d.IDs.NewID(), AccountID: accountID, StatusID: st.ID, CreatedAt: s.d.Clock.Now()}
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.ensureLive(ctx, tx, st.ID); err != nil {
			return err
		}
		_, err := s.d.Interactions.InsertBookmark(ctx, tx, b)
		return err
	})
}

// Unbookmark removes accountID's bookmark of statusID. Without one it changes nothing.
func (s *Interactions) Unbookmark(ctx context.Context, accountID, statusID string) error {
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		_, err := s.d.Interactions.DeleteBookmark(ctx, tx, accountID, statusID)
		return err
	})
}

// Pin pins accountID's own statusID to its profile. Pinning a status that is already pinned
// changes nothing. Someone else's status or a direct one fails with content.ErrNotPinnable,
// and a pin beyond content.MaxPins with content.ErrPinLimit.
func (s *Interactions) Pin(ctx context.Context, accountID, statusID string) error {
	st, err := s.visibleStatus(ctx, accountID, statusID)
	if err != nil {
		return err
	}
	if st.AuthorAccountID != accountID || !st.Visibility.Pinnable() {
		return fmt.Errorf("pin %s: %w", statusID, content.ErrNotPinnable)
	}
	p := content.Pin{ID: s.d.IDs.NewID(), AccountID: accountID, StatusID: st.ID, CreatedAt: s.d.Clock.Now()}
	repo := s.d.Interactions
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.ensureLive(ctx, tx, st.ID); err != nil {
			return err
		}
		// A round that inserts nothing although the status is not pinned and a slot is free lost
		// that slot to a concurrent pin, which has committed by then; each later round sees it.
		// Only concurrent unpins that keep freeing slots exhaust the rounds; then the transaction
		// runs again.
		for range content.MaxPins {
			inserted, err := repo.InsertPin(ctx, tx, p)
			if err != nil || inserted {
				return err
			}
			pinned, err := repo.HasPin(ctx, tx, accountID, st.ID)
			if err != nil || pinned {
				return err
			}
			n, err := repo.CountPins(ctx, tx, accountID)
			if err != nil {
				return err
			}
			if n >= content.MaxPins {
				return fmt.Errorf("pin %s: %w", statusID, content.ErrPinLimit)
			}
		}
		return uow.MarkRetryable(fmt.Errorf("pin %s: free slots taken concurrently", statusID))
	})
}

// Unpin unpins accountID's statusID. Without a pin it changes nothing.
func (s *Interactions) Unpin(ctx context.Context, accountID, statusID string) error {
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		_, err := s.d.Interactions.DeletePin(ctx, tx, accountID, statusID)
		return err
	})
}

// visibleStatus returns statusID when accountID is a local, active account that may see it
// (content.CanView in ContextStatus). A status that is missing, deleted, or hidden from the
// account fails with ErrNotFound.
func (s *Interactions) visibleStatus(ctx context.Context, accountID, statusID string) (content.Status, error) {
	viewer, err := s.d.Accounts.FindByID(ctx, accountID)
	if errors.Is(err, appaccount.ErrNotFound) {
		return content.Status{}, fmt.Errorf("account %s: %w", accountID, ErrActorUnavailable)
	}
	if err != nil {
		return content.Status{}, err
	}
	if !viewer.Account.IsLocal() || !viewer.Lifecycle.Active() {
		return content.Status{}, fmt.Errorf("account %s: %w", accountID, ErrActorUnavailable)
	}
	hidden := fmt.Errorf("status %s: %w", statusID, ErrNotFound)

	st, err := s.d.Statuses.FindStatus(ctx, statusID)
	if err != nil {
		return content.Status{}, err
	}
	switch _, err := s.d.Statuses.FindDeletion(ctx, statusID); {
	case err == nil:
		return content.Status{}, hidden
	case !errors.Is(err, ErrNotFound):
		return content.Status{}, err
	}

	r, err := s.d.Restrictions.Restrictions(ctx, st.AuthorAccountID)
	if err != nil {
		return content.Status{}, err
	}
	restrictions := r.Author
	author, err := s.d.Accounts.FindByID(ctx, st.AuthorAccountID)
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
	if st.AuthorAccountID != accountID {
		r, err := s.d.Relationships.Relationship(ctx, accountID, st.AuthorAccountID)
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
		if rel.Mentioned, err = s.d.Interactions.Mentioned(ctx, st.ID, accountID); err != nil {
			return content.Status{}, err
		}
	}

	facts := content.StatusFacts{AuthorAccountID: st.AuthorAccountID, AuthorIsLocal: st.IsLocal, Visibility: st.Visibility}
	v := content.Viewer{Kind: content.ViewerLocal, AccountID: accountID}
	if !content.CanView(content.ContextStatus, v, facts, restrictions, rel) {
		return content.Status{}, hidden
	}
	return st, nil
}

// ensureLive fails with ErrNotFound when the status was deleted after visibleStatus read it.
func (s *Interactions) ensureLive(ctx context.Context, tx uow.Tx, statusID string) error {
	deleted, err := s.d.Statuses.IsDeleted(ctx, tx, statusID)
	if err != nil {
		return err
	}
	if deleted {
		return fmt.Errorf("status %s: %w", statusID, ErrNotFound)
	}
	return nil
}

// Counters returns the replies, boosts, and favourites counts of a status that is not deleted;
// a deleted or missing status fails with ErrNotFound.
func (s *Interactions) Counters(ctx context.Context, statusID string) (content.Counters, error) {
	return s.d.Statuses.FindCounters(ctx, statusID)
}
