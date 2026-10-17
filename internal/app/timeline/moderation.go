// SPDX-License-Identifier: AGPL-3.0-only

package timeline

import (
	"context"
	"time"

	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/timeline"
)

// feedRestrictions are the moderation restrictions that the timelines apply (data-model §4.6).
var feedRestrictions = []timeline.Restriction{
	timeline.RestrictProfileView, timeline.RestrictVisibilityCeilingPrivate, timeline.RestrictPublicDiscovery,
}

// RegisterModerationProjectors subscribes feed_author_restrictions and feed_domain_restrictions to
// the moderation module's events: each event carries the restrictions in effect on one account
// or domain, and the projectors replace the timeline restrictions of that account or domain
// with those among them that bear on timelines, each with the latest end of the actions that
// impose it.
func RegisterModerationProjectors(r *uow.Projectors, repo Repository) {
	r.Register(appmoderation.EventAccountRestrictionsChanged, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appmoderation.AccountRestrictionsChanged)
		return replaceRestrictions(ev.Effects, func(r timeline.Restriction, until *time.Time) error {
			return repo.UpsertAuthorRestriction(ctx, tx, AuthorRestriction{AccountID: ev.AccountID, Restriction: r, EffectiveUntil: until})
		}, func(r timeline.Restriction) error {
			return repo.DeleteAuthorRestriction(ctx, tx, ev.AccountID, r)
		})
	})
	r.Register(appmoderation.EventDomainRestrictionsChanged, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev := e.(appmoderation.DomainRestrictionsChanged)
		return replaceRestrictions(ev.Effects, func(r timeline.Restriction, until *time.Time) error {
			return repo.UpsertDomainRestriction(ctx, tx, DomainRestriction{Domain: ev.Domain, Restriction: r, EffectiveUntil: until})
		}, func(r timeline.Restriction) error {
			return repo.DeleteDomainRestriction(ctx, tx, ev.Domain, r)
		})
	})
}

// replaceRestrictions upserts the timeline restrictions among effects and deletes the others.
func replaceRestrictions(effects []appmoderation.Effect, upsert func(timeline.Restriction, *time.Time) error, remove func(timeline.Restriction) error) error {
	for _, r := range feedRestrictions {
		var (
			found bool
			until *time.Time
		)
		for _, e := range effects {
			if string(e.Restriction) == string(r) {
				found, until = true, e.Until
			}
		}
		var err error
		if found {
			err = upsert(r, until)
		} else {
			err = remove(r)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
