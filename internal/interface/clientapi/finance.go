// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/best-friends/my/internal/domain/finance"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
)

// The instance's finances as the holders of view_finance read them (ADR-0070 §5): the user
// namespace's /finance operations (tag finance, scope read:finance), never /admin/*, so that a
// user who is not staff needs no admin scope.

// cachePrivateNoStore is the Cache-Control of the finance answers: they are the same for every
// viewer, but a permission guards them (ADR-0070 §5).
const cachePrivateNoStore = "private, no-store"

// requireFinanceViewer checks that the viewer may read the finances, with view_finance or
// manage_finance, and that the use cases are configured; it returns whether the viewer holds
// manage_finance, who reads every item line whatever the instance settings, or the problem to
// answer otherwise.
func (s *Server) requireFinanceViewer(ctx context.Context) (manage bool, _ *problem, _ error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return false, nil, err
	}
	if s.o.Permissions != nil {
		if manage, err = s.o.Permissions.HasPermission(ctx, v.UserID, identity.PermManageFinance); err != nil {
			return false, nil, fmt.Errorf("clientapi: permissions of %s: %w", v.UserID, err)
		}
	}
	if !manage {
		if p, err := s.requirePermission(ctx, identity.PermViewFinance); err != nil || p != nil {
			return false, p, err
		}
	}
	if s.o.Costs == nil {
		p := s.notFoundResponse(ctx)
		return false, &p, nil
	}
	return manage, nil, nil
}

// GetFinanceCostTotals serves GET /api/v1/finance/cost_totals: the totals of GET
// /admin/cost_totals with their breakdown and rules, and the item lines with the names and the
// memos that the instance settings finance_viewer_item_names and finance_viewer_item_memos
// show; a holder of manage_finance reads them all.
func (s *Server) GetFinanceCostTotals(ctx context.Context, req GetFinanceCostTotalsRequestObject) (GetFinanceCostTotalsResponseObject, error) {
	manage, p, err := s.requireFinanceViewer(ctx)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return GetFinanceCostTotalsdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	var settings instance.Settings
	if s.o.Instance != nil {
		if settings, err = s.o.Instance.Settings(ctx); err != nil {
			return nil, fmt.Errorf("clientapi: instance settings: %w", err)
		}
	}
	viewer := settings.FinanceViewer()
	names, memos := viewer.ItemNames || manage, viewer.ItemMemos || manage
	params := req.Params
	totals, basis, p, err := s.readCostTotals(ctx, costTotalsQuery{
		granularity: params.Granularity, from: params.From, to: params.To, basis: params.Basis, breakdown: true, items: names || memos,
	})
	if err != nil {
		return nil, err
	}
	if p != nil {
		return GetFinanceCostTotalsdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	out := FinanceCostTotals{
		Granularity: params.Granularity, Basis: basis, Periods: make([]FinanceCostPeriodTotal, 0, len(totals)), Rules: s.costRules(),
		Viewer: FinanceViewerSettings{ItemNames: viewer.ItemNames, ItemMemos: viewer.ItemMemos, Income: FinanceViewerIncome(viewer.Income)},
	}
	for _, t := range totals {
		c := newCostPeriodTotal(t, true)
		period := FinanceCostPeriodTotal{
			Label: c.Label, StartsOn: c.StartsOn, EndsOn: c.EndsOn, Estimated: c.Estimated, Totals: c.Totals, Breakdown: c.Breakdown,
			Items: make([]FinanceCostItemLine, 0, len(t.Items)),
		}
		lines := t.Items
		if !names {
			// The lines are sorted by name; without the names, an order by amount tells nothing of them.
			lines = slices.Clone(lines)
			slices.SortStableFunc(lines, func(a, b finance.ItemLine) int {
				return cmp.Or(cmp.Compare(a.Provider, b.Provider), cmp.Compare(a.Category, b.Category),
					cmp.Compare(a.Amount.Currency, b.Amount.Currency), cmp.Compare(b.Amount.Minor, a.Amount.Minor))
			})
		}
		for _, line := range lines {
			period.Items = append(period.Items, newFinanceCostItemLine(line, names, memos))
		}
		out.Periods = append(out.Periods, period)
	}
	cache := cachePrivateNoStore
	return GetFinanceCostTotals200JSONResponse{Body: out, Headers: GetFinanceCostTotals200ResponseHeaders{CacheControl: &cache}}, nil
}

// newFinanceCostItemLine presents an item line, with its name when names is true and its memo
// when memos is true and it has one.
func newFinanceCostItemLine(line finance.ItemLine, names, memos bool) FinanceCostItemLine {
	out := FinanceCostItemLine{Provider: line.Provider, Category: CostCategory(line.Category), Amount: newMoney(line.Amount)}
	if names {
		name := line.Item.Name
		out.Name = &name
	}
	if memos && line.Item.Memo != "" {
		memo := line.Item.Memo
		out.Memo = &memo
	}
	return out
}
