// SPDX-License-Identifier: AGPL-3.0-only

// Package app holds the use cases. It depends only on store ports.
package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

// ErrSelfFollow is returned when an account tries to follow itself.
var ErrSelfFollow = errors.New("app: cannot follow self")

// Follows implements the Follow / Unfollow use cases.
type Follows struct {
	UoW   store.UnitOfWork
	Now   func() time.Time // defaults to time.Now
	NewID func() string    // defaults to stdlib uuid.NewV7
}

func (s *Follows) now() time.Time {
	if s.Now != nil {
		return store.Timestamp(s.Now())
	}
	return store.Timestamp(time.Now())
}

func (s *Follows) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewV7().String()
}

// Follow makes followerID follow targetID. It is idempotent: a repeated call
// returns created=false and leaves the counters unchanged.
func (s *Follows) Follow(ctx context.Context, followerID, targetID string) (created bool, err error) {
	if followerID == targetID {
		return false, ErrSelfFollow
	}
	err = s.UoW.Tx(ctx, func(tx store.Tx) error {
		created = false // fn may run more than once; reset captured results
		if _, err := tx.Accounts().Get(ctx, followerID); err != nil {
			return err
		}
		if _, err := tx.Accounts().Get(ctx, targetID); err != nil {
			return err
		}
		ok, err := tx.Follows().Insert(ctx, store.Follow{
			ID: s.newID(), FollowerID: followerID, TargetID: targetID, CreatedAt: s.now(),
		})
		if err != nil || !ok {
			return err
		}
		if err := tx.Accounts().AddFollowCounts(ctx, targetID, +1, 0); err != nil {
			return err
		}
		if err := tx.Accounts().AddFollowCounts(ctx, followerID, 0, +1); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

// Unfollow removes the follow. It is idempotent.
func (s *Follows) Unfollow(ctx context.Context, followerID, targetID string) (deleted bool, err error) {
	err = s.UoW.Tx(ctx, func(tx store.Tx) error {
		deleted = false
		ok, err := tx.Follows().Delete(ctx, followerID, targetID)
		if err != nil || !ok {
			return err
		}
		if err := tx.Accounts().AddFollowCounts(ctx, targetID, -1, 0); err != nil {
			return err
		}
		if err := tx.Accounts().AddFollowCounts(ctx, followerID, 0, -1); err != nil {
			return err
		}
		deleted = true
		return nil
	})
	return deleted, err
}
