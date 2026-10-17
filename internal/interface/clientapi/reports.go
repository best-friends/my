// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/moderation"
)

// ReportLimit bounds the reports that one account files (createReport): each report lands in
// the moderators' queue.
var ReportLimit = kernel.Limit{Name: "api.report", Max: 30, Window: time.Hour}

// Reports files the reports of local users (FR-701). *appmoderation.LocalReports implements it.
type Reports interface {
	File(ctx context.Context, in appmoderation.LocalReportInput) (moderation.Report, error)
}

// CreateReport serves POST /api/v1/reports: the viewer reports an account and some of its
// statuses to the moderators. The response presents the report without its reporter.
func (s *Server) CreateReport(ctx context.Context, req CreateReportRequestObject) (CreateReportResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if p, h, limited := s.LimitProblem(ctx, ReportLimit, v.AccountID); limited {
		return CreateReportdefaultApplicationProblemPlusJSONResponse{Body: p, Headers: h, StatusCode: http.StatusTooManyRequests}, nil
	}
	b := req.Body
	if b == nil {
		return CreateReportdefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "account_id")), nil
	}
	l := s.Localizer(ctx)
	unprocessable := func(detail string) (CreateReportResponseObject, error) {
		return CreateReportdefaultApplicationProblemPlusJSONResponse(
			s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), detail)), nil
	}
	r, err := s.o.Reports.File(ctx, appmoderation.LocalReportInput{
		ReporterAccountID: v.AccountID, TargetAccountID: b.AccountId, StatusIDs: deref(b.StatusIds),
		Category: moderation.Category(b.Category), RuleIDs: deref(b.RuleIds), Comment: deref(b.Comment),
		Forward: deref(b.Forward),
	})
	var (
		fieldErr *appmoderation.ReportFieldError
		textErr  *moderation.TextError
		countErr *moderation.CountError
	)
	switch {
	case errors.As(err, &fieldErr):
		return unprocessable(l.ServerErrorsInvalidValue(fieldErr.Field))
	case errors.As(err, &countErr):
		return unprocessable(l.ServerErrorsTooMany(countErr.Max, countErr.Field))
	case errors.As(err, &textErr) && textErr.Max > 0:
		return unprocessable(l.ServerErrorsTooLong(textErr.Max, textErr.Field))
	case errors.As(err, &textErr):
		return unprocessable(l.ServerErrorsInvalidValue(textErr.Field))
	case err != nil:
		return nil, fmt.Errorf("clientapi: file a report on %s: %w", b.AccountId, err)
	}
	target, err := s.o.Accounts.FindByID(ctx, r.TargetAccountID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the target %s of report %s: %w", r.TargetAccountID, r.ID, err)
	}
	accounts, err := s.newAccounts(ctx, []appaccount.Actor{target})
	if err != nil {
		return nil, err
	}
	cache := cacheNoStore
	return CreateReport200JSONResponse{
		Body: Report{
			Id: r.ID, Category: ReportCategory(r.Category), Comment: r.Comment, Forwarded: r.Forwarded,
			FiledAt: r.FiledAt.UTC(), TargetAccount: accounts[0],
			StatusIds: append([]string{}, r.StatusIDs...), RuleIds: append([]string{}, r.RuleIDs...),
		},
		Headers: CreateReport200ResponseHeaders{CacheControl: &cache},
	}, nil
}
