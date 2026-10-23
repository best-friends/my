// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"time"

	appmoderation "github.com/mycroblog/my/internal/app/moderation"
	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/internal/domain/identity"
)

// InviteStatus is an invite with how it has been used.
type InviteStatus struct {
	identity.Invite
	Uses    int
	Revoked bool
	// State is what the invite allowed when the Service read it (StateAt); the repository
	// leaves it empty.
	State InviteState
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
	auditID := s.d.IDs.NewID()
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertInvite(ctx, tx, inv); err != nil {
			return err
		}
		return tx.Emit(ctx, inviteAudit(auditID, appmoderation.AuditInviteCreated, inv, in.CreatedByUserID, now))
	})
	if err != nil {
		return identity.Invite{}, err
	}
	return inv, nil
}

// inviteAuditCodeChars is how many characters of an invite's code its audit log label keeps.
const inviteAuditCodeChars = 4

// inviteAudit returns the audit log entry of an operation on an invite (FR-704), performed by
// byUserID ("" for the CLI). The label is the first characters of the code followed by an
// ellipsis: enough to tell the invite in the list of invites, while the whole code, which lets
// anyone sign up, stays with those who manage the invites. The source is the invite, whose ID is
// also the key of its revocation.
func inviteAudit(id string, action appmoderation.AuditAction, inv identity.Invite, byUserID string, at time.Time) appmoderation.AuditRecorded {
	actor, user := appmoderation.AuditActor(byUserID)
	label := inv.Code
	if len(label) > inviteAuditCodeChars {
		label = label[:inviteAuditCodeChars] + "…"
	}
	return appmoderation.AuditRecorded{Entry: appmoderation.AuditEntry{
		ID: id, ActorKind: actor, ActorUserID: user, Action: action, TargetKind: appmoderation.AuditTargetInvite,
		TargetID: inv.ID, TargetLabel: label, SourceEventID: inv.ID, OccurredAt: at,
	}}
}

// ListInvites returns invites with their usage, newest first, at most limit (1 to
// MaxListLimit) of them. A before other than "" is the ID of the last invite of the previous
// page: the list starts after it.
func (s *Service) ListInvites(ctx context.Context, before string, limit int) ([]InviteStatus, error) {
	if limit < 1 || limit > MaxListLimit {
		return nil, fmt.Errorf("identity: limit must be 1 to %d", MaxListLimit)
	}
	list, err := s.d.Repository.ListInvites(ctx, before, limit)
	if err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	for i := range list {
		list[i].State = list[i].StateAt(now)
	}
	return list, nil
}

// InviteState is what an invite allows now.
type InviteState string

// Invite states, in the order StateAt checks them.
const (
	InviteUsable    InviteState = "usable"
	InviteRevoked   InviteState = "revoked"
	InviteExpired   InviteState = "expired"
	InviteExhausted InviteState = "exhausted" // used as many times as it allows
)

// StateAt returns what the invite allows at now: a revoked invite is revoked even after it
// expired, and an expired one expired even when it is used up.
func (s InviteStatus) StateAt(now time.Time) InviteState {
	switch {
	case s.Revoked:
		return InviteRevoked
	case s.Expired(now):
		return InviteExpired
	case s.Exhausted(s.Uses):
		return InviteExhausted
	}
	return InviteUsable
}

// RevokeInviteByID stops the invite with the ID from working, as RevokeInvite does, and returns
// it with its usage. It fails with ErrNotFound when no invite has the ID.
func (s *Service) RevokeInviteByID(ctx context.Context, id, revokedByUserID string) (InviteStatus, error) {
	inv, err := s.d.Repository.FindInviteByID(ctx, id)
	if err != nil {
		return InviteStatus{}, err
	}
	out := InviteStatus{Invite: inv}
	rev := identity.InviteRevocation{InviteID: inv.ID, RevokedByUserID: revokedByUserID, RevokedAt: s.d.Clock.Now()}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.revokeInvite(ctx, tx, inv, rev, auditID); err != nil {
			return err
		}
		out.Uses, out.Revoked, err = s.d.Repository.InviteUsage(ctx, tx, inv.ID)
		return err
	})
	if err != nil {
		return InviteStatus{}, err
	}
	out.State = out.StateAt(rev.RevokedAt)
	return out, nil
}

// revokeInvite records the revocation of inv in tx with its audit log entry, unless the invite
// is already revoked: revoking it again changes nothing, so it is not audited again.
func (s *Service) revokeInvite(ctx context.Context, tx uow.Tx, inv identity.Invite, rev identity.InviteRevocation, auditID string) error {
	_, revoked, err := s.d.Repository.InviteUsage(ctx, tx, inv.ID)
	if err != nil || revoked {
		return err
	}
	if err := s.d.Repository.InsertInviteRevocation(ctx, tx, rev); err != nil {
		return err
	}
	return tx.Emit(ctx, inviteAudit(auditID, appmoderation.AuditInviteRevoked, inv, rev.RevokedByUserID, rev.RevokedAt))
}

// RevokeInvite stops an invite from working. Revoking it again keeps the first revocation. It
// fails with ErrNotFound when no invite has the code.
func (s *Service) RevokeInvite(ctx context.Context, code, revokedByUserID string) (identity.Invite, error) {
	inv, err := s.d.Repository.FindInviteByCode(ctx, identity.NormalizeInviteCode(code))
	if err != nil {
		return identity.Invite{}, err
	}
	rev := identity.InviteRevocation{InviteID: inv.ID, RevokedByUserID: revokedByUserID, RevokedAt: s.d.Clock.Now()}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.revokeInvite(ctx, tx, inv, rev, auditID) })
	if err != nil {
		return identity.Invite{}, err
	}
	return inv, nil
}
