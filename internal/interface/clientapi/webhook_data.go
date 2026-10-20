// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"

	appidentity "github.com/best-friends/my/internal/app/identity"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/moderation"
)

// The data of the webhook messages (ADR-0060): the admin operations' representation of the
// object of an event, presented by the same serializers as those operations. *Server implements
// webhook.Presenter with these methods; the webhook module calls them in the transaction of the
// event, with a context whose reads see what the transaction wrote.

// errNotConfigured reports a presenter method whose use cases the server was built without.
var errNotConfigured = errors.New("clientapi: the use cases of the webhook data are not configured")

// WebhookReport presents a report as AdminReport, as GET /admin/reports lists it.
func (s *Server) WebhookReport(ctx context.Context, id string) (any, error) {
	if s.o.Moderation == nil {
		return nil, errNotConfigured
	}
	rec, err := s.o.Moderation.ReportRecord(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("clientapi: report %s: %w", id, err)
	}
	items, err := s.newAdminReports(ctx, []moderation.ReportRecord{rec})
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

// WebhookRegistration presents the registration of a user as AdminRegistration.
func (s *Server) WebhookRegistration(ctx context.Context, userID string) (any, error) {
	if s.o.Registrations == nil {
		return nil, errNotConfigured
	}
	e, err := s.o.Registrations.Registration(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the registration of %s: %w", userID, err)
	}
	return newAdminRegistration(e), nil
}

// WebhookAccount presents an account as AdminAccount, as GET /admin/accounts lists it.
func (s *Server) WebhookAccount(ctx context.Context, id string) (any, error) {
	if s.o.AccountAdmin == nil {
		return nil, errNotConfigured
	}
	a, err := s.o.AccountAdmin.Account(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("clientapi: account %s: %w", id, err)
	}
	items, err := s.newAdminAccounts(ctx, []appidentity.AdminAccount{a})
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

// WebhookAccountAction presents an action on an account as AdminAccountAction.
func (s *Server) WebhookAccountAction(ctx context.Context, id string) (any, error) {
	if s.o.AccountModeration == nil {
		return nil, errNotConfigured
	}
	v, err := s.o.AccountModeration.AccountAction(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("clientapi: account action %s: %w", id, err)
	}
	items, err := s.newAdminAccountActions(ctx, []appmoderation.AccountActionView{v})
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

// WebhookAppeal presents an appeal as AdminAppeal, with its action.
func (s *Server) WebhookAppeal(ctx context.Context, id string) (any, error) {
	if s.o.AppealModeration == nil {
		return nil, errNotConfigured
	}
	v, err := s.o.AppealModeration.Appeal(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("clientapi: appeal %s: %w", id, err)
	}
	items, err := s.newAdminAppeals(ctx, []appmoderation.AppealView{v})
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

// WebhookDomainAction presents an action on a domain as AdminDomainAction.
func (s *Server) WebhookDomainAction(ctx context.Context, id string) (any, error) {
	if s.o.Moderation == nil {
		return nil, errNotConfigured
	}
	v, err := s.o.Moderation.DomainAction(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("clientapi: domain action %s: %w", id, err)
	}
	return newAdminDomainAction(v), nil
}

// WebhookCustomEmoji presents a custom emoji as AdminCustomEmoji.
func (s *Server) WebhookCustomEmoji(ctx context.Context, id string) (any, error) {
	if s.o.CustomEmojis == nil {
		return nil, errNotConfigured
	}
	e, err := s.o.CustomEmojis.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("clientapi: custom emoji %s: %w", id, err)
	}
	return s.newAdminCustomEmoji(e), nil
}
