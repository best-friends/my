// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	appaccount "github.com/best-friends/my/internal/app/account"
	appidentity "github.com/best-friends/my/internal/app/identity"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/moderation"
)

// The admin operations of the account management (FR-714, manage_accounts): the search, the
// details of an account, and showing the email address and the IP addresses of a local one,
// which only revealAdminAccount returns and the identity module records in the audit log.

// AccountAdmin runs the moderators' account management. *appidentity.AccountAdmin implements it.
type AccountAdmin interface {
	Search(ctx context.Context, q appidentity.AccountSearch, limit int) ([]appidentity.AdminAccount, string, error)
	Detail(ctx context.Context, accountID string) (appidentity.AdminAccountDetail, error)
	Reveal(ctx context.Context, accountID, byUserID string) (appidentity.PrivateData, error)
}

// AccountReports reads the reports of an account for its details. *appmoderation.Service
// implements it.
type AccountReports interface {
	RecentReportsOf(ctx context.Context, targetAccountID string, limit int) ([]moderation.ReportRecord, error)
}

// maxAdminAccountParam bounds the length of the text parameters of the search, in characters.
const maxAdminAccountParam = 320

// newAdminAccounts presents accounts as the moderators see them, reading their images once.
func (s *Server) newAdminAccounts(ctx context.Context, as []appidentity.AdminAccount) ([]AdminAccount, error) {
	actors := make([]appaccount.Actor, len(as))
	for i, a := range as {
		actors[i] = a.Actor
	}
	presented, err := s.newAccounts(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := make([]AdminAccount, len(as))
	for i, a := range as {
		item := AdminAccount{
			Id: a.Actor.Account.ID, Account: presented[i], Local: a.Actor.Account.IsLocal(), State: AdminAccountState(a.State),
			Restrictions: make([]AdminAccountRestriction, 0, len(a.Restrictions)),
		}
		for _, r := range a.Restrictions {
			item.Restrictions = append(item.Restrictions, AdminAccountRestriction(r))
		}
		if a.UserID != "" {
			st := AdminAccountRegistrationState(identity.RegistrationApproved)
			if a.Registration != nil {
				st = AdminAccountRegistrationState(a.Registration.State)
			}
			item.RegistrationState = &st
		}
		out[i] = item
	}
	return out, nil
}

// ListAdminAccounts serves GET /api/v1/admin/accounts, newest first. The cursor is the position
// of the search after the last account of the previous page.
func (s *Server) ListAdminAccounts(ctx context.Context, req ListAdminAccountsRequestObject) (ListAdminAccountsResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageAccounts); err != nil {
		return nil, err
	} else if p != nil {
		return ListAdminAccountsdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	pq := s.parsePage(ctx, req.Params.Cursor, req.Params.Limit)
	if pq.problem != nil {
		return ListAdminAccountsdefaultApplicationProblemPlusJSONResponse(*pq.problem), nil
	}
	p := req.Params
	q := appidentity.AccountSearch{After: pq.before}
	for _, f := range []struct {
		name  string
		value *string
		dst   *string
	}{
		{"username", p.Username, &q.Username}, {"domain", p.Domain, &q.Domain}, {"email", p.Email, &q.Email},
		{"email_domain", p.EmailDomain, &q.EmailDomain}, {"ip", p.Ip, &q.IP},
	} {
		if f.value == nil {
			continue
		}
		if utf8.RuneCountInString(*f.value) > maxAdminAccountParam {
			return ListAdminAccountsdefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, f.name)), nil
		}
		*f.dst = *f.value
	}
	if p.Origin != nil {
		q.Origin = string(*p.Origin)
	}
	if p.State != nil {
		q.State = appidentity.AdminAccountState(*p.State)
	}
	as, next, err := s.o.AccountAdmin.Search(ctx, q, pq.limit)
	if serr := (*appidentity.AccountSearchError)(nil); errors.As(err, &serr) {
		return ListAdminAccountsdefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, serr.Param)), nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: search accounts: %w", err)
	}
	items, err := s.newAdminAccounts(ctx, as)
	if err != nil {
		return nil, err
	}
	page := AdminAccountPage{Items: items}
	if next != "" {
		page.NextCursor = encodeCursor(next)
	}
	cache := cacheNoStore
	return ListAdminAccounts200JSONResponse{Body: page, Headers: ListAdminAccounts200ResponseHeaders{CacheControl: &cache}}, nil
}

