// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/domain/moderation"
)

// PublicDomainActions lists up to limit of the domain actions that the instance may list
// publicly (FR-718): those of moderation.PublicDomainActionKinds in effect now, newest first,
// whose ID is below before ("" starts from the newest). They carry no private note, and the
// domains of those that ask for it are obfuscated. Whether a reader may see the list and its
// comments is the instance settings' decision (domain_actions_visibility and
// domain_actions_rationale_visibility), which the caller checks.
func (s *Service) PublicDomainActions(ctx context.Context, before string, limit int) ([]moderation.PublicDomainAction, error) {
	now := kernel.Normalize(s.d.Clock.Now())
	rs, err := s.d.Repository.ListDomainActions(ctx, DomainActionQuery{
		Kinds: moderation.PublicDomainActionKinds(), InEffectAt: &now, Before: before,
	}, limit)
	if err != nil {
		return nil, err
	}
	out := make([]moderation.PublicDomainAction, len(rs))
	for i, r := range rs {
		out[i] = r.Action.Public()
	}
	return out, nil
}
