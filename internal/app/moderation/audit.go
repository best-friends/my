// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

// The audit log (FR-704, ADR-0037). The administrative use cases of every module emit
// AuditRecorded in the transaction of the operation, after its source event; the projector
// that RegisterAuditProjector registers writes the entry in that transaction, so an operation
// and its entry commit or roll back together. Nothing updates or deletes an entry: there is no
// use case for it, and only the retention cleanup (ADR-0038) removes old entries.

// AuditEntry is moderation.AuditEntry, an entry of the audit log as other modules emit it.
type AuditEntry = moderation.AuditEntry

// AuditAction is moderation.AuditAction.
type AuditAction = moderation.AuditAction

// AuditTargetKind is moderation.AuditTargetKind.
type AuditTargetKind = moderation.AuditTargetKind

// AuditActorKind is moderation.AuditActorKind.
type AuditActorKind = moderation.AuditActorKind

// The audited operations and their targets that other modules emit.
const (
	AuditRegistrationApproved = moderation.AuditRegistrationApproved
	AuditRegistrationRejected = moderation.AuditRegistrationRejected
	AuditSettingChanged       = moderation.AuditSettingChanged
	AuditTextChanged          = moderation.AuditTextChanged
	AuditRuleAdded            = moderation.AuditRuleAdded
	AuditRuleRevised          = moderation.AuditRuleRevised
	AuditRuleMoved            = moderation.AuditRuleMoved
	AuditRuleRetired          = moderation.AuditRuleRetired

	AuditTargetUser    = moderation.AuditTargetUser
	AuditTargetSetting = moderation.AuditTargetSetting
	AuditTargetRule    = moderation.AuditTargetRule
)

// AuditActor returns the actor of an operation given the user ID that the use cases take: the
// user, or the CLI for "".
func AuditActor(userID string) (AuditActorKind, string) { return moderation.AuditActor(userID) }

// EventAuditRecorded follows every administrative operation, in its transaction.
const EventAuditRecorded = "moderation.audit_recorded"

// AuditRecorded carries the audit log entry of an operation. Emitters generate its ID before the
// transaction, like the IDs of the operation's rows, so that a retry writes the same entry.
type AuditRecorded struct{ Entry AuditEntry }

// EventType implements uow.Event.
func (AuditRecorded) EventType() string { return EventAuditRecorded }

// AuditRepository is the persistence port of the audit log, part of Repository.
type AuditRepository interface {
	// InsertAuditEntry appends an entry. There is no method to update or delete one.
	InsertAuditEntry(ctx context.Context, tx uow.Tx, e moderation.AuditEntry) error
	// ListAuditEntries returns up to limit entries matching q, newest first.
	ListAuditEntries(ctx context.Context, q AuditQuery, limit int) ([]moderation.AuditEntry, error)
}

// AuditQuery selects the entries of the audit log. Empty fields select every entry.
type AuditQuery struct {
	// ActorUserID keeps the operations of one user.
	ActorUserID string
	// Action keeps one kind of operation.
	Action moderation.AuditAction
	// TargetKind keeps the operations on one kind of target, and TargetID, which needs
	// TargetKind, on one target.
	TargetKind moderation.AuditTargetKind
	TargetID   string
	// Before is the ID of the last entry of the previous page; "" starts from the newest.
	Before string
}

// RegisterAuditProjector registers the projector that writes the audit log entry of every
// AuditRecorded into the transaction that emits it. An invalid entry fails the operation: an
// operation that cannot be audited does not happen.
func RegisterAuditProjector(r *uow.Projectors, repo AuditRepository) {
	r.Register(EventAuditRecorded, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		entry := e.(AuditRecorded).Entry
		entry.TargetLabel = moderation.TruncateAuditLabel(entry.TargetLabel)
		if err := entry.Validate(); err != nil {
			return err
		}
		return repo.InsertAuditEntry(ctx, tx, entry)
	})
}

// AuditLog lists up to limit entries of the audit log, newest first. It fails with
// ErrInvalidQuery for an unknown action or target kind, or a target ID without its kind.
func (s *Service) AuditLog(ctx context.Context, q AuditQuery, limit int) ([]moderation.AuditEntry, error) {
	switch {
	case q.Action != "" && !q.Action.Valid():
		return nil, fmt.Errorf("%w: action %q", ErrInvalidQuery, q.Action)
	case q.TargetKind != "" && !q.TargetKind.Valid():
		return nil, fmt.Errorf("%w: target kind %q", ErrInvalidQuery, q.TargetKind)
	case q.TargetID != "" && q.TargetKind == "":
		return nil, fmt.Errorf("%w: a target ID needs its target kind", ErrInvalidQuery)
	}
	return s.d.Repository.ListAuditEntries(ctx, q, limit)
}

// emitAudit emits the audit log entry of one of this module's operations, performed by byUserID
// ("" for the CLI).
func (s *Service) emitAudit(ctx context.Context, tx uow.Tx, e moderation.AuditEntry, byUserID string) error {
	e.ActorKind, e.ActorUserID = moderation.AuditActor(byUserID)
	return tx.Emit(ctx, AuditRecorded{Entry: e})
}

// accountLabel returns the display name of an account by ID for the audit log: its handle
// (@alice for a local account, @bob@example.com for a remote one), its URI when it has none, or
// its ID when it is gone.
func (s *Service) accountLabel(ctx context.Context, id string) (string, error) {
	a, err := s.d.Accounts.FindByID(ctx, id)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		return id, nil
	case err != nil:
		return "", err
	}
	return actorLabel(a), nil
}

// actorLabel returns the display name of an account for the audit log, as accountLabel does.
func actorLabel(a appaccount.Actor) string {
	switch {
	case a.Local != nil:
		return "@" + a.Local.Username
	case a.Handle != nil:
		return "@" + a.Handle.Username + "@" + a.Handle.Domain
	}
	return a.Account.URI
}
