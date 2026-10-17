// SPDX-License-Identifier: AGPL-3.0-only

package authpages

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/moderation"
	"github.com/best-friends/my/internal/i18n"
)

// A user whom moderation keeps out of the app (disable_login, suspend) still signs in to these
// screens: the status page lists the actions in effect that keep them out, so that they can see
// why without an access token (FR-715, NFR-MOD-6).

// Moderation lists the actions on a user's own account as the user sees them, with no private
// note, report, or issuer. *appmoderation.Service implements it.
type Moderation interface {
	OwnActions(ctx context.Context, accountID string, f appmoderation.OwnActionFilter, limit int) ([]appmoderation.TargetActionView, error)
}

// UserLocales reads the UI locale that a user saved. *appidentity.Service implements it.
type UserLocales interface {
	UserLocale(ctx context.Context, userID string) (string, error)
}

// maxStatusActions bounds the actions that the status page lists.
const maxStatusActions = 40

// statusAction is one action in effect on the status page.
type statusAction struct {
	Kind, Category string
	// Reason is the moderators' explanation; empty when there is none.
	Reason string
	// Ends says when the action ends; EndsAt is that time in RFC 3339, empty for an action that
	// lasts until it is lifted.
	Ends, EndsAt string
}

// userLocalizer returns the localizer of a page for a signed-in user: their saved locale, then
// the request's (ADR-0009 §2).
func (h *Handler) userLocalizer(r *http.Request, userID string) *i18n.Localizer {
	var saved string
	if h.o.UserLocales != nil {
		var err error
		if saved, err = h.o.UserLocales.UserLocale(r.Context(), userID); err != nil {
			h.o.Logger.WarnContext(r.Context(), "read the user's locale", slog.String("user", userID), slog.Any("error", err))
		}
	}
	return h.o.Locales.UserLocalizer(r, saved)
}

// restrictionActions returns the actions in effect on the account of access that keep it out
// of the app, for the status page of a user whom moderation keeps out; none when moderation
// does not. The other actions in effect are in the app, once the user can use it again.
func (h *Handler) restrictionActions(ctx context.Context, l *i18n.Localizer, access appidentity.Access) ([]statusAction, error) {
	if !access.LoginRestricted || access.AccountID == "" || h.o.Moderation == nil {
		return nil, nil
	}
	vs, err := h.o.Moderation.OwnActions(ctx, access.AccountID, appmoderation.OwnActionFilter{Active: true}, maxStatusActions)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]statusAction, 0, len(vs))
	for _, v := range vs {
		if !slices.Contains(v.Kind.Restrictions(), moderation.RestrictLogin) {
			continue
		}
		a := statusAction{
			Kind:     l.ServerModerationActionKind(string(v.Kind)),
			Category: l.ServerAuthStatusActionReason(l.ServerModerationActionCategory(string(v.ReasonCategory))),
			Reason:   v.PublicReason,
			Ends:     l.ServerAuthStatusActionUntilLifted(),
		}
		if v.ExpiresAt != nil {
			days := max(1, int(math.Ceil(v.ExpiresAt.Sub(now).Hours()/24)))
			a.Ends, a.EndsAt = l.ServerAuthStatusActionEndsIn(days), v.ExpiresAt.UTC().Format(time.RFC3339)
		}
		out = append(out, a)
	}
	return out, nil
}
