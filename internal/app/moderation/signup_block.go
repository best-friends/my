// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

// The sign-up block list (FR-719, data-model §4.3). Moderators add and lift blocks through the
// admin API and the CLI, or add the block of a banned account's email address with the ban; the
// registration page screens every sign-up against the blocks in effect (ScreenSignup).

// SignupBlockSeverity is moderation.SignupBlockSeverity, what a block does to a sign-up.
type SignupBlockSeverity = moderation.SignupBlockSeverity

// The severities that the identity module acts on.
const (
	SignupReject          = moderation.SignupReject
	SignupRequireApproval = moderation.SignupRequireApproval
)

// SignupBlockRepository is the persistence port of the sign-up block list, part of Repository.
type SignupBlockRepository interface {
	InsertSignupBlock(ctx context.Context, tx uow.Tx, b moderation.SignupBlock) error
	// SignupBlock returns a block by ID inside tx.
	SignupBlock(ctx context.Context, tx uow.Tx, id string) (moderation.SignupBlock, error)
	// InsertSignupBlockLift fails with ErrAlreadyRecorded when the block is already lifted.
	InsertSignupBlockLift(ctx context.Context, tx uow.Tx, l moderation.SignupBlockLift) error
	// FindSignupBlockRecord returns a block with its lift.
	FindSignupBlockRecord(ctx context.Context, id string) (moderation.SignupBlockRecord, error)
	// ListSignupBlocks returns up to limit blocks with their lifts, newest first: those of
	// q.Kind (every kind when empty), only those in effect at q.InEffectAt when it is set, whose
	// ID is below q.Before (from the newest when empty).
	ListSignupBlocks(ctx context.Context, q SignupBlockQuery, limit int) ([]moderation.SignupBlockRecord, error)
	// SignupBlocksInEffect returns the blocks in effect at now (neither lifted nor expired)
	// that may match a sign-up: the email_domain blocks of one of q.Domains, the email_hash
	// blocks of q.EmailHash, and every ip_range block.
	SignupBlocksInEffect(ctx context.Context, q SignupBlockMatch, now time.Time) ([]moderation.SignupBlock, error)
}

// SignupBlockQuery selects the blocks of a list.
type SignupBlockQuery struct {
	Kind       moderation.SignupBlockKind // "" for every kind
	InEffectAt *time.Time                 // nil for every block, ended ones included
	Before     string                     // a block ID; "" for the first page
}

// SignupBlockMatch is what SignupBlocksInEffect matches.
type SignupBlockMatch struct {
	// Domains is the domain of the email address and its parents (moderation.SignupDomains).
	Domains []string
	// EmailHash is the moderation.EmailHash of the address; "" matches no email_hash block.
	EmailHash string
}

var (
	// ErrInvalidSignupBlock is moderation.ErrInvalidSignupBlock: a block with an invalid kind,
	// severity, value, comment, or duration.
	ErrInvalidSignupBlock = moderation.ErrInvalidSignupBlock
	// ErrCannotBlockEmail reports an action that asks to block the email address of its target
	// (AccountActionInput.BlockEmail) when it cannot: it is not a ban of a local account, or the
	// account's user has no email address.
	ErrCannotBlockEmail = errors.New("moderation: the action cannot block the account's email address")
)

// Emails reads the primary email address of a user, for the block that a ban makes of it.
// *identity.Service implements it.
type Emails interface {
	// PrimaryEmailAddress returns the user's primary email address, "" when the user has none.
	PrimaryEmailAddress(ctx context.Context, userID string) (string, error)
}

// SignupBlockInput is a moderator's new sign-up block.
type SignupBlockInput struct {
	Kind moderation.SignupBlockKind
	// Value is a domain (U-labels or A-labels; a leading "@" is ignored) for email_domain, an
	// email address or the hex moderation.EmailHash of one for email_hash, and a CIDR prefix or
	// a single IP address for ip_range. It is stored in its canonical form.
	Value    string
	Severity moderation.SignupBlockSeverity
	Comment  string
	// Duration is how long the block lasts; zero lasts until it is lifted.
	Duration time.Duration
	// CreatedByUserID is the moderator; "" for the CLI.
	CreatedByUserID string
}

