// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"context"
	"fmt"
	"slices"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

// The reads in a transaction: each answers as the method of the same name without Tx does, read
// in tx with what tx has written, for the use cases that present what an event's transaction
// wrote (the data of the webhook messages, ADR-0060).

// ReportRecordTx returns a report with its state as ReportRecord does, read in tx.
func (s *Service) ReportRecordTx(ctx context.Context, tx uow.Tx, id string) (moderation.ReportRecord, error) {
	r, err := s.d.Repository.FindReportTx(ctx, tx, id)
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	st, err := s.d.Repository.FindReportStatusTx(ctx, tx, id)
	if err != nil {
		return moderation.ReportRecord{}, err
	}
	return moderation.ReportRecord{Report: r, Status: st}, nil
}

// AccountActionTx returns an action on an account as AccountAction does, read in tx.
func (s *Service) AccountActionTx(ctx context.Context, tx uow.Tx, id string) (AccountActionView, error) {
	r, err := s.d.Repository.FindActionRecordTx(ctx, tx, id)
	if err != nil {
		return AccountActionView{}, err
	}
	return s.accountView(r), nil
}

// AppealTx returns an appeal with its decision and action as Appeal does, read in tx.
func (s *Service) AppealTx(ctx context.Context, tx uow.Tx, id string) (AppealView, error) {
	r, err := s.d.Repository.FindAppealTx(ctx, tx, id)
	if err != nil {
		return AppealView{}, err
	}
	a, err := s.AccountActionTx(ctx, tx, r.Appeal.ActionID)
	if err != nil {
		return AppealView{}, fmt.Errorf("moderation: the action of appeal %s: %w", r.Appeal.ID, err)
	}
	return AppealView{AppealRecord: r, Action: a}, nil
}

// DomainActionTx returns an action on a domain as DomainAction does, read in tx.
func (s *Service) DomainActionTx(ctx context.Context, tx uow.Tx, id string) (DomainActionView, error) {
	r, err := s.d.Repository.FindDomainActionRecordTx(ctx, tx, id)
	if err != nil {
		return DomainActionView{}, err
	}
	return s.view(r), nil
}

// OwnRestrictionsTx returns the restrictions of an account's own actions as OwnRestrictions
// does, read in tx.
func (p *AccountPolicy) OwnRestrictionsTx(ctx context.Context, tx uow.Tx, accountID string) ([]moderation.Restriction, error) {
	return p.repo.AccountRestrictionsInEffectTx(ctx, tx, accountID, p.clock.Now())
}

// RejectsMediaTx reports whether the media of a domain are rejected as RejectsMedia does, read
// in tx.
func (p *FederationPolicy) RejectsMediaTx(ctx context.Context, tx uow.Tx, domain string) (bool, error) {
	domains := moderation.DomainAndParents(domain)
	if len(domains) == 0 {
		return false, nil
	}
	rs, err := p.repo.DomainRestrictionsInEffectTx(ctx, tx, domains, p.clock.Now())
	return slices.Contains(rs, moderation.RestrictMediaFetch), err
}
