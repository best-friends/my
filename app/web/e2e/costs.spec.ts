// SPDX-License-Identifier: AGPL-3.0-only
// The public running costs (FR-903, ADR-0067 §8) in every width × locale project: the section of the information page
// and the page at /about/costs as the instance setting finance_visibility publishes them, against `vite preview`, whose
// client API the spec plays with page.route (no session). Names change with the locale, so elements are found by role,
// structure and addresses, and the providers, which are the operators' words, are in English in every project.
import type { Page } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Instance = components["schemas"]["Instance"];
type CostTotals = components["schemas"]["CostTotals"];

/** The totals of the periods of a query: the last one with costs in two currencies and, with `breakdown`, their breakdown. */
function totals(url: URL, breakdown: boolean): CostTotals {
  const to = url.searchParams.get("to") ?? "2026-10-22";
  const month = to.slice(0, 7);
  const start = new Date(`${month}-01T00:00:00Z`);
  start.setUTCMonth(start.getUTCMonth() - 1);
  const previous = start.toISOString().slice(0, 7);
  return {
    granularity: (url.searchParams.get("granularity") ?? "month") as CostTotals["granularity"],
    basis: (url.searchParams.get("basis") ?? "amortized") as CostTotals["basis"],
    periods: [
      { label: previous, starts_on: `${previous}-01`, ends_on: `${previous}-28`, totals: [{ amount: "1650", currency: "JPY" }], breakdown: [], estimated: false },
      {
        label: month,
        starts_on: `${month}-01`,
        ends_on: to,
        totals: [
          { amount: "1650", currency: "JPY" },
          { amount: "1234.56", currency: "USD" },
        ],
        breakdown: breakdown
          ? [
              { provider: "A registrar with a rather long name that has to wrap on the narrowest screens", category: "domain", amount: { amount: "1650", currency: "JPY" } },
              { provider: "Cloud", category: "compute", amount: { amount: "1234.56", currency: "USD" } },
            ]
          : [],
        estimated: true,
      },
    ],
    rules: { week_start: "monday", sources: breakdown ? [{ source: "aws", provider: "AWS", time_zone: "UTC" }] : [] },
  };
}

/** Plays the instance with `visibility` and its public totals for a visitor without a session. */
async function serveCosts(page: Page, visibility: Instance["finance_visibility"]): Promise<void> {
  const instance: Instance = {
    domain: "my.example",
    name: "my example",
    version: "0.0.0-e2e",
    source_url: "https://github.com/best-friends/my",
    registrations: { mode: "approval" },
    limits: { max_post_characters: 500, require_media_description: false },
    timelines: { local: true, federated: true },
    sends_email: true,
    search_indexing: "deny",
    ai_usage: "deny",
    finance_visibility: visibility,
    rules: [],
  };
  await page.route("**/api/v1/instance", (route) => route.fulfill({ json: instance }));
  await page.route("**/api/v1/instance/domain_actions*", (route) =>
    route.fulfill({ status: 404, contentType: "application/problem+json", json: { type: "about:blank", title: "Not Found", status: 404 } }),
  );
  await page.route("**/api/v1/instance/cost_totals*", (route) =>
    visibility === "private"
      ? route.fulfill({ status: 404, contentType: "application/problem+json", json: { type: "about:blank", title: "Not Found", status: 404 } })
      : route.fulfill({ json: totals(new URL(route.request().url()), visibility === "breakdown") }),
  );
}

test.describe("the costs page of a server that publishes the breakdown", () => {
  test.beforeEach(async ({ page }) => {
    await serveCosts(page, "breakdown");
    await page.goto("/about/costs");
    await expect(page.getByRole("main").getByRole("table")).toBeVisible();
  });

  test("shows the totals by period, the breakdown of the current one, and the rules", async ({ page }) => {
    const main = page.getByRole("main");
    await expect(main.getByRole("heading", { level: 1 })).toContainText("my example");
    await expect(main.getByRole("tab")).toHaveCount(3);
    await expect(main.getByRole("table").getByRole("row")).toHaveCount(3);
    await expect(main.locator('[dir="auto"]', { hasText: "Cloud" })).toBeVisible();
    await expect(main.getByRole("region")).toHaveCount(2);
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });
});

test("the costs page of a server that publishes the totals only", async ({ page, makeAxeBuilder }) => {
  await serveCosts(page, "totals");
  await page.goto("/about/costs");
  const main = page.getByRole("main");
  await expect(main.getByRole("table")).toBeVisible();
  // Only the rules: no breakdown.
  await expect(main.getByRole("region")).toHaveCount(1);
  await expect(main.getByText("Cloud")).toHaveCount(0);
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);
});

test("the costs page of a server that publishes nothing", async ({ page, makeAxeBuilder }) => {
  await serveCosts(page, "private");
  await page.goto("/about/costs");
  const main = page.getByRole("main");
  await expect(main.locator('a[href="/about"]')).toBeVisible();
  await expect(main.getByRole("table")).toHaveCount(0);
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);
});

test("the information page of a server that publishes its costs links to them", async ({ page, makeAxeBuilder }) => {
  await serveCosts(page, "totals");
  await page.goto("/about");
  const main = page.getByRole("main");
  await expect(main.locator('a[href="/about/costs"]')).toBeVisible();
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);
});
