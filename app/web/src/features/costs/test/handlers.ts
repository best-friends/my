// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the costs feature (web-client.md §7): the public totals of the instance's costs.
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

export const handlers = [http.get(`${apiBase}/instance/cost_totals`, ({ request }) => HttpResponse.json(publicTotalsFixture(new URL(request.url))))];
