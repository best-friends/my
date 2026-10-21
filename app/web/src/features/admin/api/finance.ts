// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the costs screen (ADR-0067 §9, ADR-0068 §9, manage_finance, web-client.md S1): the
// cost items (GET /admin/cost_items, newest first, as an infinite query), the totals of a range of weeks, months or
// years under a basis (GET /admin/cost_totals), the state of the billing imports (GET /admin/billing_imports),
// recording, relabeling, revising, ending and canceling an item, and importing a provider's charges now. Every change
// goes to the audit log and changes the totals.
import { infiniteQueryOptions, type QueryClient, queryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";
import { ApiError } from "../../../lib/errors";
import { pageQuery, refreshAuditLog } from "../../moderation";

type Schemas = components["schemas"];

export type CostItem = Schemas["CostItem"];
export type CostSchedule = Schemas["CostSchedule"];
export type CostCategory = Schemas["CostCategory"];
export type CostBasis = Schemas["CostBasis"];
export type CostGranularity = Schemas["CostGranularity"];
export type CostTotals = Schemas["CostTotals"];
export type CostPeriodTotal = Schemas["CostPeriodTotal"];
export type Money = Schemas["Money"];
export type NewCostItem = Schemas["CostItemCreateRequest"];
export type CostItemLabel = Schemas["CostItemLabelRequest"];
export type BillingImport = Schemas["BillingImport"];

/** The schedules, in the order of the form. */
export const costSchedules = ["once", "weeks", "months", "years"] as const satisfies readonly CostSchedule[];

/** The categories, in the order of the API (ADR-0067 §2). */
export const costCategories = [
  "compute",
  "database",
  "storage",
  "network",
  "domain",
  "email",
  "software",
  "other",
] as const satisfies readonly CostCategory[];

/** The granularities of the totals, in the order of their tabs. */
export const costGranularities = ["week", "month", "year"] as const satisfies readonly CostGranularity[];

/** The bases of the totals; amortized is the default (ADR-0067 §4). */
export const costBases = ["amortized", "cash"] as const satisfies readonly CostBasis[];

/** The most N of an every-N schedule, and the longest name, provider and memo (CostItemCreateRequest). */
export const maxCostInterval = 99;
export const costNameLimit = 100;
export const costMemoLimit = 10_000;

/** How many periods back from the current one the totals show, the current one included. */
const shownPeriods: Readonly<Record<CostGranularity, number>> = { week: 12, month: 12, year: 5 };

/** A calendar date `YYYY-MM-DD` as a UTC Date, for date arithmetic and for Intl in the UTC time zone. */
export function utcDate(date: string): Date {
  return new Date(`${date}T00:00:00Z`);
}

/** The calendar date `YYYY-MM-DD` of a UTC Date. */
function dateString(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/**
 * Today's calendar date in the device's time zone (ADR-0067 §5: only "today" depends on the viewer, and the API
 * takes dates). Called outside rendering, by the route.
 */
export function deviceToday(now: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${String(now.getFullYear()).padStart(4, "0")}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/** The dates of the totals: from a date in the first of the shown periods to `today`, in the last one. */
function totalsRange(granularity: CostGranularity, today: string): { from: string; to: string } {
  const date = utcDate(today);
  const back = shownPeriods[granularity] - 1;
  if (granularity === "week") date.setUTCDate(date.getUTCDate() - 7 * back);
  // The first day of the month or year: stepping back from the 31st would skip a shorter month.
  else if (granularity === "month") date.setUTCFullYear(date.getUTCFullYear(), date.getUTCMonth() - back, 1);
  else date.setUTCFullYear(date.getUTCFullYear() - back, 0, 1);
  return { from: dateString(date), to: today };
}

/** The price of an item's payment on `date`: the revision with the latest effective date on or before it, the last recorded of a date. */
export function priceOn(item: CostItem, date: string): Money | undefined {
  let best: Schemas["CostPriceRevision"] | undefined;
  for (const revision of item.prices) {
    if (revision.effective_on > date) continue;
    if (best === undefined || revision.effective_on >= best.effective_on) best = revision;
  }
  return best?.price;
}

/** The cost items that are not canceled, newest first (GET /admin/cost_items). */
export function costItemsQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/cost_items", "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/cost_items", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The totals of the shown periods up to `today` (GET /admin/cost_totals). */
export function costTotalsQueryOptions(api: Api, granularity: CostGranularity, basis: CostBasis, today: string) {
  const query = { granularity, basis, ...totalsRange(granularity, today) };
  return queryOptions({
    queryKey: ["get", "/admin/cost_totals", query] as const,
    queryFn: async ({ signal }) => {
      const { data, error } = await api.fetch.GET("/admin/cost_totals", { params: { query }, signal });
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  });
}

/** The state of the imports of every billing source (GET /admin/billing_imports). */
export function billingImportsQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/admin/billing_imports");
}

/** Records a cost item (POST /admin/cost_items), as the options of useMutation. */
export function createCostItemMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/cost_items"] as const,
    mutationFn: async (body: NewCostItem): Promise<CostItem> => {
      const { data, error } = await api.fetch.POST("/admin/cost_items", { body });
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Changes the label of a cost item (PATCH /admin/cost_items/{id}). */
export function updateCostItemMutation(api: Api) {
  return {
    mutationKey: ["patch", "/admin/cost_items/{id}"] as const,
    mutationFn: async ({ id, body }: { id: string; body: CostItemLabel }): Promise<CostItem> => {
      const { data, error } = await api.fetch.PATCH("/admin/cost_items/{id}", { params: { path: { id } }, body });
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Revises the price of a cost item from a date on (POST /admin/cost_items/{id}/price_revisions). */
export function reviseCostItemPriceMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/cost_items/{id}/price_revisions"] as const,
    mutationFn: async ({ id, body }: { id: string; body: Schemas["CostPriceRevisionRequest"] }): Promise<CostItem> => {
      const { data, error } = await api.fetch.POST("/admin/cost_items/{id}/price_revisions", { params: { path: { id } }, body });
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Sets the end of a cost item, or none without `ends_on` (PUT /admin/cost_items/{id}/end). */
export function setCostItemEndMutation(api: Api) {
  return {
    mutationKey: ["put", "/admin/cost_items/{id}/end"] as const,
    mutationFn: async ({ id, endsOn }: { id: string; endsOn: string | undefined }): Promise<CostItem> => {
      const body = endsOn === undefined ? {} : { ends_on: endsOn };
      const { data, error } = await api.fetch.PUT("/admin/cost_items/{id}/end", { params: { path: { id } }, body });
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Cancels a cost item for good (POST /admin/cost_items/{id}/cancel). */
export function cancelCostItemMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/cost_items/{id}/cancel"] as const,
    mutationFn: async (id: string): Promise<void> => {
      // The middleware throws for every error status; a 204 has no body to read.
      await api.fetch.POST("/admin/cost_items/{id}/cancel", { params: { path: { id } } });
    },
  };
}

/** Queues an import of a billing source now (POST /admin/billing_imports/{source}/runs). */
export function runBillingImportMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/billing_imports/{source}/runs"] as const,
    mutationFn: async (source: Schemas["BillingSourceName"]): Promise<void> => {
      await api.fetch.POST("/admin/billing_imports/{source}/runs", { params: { path: { source } } });
    },
  };
}

/**
 * Marks what a change of the cost items changes (ADR-0045): the list, every range of totals and the audit log; the
 * queries shown now are fetched again and awaited.
 */
export async function refreshCosts(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ["get", "/admin/cost_items", "pages"] }),
    queryClient.invalidateQueries({ queryKey: ["get", "/admin/cost_totals"] }),
    refreshAuditLog(queryClient),
  ]);
}

/** Marks what an import started by hand changes: the state of the imports and the audit log. */
export async function refreshBillingImports(queryClient: QueryClient): Promise<void> {
  await Promise.all([queryClient.invalidateQueries({ queryKey: ["get", "/admin/billing_imports"] }), refreshAuditLog(queryClient)]);
}