// GetAdminAccount serves GET /api/v1/admin/accounts/{id}.
func (s *Server) GetAdminAccount(ctx context.Context, req GetAdminAccountRequestObject) (GetAdminAccountResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageAccounts); err != nil {
		return nil, err
	} else if p != nil {
		return GetAdminAccountdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	d, err := s.o.AccountAdmin.Detail(ctx, req.Id)
	if errors.Is(err, appaccount.ErrNotFound) {
		return GetAdminAccountdefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: account %s: %w", req.Id, err)
	}
	presented, err := s.newAdminAccounts(ctx, []appidentity.AdminAccount{d.AdminAccount})
	if err != nil {
		return nil, err
	}
	a := presented[0]
	out := AdminAccountDetail{
		Id: a.Id, Account: a.Account, Local: a.Local, State: a.State, Restrictions: a.Restrictions,
		RegistrationState: a.RegistrationState, Roles: make([]string, 0, len(d.Roles)),
	}
	for _, r := range d.Roles {
		out.Roles = append(out.Roles, r.Key)
	}
	if d.HasEmail {
		v := d.EmailVerified
		out.EmailVerified = &v
	}
	if app := d.Application; app != nil {
		reg := AdminAccountRegistration{SubmittedAt: app.SubmittedAt.UTC(), StateChangedAt: app.SubmittedAt.UTC()}
		if d.Registration != nil {
			reg.StateChangedAt = d.Registration.Since.UTC()
		}
		if app.Reason != "" {
			reg.Reason = &app.Reason
		}
		if app.InviteID != "" {
			reg.InviteId = &app.InviteID
		}
		out.Registration = &reg
	}
	actions, err := s.o.AccountModeration.AccountActions(ctx, appmoderation.AccountActionFilter{AccountID: req.Id}, appidentity.AccountDetailHistory)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the actions on account %s: %w", req.Id, err)
	}
	if out.Actions, err = s.newAdminAccountActions(ctx, actions); err != nil {
		return nil, err
	}
	reports, err := s.o.AccountReports.RecentReportsOf(ctx, req.Id, appidentity.AccountDetailHistory)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the reports of account %s: %w", req.Id, err)
	}
	if out.Reports, err = s.newAdminReports(ctx, reports); err != nil {
		return nil, err
	}
	cache := cacheNoStore
	return GetAdminAccount200JSONResponse{Body: out, Headers: GetAdminAccount200ResponseHeaders{CacheControl: &cache}}, nil
}

// RevealAdminAccount serves POST /api/v1/admin/accounts/{id}/reveal.
func (s *Server) RevealAdminAccount(ctx context.Context, req RevealAdminAccountRequestObject) (RevealAdminAccountResponseObject, error) {
	if p, err := s.requirePermission(ctx, identity.PermManageAccounts); err != nil {
		return nil, err
	} else if p != nil {
		return RevealAdminAccountdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.o.AccountAdmin.Reveal(ctx, req.Id, v.UserID)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		return RevealAdminAccountdefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	case errors.Is(err, appidentity.ErrNotLocalAccount):
		l := s.Localizer(ctx)
		return RevealAdminAccountdefaultApplicationProblemPlusJSONResponse(
			s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), l.ServerErrorsInvalidValue("id"))), nil
	case err != nil:
		return nil, fmt.Errorf("clientapi: reveal account %s: %w", req.Id, err)
	}
	out := AdminAccountPrivateData{SignIns: make([]AdminAccountSignIn, 0, len(d.SignIns)), SameIpAccounts: make([]AdminSameIpAccounts, 0, len(d.SameIP))}
	if d.Email != "" {
		email, verified := d.Email, d.EmailVerified
		out.Email, out.EmailVerified = &email, &verified
	}
	if r := d.Registration; r != nil {
		ip, at := r.IP, r.RecordedAt.UTC()
		out.RegistrationIp, out.RegisteredAt = &ip, &at
	}
	for _, o := range d.SignIns {
		item := AdminAccountSignIn{SignedInAt: o.CreatedAt.UTC()}
		if o.IP != "" {
			ip := o.IP
			item.Ip = &ip
		}
		if o.UserAgent != "" {
			ua := o.UserAgent
			item.UserAgent = &ua
		}
		out.SignIns = append(out.SignIns, item)
	}
	for _, g := range d.SameIP {
		accounts, err := s.newAdminAccounts(ctx, g.Accounts)
		if err != nil {
			return nil, err
		}
		out.SameIpAccounts = append(out.SameIpAccounts, AdminSameIpAccounts{Ip: g.IP, Accounts: accounts})
	}
	cache := cacheNoStore
	return RevealAdminAccount200JSONResponse{Body: out, Headers: RevealAdminAccount200ResponseHeaders{CacheControl: &cache}}, nil
}