// AddSignupBlock records a new sign-up block, in effect at once, and its audit log entry. It
// fails with an error wrapping ErrInvalidSignupBlock for an invalid field.
func (s *Service) AddSignupBlock(ctx context.Context, in SignupBlockInput) (moderation.SignupBlock, error) {
	if in.Duration < 0 {
		return moderation.SignupBlock{}, fmt.Errorf("%w: a negative duration", ErrInvalidSignupBlock)
	}
	value, err := s.canonicalBlockValue(in.Kind, in.Value)
	if err != nil {
		return moderation.SignupBlock{}, err
	}
	now := kernel.Normalize(s.d.Clock.Now())
	b := moderation.SignupBlock{
		ID: s.d.IDs.NewID(), Kind: in.Kind, Value: value, Severity: in.Severity, Comment: strings.TrimSpace(in.Comment),
		CreatedByUserID: in.CreatedByUserID, ExpiresAt: expiry(now, in.Duration), CreatedAt: now,
	}
	if err := b.Validate(); err != nil {
		return moderation.SignupBlock{}, err
	}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return s.insertSignupBlock(ctx, tx, b, auditID)
	})
	if err != nil {
		return moderation.SignupBlock{}, err
	}
	return b, nil
}

// insertSignupBlock writes a block and its audit log entry inside tx.
func (s *Service) insertSignupBlock(ctx context.Context, tx uow.Tx, b moderation.SignupBlock, auditID string) error {
	if err := s.d.Repository.InsertSignupBlock(ctx, tx, b); err != nil {
		return err
	}
	return s.emitAudit(ctx, tx, moderation.AuditEntry{
		ID: auditID, Action: moderation.AuditSignupBlockAdded, TargetKind: moderation.AuditTargetSignupBlock,
		TargetID: b.ID, TargetLabel: moderation.SignupBlockLabel(b), SourceEventID: b.ID, OccurredAt: b.CreatedAt,
	}, b.CreatedByUserID)
}

// LiftSignupBlock lifts a sign-up block: it stops matching sign-ups at once, and the lift is
// audited. Lifting a block that is already lifted changes nothing. An unknown block fails with
// ErrNotFound.
func (s *Service) LiftSignupBlock(ctx context.Context, blockID, liftedByUserID string) error {
	now := kernel.Normalize(s.d.Clock.Now())
	repo := s.d.Repository
	auditID := s.d.IDs.NewID()
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		b, err := repo.SignupBlock(ctx, tx, blockID)
		if err != nil {
			return err
		}
		err = repo.InsertSignupBlockLift(ctx, tx, moderation.SignupBlockLift{BlockID: b.ID, LiftedByUserID: liftedByUserID, LiftedAt: now})
		if errors.Is(err, ErrAlreadyRecorded) {
			return nil
		}
		if err != nil {
			return err
		}
		// The lift's row is keyed by the block's ID.
		return s.emitAudit(ctx, tx, moderation.AuditEntry{
			ID: auditID, Action: moderation.AuditSignupBlockLifted, TargetKind: moderation.AuditTargetSignupBlock,
			TargetID: b.ID, TargetLabel: moderation.SignupBlockLabel(b), SourceEventID: b.ID, OccurredAt: now,
		}, liftedByUserID)
	})
}

// SignupBlockView is a sign-up block with its lift and whether it is in effect now.
type SignupBlockView struct {
	moderation.SignupBlockRecord
	State moderation.ActionState
}

func (s *Service) signupBlockView(r moderation.SignupBlockRecord) SignupBlockView {
	return SignupBlockView{SignupBlockRecord: r, State: r.State(s.d.Clock.Now())}
}

// SignupBlock returns a sign-up block with its lift; it fails with ErrNotFound for an unknown ID.
func (s *Service) SignupBlock(ctx context.Context, id string) (SignupBlockView, error) {
	r, err := s.d.Repository.FindSignupBlockRecord(ctx, id)
	if err != nil {
		return SignupBlockView{}, err
	}
	return s.signupBlockView(r), nil
}

// SignupBlockFilter selects the sign-up blocks that SignupBlocks lists.
type SignupBlockFilter struct {
	// Kind keeps the blocks of one kind; "" keeps every kind.
	Kind moderation.SignupBlockKind
	// Active keeps the blocks in effect now: neither lifted nor expired.
	Active bool
	// Before is the ID of the last block of the previous page; "" starts from the newest.
	Before string
}

// SignupBlocks lists up to limit sign-up blocks, newest first. It fails with ErrInvalidQuery for
// an unknown kind.
func (s *Service) SignupBlocks(ctx context.Context, f SignupBlockFilter, limit int) ([]SignupBlockView, error) {
	if f.Kind != "" && !f.Kind.Valid() {
		return nil, fmt.Errorf("%w: kind %q", ErrInvalidQuery, f.Kind)
	}
	q := SignupBlockQuery{Kind: f.Kind, Before: f.Before}
	if f.Active {
		now := kernel.Normalize(s.d.Clock.Now())
		q.InEffectAt = &now
	}
	rs, err := s.d.Repository.ListSignupBlocks(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SignupBlockView, len(rs))
	for i, r := range rs {
		out[i] = s.signupBlockView(r)
	}
	return out, nil
}

