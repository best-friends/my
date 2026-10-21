// SPDX-License-Identifier: AGPL-3.0-only
// The public running costs through the whole app (web-client.md T4, FR-903, ADR-0067 §8): the section of the
// information page and the page at /about/costs show what the instance setting finance_visibility publishes, for
// everyone, signed in or not: nothing, the totals only, or the totals with their breakdown by provider and category.
import { screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import { instanceFixture, type Schemas } from "../../test/fixtures";
import { apiBase, problem } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { deviceToday } from "./api/costs";
import { publicTotalsFixture } from "./test/handlers";

beforeEach(() => {
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

/** Plays the instance with `visibility` and the public totals as an instance with it answers them; records the queries. */
function serveCosts(visibility: Schemas["Instance"]["finance_visibility"]) {
  const queries: Record<string, string>[] = [];
  server.use(
    http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, finance_visibility: visibility })),
    http.get(`${apiBase}/instance/cost_totals`, ({ request }) => {
      const url = new URL(request.url);
      queries.push(Object.fromEntries(url.searchParams));
      if (visibility === "private") return problem(404, "about:blank", "Not Found");
      const totals = publicTotalsFixture(url);
      if (visibility === "breakdown") return HttpResponse.json(totals);
      return HttpResponse.json({ ...totals, periods: totals.periods.map((period) => ({ ...period, breakdown: [] })) });
    }),
  );
  return { queries };
}

describe("a server that publishes nothing", () => {
  test("has no costs on its information page", async () => {
    serveCosts("private");
    await renderApp({ path: "/about" });

    await screen.findByRole("region", { name: "Search engines and AI" });
    expect(screen.queryByRole("region", { name: "Running costs" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "See the running costs" })).not.toBeInTheDocument();
  });

  test("says so on the costs page, without any amount", async () => {
    serveCosts("private");
    await renderApp({ path: "/about/costs" });

    expect(await screen.findByRole("heading", { level: 1, name: "Running costs of my example" })).toBeInTheDocument();
    expect(await screen.findByRole("heading", { level: 2, name: "This server does not publish its running costs." })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "About this server" })).toHaveAttribute("href", "/about");
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});

describe("a server that publishes its totals", () => {
  test("links to them from its information page", async () => {
    serveCosts("totals");
    await renderApp({ path: "/about" });

    const section = await screen.findByRole("region", { name: "Running costs" });
    expect(section).toHaveTextContent("This server publishes the totals of what it costs to run by week, month, and year.");
    expect(within(section).getByRole("link", { name: "See the running costs" })).toHaveAttribute("href", "/about/costs");
  });

  test("shows the totals by period without the breakdown, and switches to weeks and cash", async () => {
    const { queries } = serveCosts("totals");
    const { user, router } = await renderApp({ path: "/about/costs" });

    const table = await screen.findByRole("table", { name: "Totals by Months, Spread over the period paid for" });
    expect(within(table).getAllByRole("row")[1]).toHaveTextContent("¥1,650 + $12.34");
    expect(screen.getByText("This server publishes the totals only, not what they are spent on.")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /^Breakdown of/u })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "How the periods are counted" })).toHaveTextContent("Each currency is totaled on its own and never converted.");
    expect(queries[0]).toEqual({ granularity: "month", basis: "amortized", from: expect.stringMatching(/-01$/u) as string, to: deviceToday() });

    await user.click(screen.getByRole("tab", { name: "Weeks" }));
    expect(await screen.findByRole("table", { name: "Totals by Weeks, Spread over the period paid for" })).toBeInTheDocument();
    // The panel of the selected tab is the one its aria-controls names, labeled by it.
    const weeks = screen.getByRole("tab", { name: "Weeks" });
    const panel = screen.getByRole("tabpanel");
    expect(weeks).toHaveAttribute("aria-controls", panel.id);
    expect(panel).toHaveAttribute("aria-labelledby", weeks.id);
    await user.click(screen.getByRole("radio", { name: /On the payment date/u }));
    expect(await screen.findByRole("table", { name: "Totals by Weeks, On the payment date" })).toBeInTheDocument();
    expect(router.state.location.search).toEqual({ granularity: "week", basis: "cash" });
    expect(queries.at(-1)).toMatchObject({ granularity: "week", basis: "cash" });
  });
});

describe("a server that publishes the breakdown", () => {
  test("shows the breakdown of the current period by provider and category", async () => {
    serveCosts("breakdown");
    await renderApp({ path: "/about/costs" });

    const breakdown = await screen.findByRole("region", { name: /^Breakdown of/u });
    const lines = within(breakdown).getAllByRole("listitem");
    expect(lines.map((line) => line.textContent)).toEqual(["RegistrarDomain¥1,650", "CloudServers$12.34"]);
    expect(within(breakdown).getByText("Cloud")).toHaveAttribute("dir", "auto");
    expect(screen.queryByText("This server publishes the totals only, not what they are spent on.")).not.toBeInTheDocument();
  });

  test("says what it publishes on its information page", async () => {
    serveCosts("breakdown");
    await renderApp({ path: "/about" });

    const section = await screen.findByRole("region", { name: "Running costs" });
    expect(section).toHaveTextContent("with a breakdown by provider and category");
  });
});
