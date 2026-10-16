// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// InviteStatus is an invite with how it has been used.
type InviteStatus struct {
	identity.Invite
	Uses    int
	Revoked bool
}

// Usable reports whether the invite still lets someone sign up at now.
func (s InviteStatus) Usable(now time.Time) bool {
	return !s.Revoked && !s.Expired(now) && !s.Exhausted(s.Uses)
}

// CreateInviteInput describes a new invite.
type CreateInviteInput struct {
	// CreatedByUserID is the user creating the invite, or "" for the CLI.
	CreatedByUserID string
	// MaxUses is how many sign-ups the invite allows; 0 means no limit.
	MaxUses int
	// ExpiresIn is how long the invite works; 0 means forever.
	ExpiresIn time.Duration
}

// MaxInviteUses bounds CreateInviteInput.MaxUses (the integer column of invites.max_uses).
const MaxInviteUses = 1_000_000

// newInviteCode returns InviteCodeLength characters of base32 from 80 random bits.
func newInviteCode() string {
	b := make([]byte, identity.InviteCodeLength*5/8)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// CreateInvite creates an invite with a new random code.
func (s *Service) CreateInvite(ctx context.Context, in CreateInviteInput) (identity.Invite, error) {
	if in.MaxUses < 0 || in.MaxUses > MaxInviteUses {
		return identity.Invite{}, fmt.Errorf("identity: max uses must be 0 (no limit) to %d", MaxInviteUses)
	}
	if in.ExpiresIn < 0 {
		return identity.Invite{}, errors.New("identity: the expiry must not be negative")
	}
	now := s.d.Clock.Now()
	inv := identity.Invite{
		ID: s.d.IDs.NewID(), Code: newInviteCode(), CreatedByUserID: in.CreatedByUserID, MaxUses: in.MaxUses, CreatedAt: now,
	}
	if in.ExpiresIn > 0 {
		at := now.Add(in.ExpiresIn)
		inv.ExpiresAt = &at
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.d.Repository.InsertInvite(ctx, tx, inv) })
	if err != nil {
		return identity.Invite{}, err
	}
	return inv, nil
}

// ListInvites returns invites with their usage, newest first, at most limit (1 to
// MaxListLimit) of them.
func (s *Service) ListInvites(ctx context.Context, limit int) ([]InviteStatus, error) {
	if limit < 1 || limit > MaxListLimit {
		return nil, fmt.Errorf("identity: limit must be 1 to %d", MaxListLimit)
	}
	return s.d.Repository.ListInvites(ctx, limit)
}

// RevokeInvite stops an invite from working. Revoking it again keeps the first revocation. It
// fails with ErrNotFound when no invite has the code.
func (s *Service) RevokeInvite(ctx context.Context, code, revokedByUserID string) (identity.Invite, error) {
	inv, err := s.d.Repository.FindInviteByCode(ctx, identity.NormalizeInviteCode(code))
	if err != nil {
		return identity.Invite{}, err
	}
	rev := identity.InviteRevocation{InviteID: inv.ID, RevokedByUserID: revokedByUserID, RevokedAt: s.d.Clock.Now()}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.d.Repository.InsertInviteRevocation(ctx, tx, rev) })
	if err != nil {
		return identity.Invite{}, err
	}
	return inv, nil
}
