// SPDX-License-Identifier: AGPL-3.0-only
// The costs screen in every width × locale project (ADR-0067, ADR-0068, ADR-0027): an administrator with
// manage_finance sees the totals by month amortized by default, switches to weeks and to payment dates, records a cost
// paid every year, and deletes one after its dialog; the billing imports show their state. Each step passes axe
// without scrolling sideways. `vite preview` has no API, so the test plays the server with page.route; names change
// with the locale, so the elements are found by role and by the names in the project's locale catalog.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Schemas = components["schemas"];
type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale, with each `{name}` of `values` replaced. */
function message(key: string, values: Record<string, string> = {}): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return Object.entries(values).reduce((out, [name, value]) => out.replace(`{${name}}`, value), text);
}

/** The signed-in administrator, as GET /accounts/verify_credentials answers. */
const account = {
  id: "0192f0c4-7a8e-7000-8000-000000000001",
  username: "alice",
  acct: "alice",
  display_name: "Alice",
  note: "",
  fields: [],
  emojis: [],
  url: "https://my.example/@alice",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
  discoverable: true,
  indexable: true,
  source: { note: "", fields: [] },
  posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
  permissions: ["manage_finance"],
};

/** A monthly server plan with a long provider name and a memo in Arabic, which reads right to left. */
const server: Schemas["CostItem"] = {
  id: "0192f0c4-7a8e-7000-8000-0000000000a1",
  schedule: "months",
  interval: 1,
  starts_on: "2026-01-31",
  name: "Main server",
  provider: "a-very-long-cloud-provider-name-that-does-not-wrap-on-its-own.example",
  category: "compute",
  memo: "الخادم الرئيسي",
  prices: [{ effective_on: "2026-01-31", price: { amount: "12.34", currency: "USD" }, revised_at: "2026-01-31T00:00:00Z" }],
  created_at: "2026-01-31T00:00:00Z",
};

/** The totals of a query: one period of the granularity asked for, with yen and dollars and an estimate. */
function totals(url: URL): Schemas["CostTotals"] {
  const granularity = (url.searchParams.get("granularity") ?? "month") as Schemas["CostGranularity"];
  const basis = (url.searchParams.get("basis") ?? "amortized") as Schemas["CostBasis"];
  const period =
    granularity === "week"
      ? { label: "2026-W43", starts_on: "2026-10-19", ends_on: "2026-10-25" }
      : granularity === "year"
        ? { label: "2026", starts_on: "2026-01-01", ends_on: "2026-12-31" }
        : { label: "2026-10", starts_on: "2026-10-01", ends_on: "2026-10-31" };
  return {
    granularity,
    basis,
    periods: [
      {
        ...period,
        totals: [
          { amount: basis === "cash" ? "1650" : "140", currency: "JPY" },
          { amount: "12.34", currency: "USD" },
        ],
        breakdown: [{ provider: "registrar", category: "domain", amount: { amount: "140", currency: "JPY" } }],
        estimated: true,
      },
    ],
    rules: { week_start: "monday", sources: [{ source: "aws", provider: "AWS", time_zone: "UTC" }] },
  };
}