// ScreenSignup returns the strongest severity of the blocks in effect that match a sign-up from
// the registration page with the email address email from the client address ip ("" or an
// unparsable address matches no ip_range block): SignupReject, SignupRequireApproval, or ""
// when none matches.
func (s *Service) ScreenSignup(ctx context.Context, email, ip string) (moderation.SignupBlockSeverity, error) {
	domain, hash := s.emailKeys(email)
	addr, _ := netip.ParseAddr(strings.TrimSpace(ip))
	now := kernel.Normalize(s.d.Clock.Now())
	blocks, err := s.d.Repository.SignupBlocksInEffect(ctx, SignupBlockMatch{Domains: moderation.SignupDomains(domain), EmailHash: hash}, now)
	if err != nil {
		return "", err
	}
	var severity moderation.SignupBlockSeverity
	for _, b := range blocks {
		if b.Matches(domain, hash, addr) {
			severity = severity.Stronger(b.Severity)
		}
	}
	return severity, nil
}

// emailKeys returns the canonical domain of an email address and its moderation.EmailHash, or ""
// for both when the address has no domain.
func (s *Service) emailKeys(email string) (domain, hash string) {
	at := strings.LastIndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return "", ""
	}
	domain, err := s.canonicalDomain(email[at+1:])
	if err != nil {
		domain = strings.ToLower(email[at+1:])
	}
	return domain, moderation.EmailHash(email[:at] + "@" + domain)
}

// canonicalDomain returns a domain in IDNA A-labels through Deps.CanonicalDomain, or in lower
// case without it.
func (s *Service) canonicalDomain(d string) (string, error) {
	d = strings.TrimSuffix(strings.TrimSpace(d), ".")
	if s.d.CanonicalDomain == nil {
		return strings.ToLower(d), nil
	}
	return s.d.CanonicalDomain(d)
}

// canonicalBlockValue returns the value of a new block of kind k in its canonical form.
func (s *Service) canonicalBlockValue(k moderation.SignupBlockKind, raw string) (string, error) {
	v := strings.TrimSpace(raw)
	switch k {
	case moderation.SignupBlockEmailDomain:
		v = strings.TrimPrefix(v, "@")
		if _, err := netip.ParseAddr(v); err == nil || v == "" {
			return "", fmt.Errorf("%w: %q is not a domain", ErrInvalidSignupBlock, raw)
		}
		d, err := s.canonicalDomain(v)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrInvalidSignupBlock, err)
		}
		return d, nil
	case moderation.SignupBlockEmailHash:
		if strings.Contains(v, "@") {
			_, hash := s.emailKeys(v)
			if hash == "" {
				return "", fmt.Errorf("%w: %q is not an email address", ErrInvalidSignupBlock, raw)
			}
			return hash, nil
		}
		return strings.ToLower(v), nil
	case moderation.SignupBlockIPRange:
		if a, err := netip.ParseAddr(v); err == nil {
			a = a.Unmap()
			return netip.PrefixFrom(a, a.BitLen()).String(), nil
		}
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return "", fmt.Errorf("%w: %q is not a CIDR prefix", ErrInvalidSignupBlock, raw)
		}
		if p.Addr().Is4In6() {
			if p.Bits() < 96 {
				return "", fmt.Errorf("%w: %q is not a CIDR prefix", ErrInvalidSignupBlock, raw)
			}
			p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
		}
		return p.Masked().String(), nil
	}
	return "", fmt.Errorf("%w: kind %q", ErrInvalidSignupBlock, k)
}

// banEmailBlock returns the email_hash block, rejecting sign-ups, that the ban a makes of the
// primary email address of userID, the user of the banned account; it fails with
// ErrCannotBlockEmail when the user has no email address.
func (s *Service) banEmailBlock(ctx context.Context, userID string, a moderation.Action) (moderation.SignupBlock, error) {
	if s.d.Emails == nil {
		return moderation.SignupBlock{}, fmt.Errorf("%w: no email addresses to read", ErrCannotBlockEmail)
	}
	email, err := s.d.Emails.PrimaryEmailAddress(ctx, userID)
	if err != nil {
		return moderation.SignupBlock{}, err
	}
	_, hash := s.emailKeys(email)
	if hash == "" {
		return moderation.SignupBlock{}, fmt.Errorf("%w: the account's user has no email address", ErrCannotBlockEmail)
	}
	b := moderation.SignupBlock{
		ID: s.d.IDs.NewID(), Kind: moderation.SignupBlockEmailHash, Value: hash, Severity: moderation.SignupReject,
		ModerationActionID: a.ID, CreatedByUserID: a.IssuedByUserID, CreatedAt: a.IssuedAt,
	}
	return b, b.Validate()
}
