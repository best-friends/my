// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/projection"
	"github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/app/uow"
)

// projectionRepositories are the repositories whose modules' rebuild functions the projections
// run.
type projectionRepositories struct {
	accounts      account.Repository
	instances     instance.Repository
	follows       graph.Repository
	statuses      content.Repository
	interactions  content.InteractionRepository
	feeds         timeline.Repository
	notifications notification.Repository
	moderation    moderation.Repository
	identities    identity.Repository
}

// setUpProjections builds the service of `my projection` and registers the projection.verify
// job (data-model §2.3, §9). Each projection runs its module's rebuild function; the order is
// the order of `my projection list` and of the job.
func (a *App) setUpProjections(store projection.Store, r projectionRepositories) {
	a.Projections = projection.NewService(projection.Options{
		UnitOfWork: a.UnitOfWork, Store: store, Logger: a.Logger,
		Projections: []projection.Projection{
			{
				Name: "instance", Module: "instance",
				Tables: []string{"instance_settings", "instance_texts", "instance_rule_heads", "instance_theme"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return instance.RebuildProjections(ctx, u, r.instances)
				},
			},
			{
				Name: "account", Module: "account",
				Tables: []string{"account_handles", "account_profile_heads", "account_lifecycle_states", "remote_account_heads"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return account.RebuildProjections(ctx, u, r.accounts)
				},
			},
			{
				Name: "identity", Module: "identity",
				Tables: []string{"user_roles", "user_registration_states", "user_primary_emails"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return identity.RebuildProjections(ctx, u, r.identities)
				},
			},
			{
				Name: "moderation", Module: "moderation",
				Tables: []string{"account_restrictions", "domain_restrictions", "report_states", "appeal_states"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return moderation.RebuildProjections(ctx, u, r.moderation, a.Clock)
				},
			},
			{
				Name: "graph", Module: "graph",
				Tables: []string{"follow_edges", "follow_counters", "block_edges"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return graph.RebuildProjections(ctx, u, r.follows)
				},
			},
			{
				Name: "content", Module: "content",
				Tables: []string{"status_heads", "status_mention_index", "status_reply_links", "status_counters", "account_post_counters", "local_post_counters", "poll_tallies"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return content.RebuildProjections(ctx, u, r.statuses)
				},
			},
			{
				Name: "status_sensitivity_overrides", Module: "content",
				Tables: []string{"status_sensitivity_overrides"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return content.RebuildSensitivityOverrides(ctx, u, r.statuses,
						func(ctx context.Context, tx uow.Tx, mark func(statusID, actionID string) error) error {
							return moderation.ReplaySensitivityMarks(ctx, tx, r.moderation, mark)
						})
				},
			},
			{
				Name: "boost_edges", Module: "content",
				Tables: []string{"boost_edges"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return content.RebuildInteractionProjections(ctx, u, r.interactions)
				},
			},
			{
				Name: "feeds", Module: "timeline",
				Tables: []string{"feed_entries", "feed_direct_recipients", "feed_tag_entries", "feed_follow_edges", "feed_exclusions"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return timeline.RebuildProjections(ctx, u, r.feeds,
						timeline.ContentSource(r.statuses, r.interactions), timeline.GraphSource(r.follows))
				},
			},
			{
				Name: "feed_restrictions", Module: "timeline",
				Tables: []string{"feed_author_restrictions", "feed_domain_restrictions"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return timeline.RebuildRestrictions(ctx, u, r.feeds, a.Accounts, a.Clock,
						timeline.ModerationSource(r.moderation, a.Clock), timeline.AccountSource(r.accounts))
				},
			},
			{
				Name: "notification_exclusions", Module: "notification",
				Tables: []string{"notification_exclusions"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return notification.RebuildExclusions(ctx, u, r.notifications, notification.GraphSource(r.follows))
				},
			},
			{
				Name: "notification_deleted_statuses", Module: "notification",
				Tables: []string{"notification_deleted_statuses"},
				Rebuild: func(ctx context.Context, u uow.UnitOfWork) error {
					return notification.RebuildDeletedStatuses(ctx, u, r.notifications, a.Content)
				},
			},
		},
	})
	a.Projections.RegisterJobs(a.Handlers, a.Scheduler)
}