const awsImport: Schemas["BillingImport"] = {
  source: "aws",
  provider: "AWS",
  time_zone: "UTC",
  method: "cost_explorer",
  last_run: { outcome: "succeeded", error_kind: "none", requests: 1, bytes_billed: 0, occurred_at: "2026-10-21T01:00:00Z" },
  last_success_at: "2026-10-21T01:00:00Z",
  requests_today: 1,
  requests_this_month: 21,
  max_requests_per_day: 4,
  max_requests_per_month: 120,
  bytes_billed_this_month: 0,
  months: [{ month: "2026-10", kind: "estimate", totals: [{ amount: "31.24", currency: "USD" }] }],
};

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/** Plays the API of the administrator: the costs change with each write, which `sent` records. */
async function serveApi(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  const sent: string[] = [];
  let items = [server];
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow admin" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api\/v1/, "");
    const key = `${request.method()} ${path}`;
    if (key === "GET /instance") {
      return json(route, {
        domain: "my.example",
        name: "my example",
        version: "0.0.0",
        source_url: "https://github.com/best-friends/my",
        registrations: { mode: "approval" },
        limits: { max_post_characters: 500, require_media_description: false },
        timelines: { local: true, federated: true },
        rules: [],
      });
    }
    if (key === "GET /accounts/verify_credentials") return json(route, account);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /notifications/marker") return json(route, { last_read_id: "0", unread_count: 0 });
    if (key === "GET /admin/counts") return json(route, { open_reports: 0, pending_registrations: 0, pending_appeals: 0 });
    if (key === "GET /admin/audit_log") return json(route, { items: [] });
    if (key === "GET /admin/cost_totals") {
      sent.push(`totals ${url.searchParams.get("granularity") ?? ""} ${url.searchParams.get("basis") ?? ""}`);
      return json(route, totals(url));
    }
    if (key === "GET /admin/cost_items") return json(route, { items });
    if (key === "GET /admin/billing_imports") return json(route, { items: [awsImport] });
    if (key === "POST /admin/cost_items") {
      const { price, memo, ends_on, ...body } = request.postDataJSON() as Schemas["CostItemCreateRequest"];
      sent.push(`create ${body.schedule} ${price.amount} ${price.currency}`);
      const created: Schemas["CostItem"] = {
        ...body,
        id: "0192f0c4-7a8e-7000-8000-0000000000a2",
        memo: memo ?? "",
        ...(ends_on === undefined ? {} : { ends_on }),
        prices: [{ effective_on: body.starts_on, price, revised_at: "2026-10-21T00:00:00Z" }],
        created_at: "2026-10-21T00:00:00Z",
      };
      items = [created, ...items];
      return json(route, created);
    }
    if (/^POST \/admin\/cost_items\/[^/]+\/cancel$/.test(key)) {
      sent.push("cancel");
      items = items.filter((item) => !path.includes(item.id));
      return route.fulfill({ status: 204 });
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return sent;
}

test.describe("costs", () => {
  test("switches the totals, records a yearly cost and deletes one after its dialog", async ({ page, makeAxeBuilder }) => {
    const sent = await serveApi(page);
    await page.goto("/admin/finance");
    const main = page.getByRole("main");
    const totalsRegion = main.getByRole("region", { name: message("admin.finance.totals.title"), exact: true });
    await expect(totalsRegion.getByRole("table")).toBeVisible();
    await expect(totalsRegion.getByRole("radio", { name: new RegExp(message("costs.basis.amortized"), "u") })).toBeChecked();
    await expect(totalsRegion.getByText(message("costs.totals.estimated"), { exact: true })).toBeVisible();
    await expect(totalsRegion.getByText(message("costs.rules.week_start"), { exact: true })).toBeVisible();
    await expect(main.getByText("الخادم الرئيسي")).toHaveAttribute("dir", "auto");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    // The tabs follow the APG with automatic activation: the arrow key selects the weeks.
    await totalsRegion.getByRole("tab", { name: message("costs.granularity.month"), exact: true }).focus();
    await page.keyboard.press(test.info().project.use.locale === "ar-XB" ? "ArrowRight" : "ArrowLeft");
    await expect(totalsRegion.getByRole("tab", { name: message("costs.granularity.week"), exact: true })).toHaveAttribute("aria-selected", "true");
    await expect(totalsRegion.getByRole("row", { name: /2026-W43/u })).toBeVisible();
    await totalsRegion.getByRole("radio", { name: new RegExp(message("costs.basis.cash"), "u") }).check();
    await expect(totalsRegion.getByText(message("costs.rules.cash"), { exact: true })).toBeVisible();
    await expect(page).toHaveURL(/granularity=week/u);
    await expect(page).toHaveURL(/basis=cash/u);
    expect(sent).toContain("totals week cash");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    const form = main.getByRole("region", { name: message("admin.finance.new.title"), exact: true });
    await form.getByRole("textbox", { name: message("admin.finance.name"), exact: true }).fill("example.com");
    await form.getByRole("textbox", { name: message("admin.finance.provider"), exact: true }).fill("Registrar");
    await form.getByRole("radio", { name: message("admin.finance.schedule.years"), exact: true }).check();
    await form.getByLabel(message("admin.finance.starts_on"), { exact: true }).fill("2026-04-10");
    await form.getByRole("textbox", { name: message("admin.finance.amount"), exact: true }).fill("1650");
    await form.getByRole("textbox", { name: message("admin.finance.currency"), exact: true }).fill("JPY");
    await form.getByRole("button", { name: message("admin.finance.create"), exact: true }).click();
    const list = main.getByRole("region", { name: message("admin.finance.list.title"), exact: true });
    await expect(list.getByRole("heading", { level: 3, name: "example.com" })).toBeVisible();
    expect(sent).toContain("create years 1650 JPY");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await list.getByRole("button", { name: message("admin.finance.cancel_one", { name: "example.com" }), exact: true }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await dialog.getByRole("button", { name: message("admin.finance.cancel_item"), exact: true }).click();
    await expect(list.getByRole("heading", { level: 3, name: "example.com" })).toBeHidden();
    expect(sent).toContain("cancel");

    const imports = main.getByRole("region", { name: message("admin.finance.imports.title"), exact: true });
    await expect(imports.getByRole("heading", { level: 3, name: "AWS" })).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
