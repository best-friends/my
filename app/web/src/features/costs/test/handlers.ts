// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the costs feature (web-client.md §7): the public totals of the instance's costs, and the
// totals of the finances page.
import { http, HttpResponse } from "msw";
import type { Schemas } from "../../../test/fixtures";
import { apiBase } from "../../../test/msw/http";

/**
 * The public totals of the periods of a query (GET /instance/cost_totals): 12.34 USD and 1,650 yen in the last one,
 * with its breakdown, as an instance that publishes the breakdown answers it; nothing in the others.
 */
export function publicTotalsFixture(url: URL): Schemas["CostTotals"] {
  const granularity = (url.searchParams.get("granularity") ?? "month") as Schemas["CostGranularity"];
  const basis = (url.searchParams.get("basis") ?? "amortized") as Schemas["CostBasis"];
  const to = url.searchParams.get("to") ?? "2026-10-22";
  const month = to.slice(0, 7);
  return {
    granularity,
    basis,
    periods: [
      {
        label: month,
        starts_on: `${month}-01`,
        ends_on: to,
        totals: [
          { amount: "1650", currency: "JPY" },
          { amount: "12.34", currency: "USD" },
        ],
        breakdown: [
          { provider: "Registrar", category: "domain", amount: { amount: "1650", currency: "JPY" } },
          { provider: "Cloud", category: "compute", amount: { amount: "12.34", currency: "USD" } },
        ],
        estimated: false,
      },
    ],
    rules: { week_start: "monday", sources: [] },
  };
}

/**
 * The totals of the finances page (GET /finance/cost_totals) as a server with `viewer` answers them: the public
 * fixture's periods, with the lines of the last one's items (two, one with a memo) when names or memos are shown, and
 * only the fields the settings show.
 */
export function financeTotalsFixture(url: URL, viewer: Schemas["FinanceViewerSettings"] = { item_names: false, item_memos: false, income: "totals" }): Schemas["FinanceCostTotals"] {
  const totals = publicTotalsFixture(url);
  const lines: Schemas["FinanceCostItemLine"][] = [
    { provider: "Cloud", category: "compute", amount: { amount: "12.34", currency: "USD" }, name: "Main VM", memo: "the main server" },
    { provider: "Registrar", category: "domain", amount: { amount: "1650", currency: "JPY" }, name: "example.com" },
  ];
  const shown = viewer.item_names || viewer.item_memos;
  return {
    ...totals,
    viewer,
    periods: totals.periods.map((period, index) => ({
      ...period,
      items:
        shown && index === totals.periods.length - 1
          ? lines.map(({ name, memo, ...line }) => ({
              ...line,
              ...(viewer.item_names && name !== undefined ? { name } : {}),
              ...(viewer.item_memos && memo !== undefined ? { memo } : {}),
            }))
          : [],
    })),
  };
}

export const handlers = [
  http.get(`${apiBase}/instance/cost_totals`, ({ request }) => HttpResponse.json(publicTotalsFixture(new URL(request.url)))),
  http.get(`${apiBase}/finance/cost_totals`, ({ request }) => HttpResponse.json(financeTotalsFixture(new URL(request.url)))),
];
