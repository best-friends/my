// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/graph"
)

// MuteDomain makes accountID mute domain and its subdomains (FR-304): their posts leave the
// account's timelines other than home (and their boosts leave home), and their notifications
// are hidden. The accounts of the domain that the account follows stay on its home timeline.
// domain is a canonical host (appfederation.CanonicalHost); this instance's own domain fails
// with graph.ErrInvalidDomain, and a mute beyond graph.MaxDomainMutes with
// graph.ErrTooManyDomainMutes. Muting a domain that the account already mutes changes nothing
// and emits nothing; otherwise it emits DomainMuted. Nothing federates. The account must be
// local and active.
func (s *Service) MuteDomain(ctx context.Context, accountID, domain string) (graph.DomainMute, error) {
	m := graph.DomainMute{AccountID: accountID, Domain: domain, CreatedAt: s.d.Clock.Now()}
	if err := m.Validate(); err != nil {
		return graph.DomainMute{}, err
	}
	if domain == s.d.Host {
		return graph.DomainMute{}, fmt.Errorf("%w: %q is this instance's domain", graph.ErrInvalidDomain, domain)
	}
	if _, err := s.localActive(ctx, accountID); err != nil {
		return graph.DomainMute{}, err
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		n, err := s.d.Repository.CountDomainMutes(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if n >= graph.MaxDomainMutes {
			return graph.ErrTooManyDomainMutes
		}
		inserted, err := s.d.Repository.InsertDomainMute(ctx, tx, m)
		if err != nil || !inserted {
			return err
		}
		return tx.Emit(ctx, DomainMuted{Mute: m})
	})
	if err != nil {
		return graph.DomainMute{}, err
	}
	return m, nil
}

// MutesDomain reports whether accountID mutes domain, the domain of a remote account, or one of
// its parent domains (FR-304). The local domain "" is never muted.
func (s *Service) MutesDomain(ctx context.Context, accountID, domain string) (bool, error) {
	return s.d.Repository.MutesAnyDomain(ctx, accountID, graph.MutedDomains(domain))
}

// UnmuteDomain removes accountID's mute of domain and emits DomainUnmuted. Without a mute it
// changes nothing and emits nothing.
func (s *Service) UnmuteDomain(ctx context.Context, accountID, domain string) error {
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		deleted, err := s.d.Repository.DeleteDomainMute(ctx, tx, accountID, domain)
		if err != nil || !deleted {
			return err
		}
		return tx.Emit(ctx, DomainUnmuted{AccountID: accountID, Domain: domain})
	})
}

// DomainMutePage selects a page of a list of domain mutes ordered by domain.
type DomainMutePage struct {
	// After is the domain of the last mute of the previous page; "" starts at the first.
	After string
	// Limit is the page size, as in Page.
	Limit int
}

// DomainMutes returns accountID's domain mutes in the order of their domains.
func (s *Service) DomainMutes(ctx context.Context, accountID string, p DomainMutePage) ([]graph.DomainMute, error) {
	return s.d.Repository.ListDomainMutes(ctx, accountID, p.After, Page{Limit: p.Limit}.limit())
}
