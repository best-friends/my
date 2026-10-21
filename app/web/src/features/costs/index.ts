// SPDX-License-Identifier: AGPL-3.0-only
// The costs feature (FR-902, FR-903, ADR-0067, ADR-0070): the totals of the instance's running costs by week, month or
// year as the public costs page (`/about/costs`) shows what the instance setting `finance_visibility` publishes and
// the finances page (`/finance`) shows them to the holders of view_finance, with the navigation's entry to it, and the
// parts of the totals that the administrators' costs screen (the admin feature) shows too.
// Routes and other features use it through this file only (web-client.md A3).
export {
  type CostBasis,
  type CostCategory,
  costCategories,
  type CostGranularity,
  type CostTotals,
  deviceToday,
  financeCostTotalsQueryOptions,
  type Money,
  publicCostTotalsQueryOptions,
  totalsQuery,
  utcDate,
} from "./api/costs";
export { categoryLabels, CostTotalsPanel, CostTotalsView } from "./components/CostTotals";
export { FinanceCosts } from "./components/FinanceCosts";
export { PublicCosts } from "./components/PublicCosts";
export { useFinanceEntry } from "./hooks/useFinanceCosts";
export { useFormatMoney, useFormatTotals } from "./hooks/useCostFormat";
