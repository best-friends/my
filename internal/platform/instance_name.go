// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"log/slog"

	"github.com/best-friends/my/internal/domain/instance"
)

// instanceName returns the display name of the instance: the name text, the same in every
// locale, or "" when there is none. Everything that names the instance to people (the web shell,
// the sign-in pages, emails, link previews, NodeInfo) then names it by its host; handles and
// URLs always use the host and the account domain. The text is read per call, so that a change
// applies at once. A failure to read it is logged and names the instance by its host.
func (a *App) instanceName(ctx context.Context) string {
	t, ok, err := a.Instance.Text(ctx, instance.TextName)
	if err != nil {
		a.Logger.WarnContext(ctx, "read the instance name", slog.Any("error", err))
		return ""
	}
	if !ok {
		return ""
	}
	return t.Body
}

// instanceSummary returns the instance's one-paragraph summary (the short_description text), the
// same in every locale, or "" when there is none.
// The link preview of the landing page shows it. Like instanceName, it is read per call, and a
// failure to read it is logged and leaves the summary out.
func (a *App) instanceSummary(ctx context.Context) string {
	t, ok, err := a.Instance.Text(ctx, instance.TextShortDescription)
	if err != nil {
		a.Logger.WarnContext(ctx, "read the instance summary", slog.Any("error", err))
		return ""
	}
	if !ok {
		return ""
	}
	return t.Body
}
