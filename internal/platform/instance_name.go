// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"log/slog"

	"github.com/best-friends/my/internal/domain/instance"
)

// instanceName returns the display name of the instance for a reader of locale ("" for the
// instance's default locale): the name text along the locale fallback chain, or "" when no
// locale of the chain has one. Everything that names the instance to people (the web shell,
// the sign-in pages, emails, link previews, NodeInfo) then names it by its host; handles and
// URLs always use the host and the account domain. The text is read per call, so that a change
// applies at once. A failure to read it is logged and names the instance by its host.
func (a *App) instanceName(ctx context.Context, locale string) string {
	t, ok, err := a.Instance.Text(ctx, instance.TextName, locale)
	if err != nil {
		a.Logger.WarnContext(ctx, "read the instance name", slog.Any("error", err))
		return ""
	}
	if !ok {
		return ""
	}
	return t.Body
}
