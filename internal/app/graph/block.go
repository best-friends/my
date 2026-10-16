// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"context"
	"errors"
	"fmt"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/graph"
)

// Block makes blockerID block blockedID (FR-303). In the same transaction it ends the follows
// between them in both directions, pending or active, with the reason blocked, and while the
// block is in force neither account can follow the other (Follow fails with ErrBlocked).
// Blocking an account that the blocker already blocks changes nothing. The blocker must be
// local and active; the blocked account must exist and be local.
func (s *Service) Block(ctx context.Context, blockerID, blockedID string) (Relationship, error) {
	b := graph.Block{BlockerAccountID: blockerID, BlockedAccountID: blockedID}
	if err := b.Validate(); err != nil {
		return Relationship{}, err
	}
	if _, err := s.localActive(ctx, blockerID); err != nil {
		return Relationship{}, err
	}
	target, err := s.existing(ctx, blockedID)
	if err != nil {
		return Relationship{}, err
	}
	if !target.Account.IsLocal() {
		return Relationship{}, fmt.Errorf("account %s: %w", blockedID, ErrRemoteAccount)
	}
	b.ID = s.d.IDs.NewID()
	b.URI = graph.BlockURI(s.d.Host, b.ID)
	b.CreatedAt = s.d.Clock.Now()

	repo := s.d.Repository
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := repo.LockPair(ctx, tx, blockerID, blockedID); err != nil {
			return err
		}
		if _, err := repo.BlockEdge(ctx, tx, blockerID, blockedID); !errors.Is(err, ErrNotFound) {
			return err // nil: already blocked
		}
		if err := repo.InsertBlock(ctx, tx, b); err != nil {
			return err
		}
		if err := s.terminateIn(ctx, tx, blockerID, blockedID, graph.TerminatedBlocked, false, b.CreatedAt); err != nil {
			return err
		}
		if err := s.terminateIn(ctx, tx, blockedID, blockerID, graph.TerminatedBlocked, false, b.CreatedAt); err != nil {
			return err
		}
		return tx.Emit(ctx, Blocked{Block: b})
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, blockerID, blockedID)
}

// Unblock lifts blockerID's block of blockedID. The follows that the block ended stay ended.
// Without a block it changes nothing.
func (s *Service) Unblock(ctx context.Context, blockerID, blockedID string) (Relationship, error) {
	now := s.d.Clock.Now()
	repo := s.d.Repository
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		edge, err := repo.BlockEdge(ctx, tx, blockerID, blockedID)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		b, err := repo.Block(ctx, tx, edge.BlockID)
		if err != nil {
			return err
		}
		r := graph.BlockRetraction{BlockID: b.ID, RetractedAt: now}
		if err := repo.InsertBlockRetraction(ctx, tx, r); err != nil {
			return err
		}
		return tx.Emit(ctx, Unblocked{Block: b, Retraction: r})
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, blockerID, blockedID)
}

// Blocks returns the accounts accountID blocks, newest block first.
func (s *Service) Blocks(ctx context.Context, accountID string, p Page) ([]graph.BlockEdge, error) {
	return s.d.Repository.ListBlocking(ctx, accountID, p.Before, p.limit())
}

// checkNotBlocked fails with ErrBlocked when either account blocks the other.
func (s *Service) checkNotBlocked(ctx context.Context, tx uow.Tx, a, b string) error {
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		_, err := s.d.Repository.BlockEdge(ctx, tx, pair[0], pair[1])
		if err == nil {
			return fmt.Errorf("%s blocks %s: %w", pair[0], pair[1], ErrBlocked)
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

// existing returns an account that exists, local or remote, in any lifecycle state.
func (s *Service) existing(ctx context.Context, id string) (appaccount.Actor, error) {
	a, err := s.d.Accounts.FindByID(ctx, id)
	if errors.Is(err, appaccount.ErrNotFound) {
		return appaccount.Actor{}, fmt.Errorf("account %s: %w", id, ErrAccountUnavailable)
	}
	return a, err
}

// MuteInput is the account's settings of a mute.
type MuteInput struct {
	// HideNotifications also hides the target's notifications.
	HideNotifications bool
	// Duration is how long the mute lasts; zero mutes until unmuted. A non-zero duration must be
	// at least one second (graph.ErrMuteExpiry).
	Duration time.Duration
}

// Mute makes accountID mute targetID (FR-303), replacing the settings and expiry of an existing
// mute of the target. A mute with a duration stops applying when it expires, without any
// write: Relationship and Mutes leave it out from then on. The account must be local and
// active; the target must exist and may be remote.
func (s *Service) Mute(ctx context.Context, accountID, targetID string, in MuteInput) (Relationship, error) {
	m := graph.Mute{
		AccountID: accountID, TargetAccountID: targetID, HideNotifications: in.HideNotifications,
		CreatedAt: s.d.Clock.Now(),
	}
	if in.Duration != 0 {
		if in.Duration < time.Second {
			return Relationship{}, graph.ErrMuteExpiry
		}
		expires := m.CreatedAt.Add(in.Duration)
		m.ExpiresAt = &expires
	}
	if err := m.Validate(); err != nil {
		return Relationship{}, err
	}
	if _, err := s.localActive(ctx, accountID); err != nil {
		return Relationship{}, err
	}
	if _, err := s.existing(ctx, targetID); err != nil {
		return Relationship{}, err
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return s.d.Repository.UpsertMute(ctx, tx, m)
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, accountID, targetID)
}

// Unmute removes accountID's mute of targetID. Without a mute it changes nothing.
func (s *Service) Unmute(ctx context.Context, accountID, targetID string) (Relationship, error) {
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return s.d.Repository.DeleteMute(ctx, tx, accountID, targetID)
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, accountID, targetID)
}

// MutePage selects a page of a list of mutes ordered newest first.
type MutePage struct {
	// Before is the position of the last mute of the previous page (MuteCursorOf); the zero
	// cursor starts at the newest.
	Before MuteCursor
	// Limit is the page size, as in Page.
	Limit int
}

// Mutes returns accountID's mutes that have not expired, newest first.
func (s *Service) Mutes(ctx context.Context, accountID string, p MutePage) ([]graph.Mute, error) {
	return s.d.Repository.ListMutes(ctx, accountID, s.d.Clock.Now(), p.Before, Page{Limit: p.Limit}.limit())
}

// HidesNotifications reports, inside tx, whether recipientID is not to be notified of what
// sourceID does: either account blocks the other, or recipientID mutes sourceID with
// HideNotifications and the mute has not expired. The notification module's projectors call it
// in the transaction of the event they project, so that it sees that transaction's writes.
func (s *Service) HidesNotifications(ctx context.Context, tx uow.Tx, recipientID, sourceID string) (bool, error) {
	err := s.checkNotBlocked(ctx, tx, recipientID, sourceID)
	if errors.Is(err, ErrBlocked) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	m, err := s.d.Repository.Mute(ctx, tx, recipientID, sourceID, s.d.Clock.Now())
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return m.HideNotifications, nil
}
