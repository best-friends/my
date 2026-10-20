// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"fmt"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/timeline"
)

// setUpErasure registers the erasure of deleted local accounts' data and its schedule
// (data-model §8, FR-008). Every database adapter's repositories implement the modules'
// erasure repositories. The erasers run in order: the statuses first, whose notifications the
// notification module erases with them, then the account's interactions, follows, timeline and
// notification rows, its uploads with their blobs, and its user's identity data; the account
// module erases the profile and keys last. A retention hold stops it, and the statuses that the
// moderation records need stay with their media files (ADR-0038). It comes after the content,
// graph, timeline, notification, and media services, the blob store, and the data exports and imports.
func (a *App) setUpErasure(ids identity.Repository, accounts account.Repository, statuses content.Repository,
	follows graph.Repository, feeds timeline.Repository, notifs notification.Repository, medias content.MediaRepository,
) error {
	identities, okI := ids.(identity.ErasureRepository)
	profiles, okA := accounts.(account.ErasureRepository)
	posts, okC := statuses.(content.ErasureRepository)
	edges, okG := follows.(graph.ErasureRepository)
	entries, okT := feeds.(timeline.ErasureRepository)
	notes, okN := notifs.(notification.ErasureRepository)
	if !okI || !okA || !okC || !okG || !okT || !okN {
		return fmt.Errorf("platform: the database adapter does not implement the erasure of deleted accounts")
	}
	notifications := notification.NewEraser(a.UnitOfWork, notes)
	account.NewEraser(account.ErasureDeps{
		UnitOfWork: a.UnitOfWork, Repository: profiles, Holds: a.Moderation, Jobs: a.Jobs, Clock: a.Clock,
		Erasers: []account.DataEraser{
			content.NewStatusEraser(content.ErasureDeps{
				UnitOfWork: a.UnitOfWork, Repository: statuses, Erasure: posts, Notifications: notifications,
				Evidence: a.Moderation, Clock: a.Clock,
			}),
			content.NewInteractionEraser(a.UnitOfWork, posts),
			graph.NewEraser(a.UnitOfWork, follows, edges, a.Clock),
			timeline.NewEraser(a.UnitOfWork, entries),
			notifications,
			// The instance settings' images stay; the profile's go with the account.
			content.NewMediaEraser(content.MediaErasureDeps{
				UnitOfWork: a.UnitOfWork, Repository: medias, Blobs: a.Blobs, Referrers: []content.MediaReferrer{a.Instance},
			}),
			// The user's data exports with their archives (FR-008).
			a.Exports,
			// The user's CSV imports (FR-008).
			a.Imports,
			identity.NewEraser(a.UnitOfWork, identities),
		},
	}).RegisterJobs(a.Handlers, a.Scheduler)
	return nil
}
