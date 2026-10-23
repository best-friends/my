// SPDX-License-Identifier: AGPL-3.0-only

package platform

import "github.com/best-friends/my/internal/app/timeline"

// setUpTimeline registers the timeline module's projectors and builds its service. It comes
// after the content and graph projectors, which reject the commands that must not reach the
// timelines (data-model §4.6).
func (a *App) setUpTimeline(repo timeline.Repository) {
	timeline.RegisterProjectors(a.Projectors, repo)
	timeline.RegisterModerationProjectors(a.Projectors, repo, a.Accounts)
	a.Timeline = timeline.NewService(timeline.Deps{Repository: repo, Clock: a.Clock})
}
