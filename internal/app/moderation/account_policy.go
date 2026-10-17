// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"errors"
	"slices"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/domain/moderation"
)

// AccountDomains finds the domain of an account. *appaccount.Service implements it.
type AccountDomains interface {
	AccountDomain(ctx context.Context, id string) (string, error)
}

// AccountPolicy is the RestrictionPolicy of data-model §5.3 as far as moderation decides it: the
// restrictions in effect on an account are those of its own actions (account_restrictions) and
// those of its domain and the parent domains (domain_restrictions), read at the time of each
// call. The other modules decide through it, never from the moderation tables: the content
// module through Content, the identity module through LoginRestricted, and the graph module
// through InteractionRestricted.
type AccountPolicy struct {
	repo     Repository
	accounts AccountDomains
	clock    kernel.Clock
}

// NewAccountPolicy returns the AccountPolicy on repo.
func NewAccountPolicy(repo Repository, accounts AccountDomains, clock kernel.Clock) *AccountPolicy {
	return &AccountPolicy{repo: repo, accounts: accounts, clock: clock}
}

// Restrictions returns the restrictions in effect on an account, in restriction order. An
// unknown account has those of its own actions only.
func (p *AccountPolicy) Restrictions(ctx context.Context, accountID string) ([]moderation.Restriction, error) {
	now := p.clock.Now()
	rs, err := p.repo.AccountRestrictionsInEffect(ctx, accountID, now)
	if err != nil {
		return nil, err
	}
	domain, err := p.accounts.AccountDomain(ctx, accountID)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		return rs, nil
	case err != nil:
		return nil, err
	}
	domains := moderation.DomainAndParents(domain)
	if len(domains) == 0 {
		return rs, nil
	}
	ds, err := p.repo.DomainRestrictionsInEffect(ctx, domains, now)
	if err != nil {
		return nil, err
	}
	rs = append(rs, ds...)
	slices.Sort(rs)
	return slices.Compact(rs), nil
}

func (p *AccountPolicy) has(ctx context.Context, accountID string, r moderation.Restriction) (bool, error) {
	rs, err := p.Restrictions(ctx, accountID)
	return slices.Contains(rs, r), err
}

// LoginRestricted reports whether an account may not log in or use the API (the login
// restriction: disable_login, suspend).
func (p *AccountPolicy) LoginRestricted(ctx context.Context, accountID string) (bool, error) {
	return p.has(ctx, accountID, moderation.RestrictLogin)
}

// InteractionRestricted reports whether an account may not follow, favourite, boost, or reply
// (the interact restriction: restrict_interaction, suspend).
func (p *AccountPolicy) InteractionRestricted(ctx context.Context, accountID string) (bool, error) {
	return p.has(ctx, accountID, moderation.RestrictInteract)
}

// ProfileHidden reports whether an account's profile is shown to nobody but itself and the
// moderators (the profile_view restriction: suspend, or a suspended domain).
func (p *AccountPolicy) ProfileHidden(ctx context.Context, accountID string) (bool, error) {
	return p.has(ctx, accountID, moderation.RestrictProfileView)
}

// Content returns the policy as the content module's RestrictionPolicy.
func (p *AccountPolicy) Content() appcontent.RestrictionPolicy { return contentPolicy{p} }

type contentPolicy struct{ p *AccountPolicy }

// Restrictions implements appcontent.RestrictionPolicy.
func (c contentPolicy) Restrictions(ctx context.Context, accountID string) (appcontent.Restrictions, error) {
	rs, err := c.p.Restrictions(ctx, accountID)
	if err != nil {
		return appcontent.Restrictions{}, err
	}
	return appcontent.Restrictions{
		Post:           slices.Contains(rs, moderation.RestrictPost),
		Interact:       slices.Contains(rs, moderation.RestrictInteract),
		MediaSensitive: slices.Contains(rs, moderation.RestrictMediaSensitive),
		Author: appcontent.AuthorRestrictions{
			ProfileView:              slices.Contains(rs, moderation.RestrictProfileView),
			VisibilityCeilingPrivate: slices.Contains(rs, moderation.RestrictVisibilityCeilingPrivate),
			PublicDiscovery:          slices.Contains(rs, moderation.RestrictPublicDiscovery),
		},
	}, nil
}
