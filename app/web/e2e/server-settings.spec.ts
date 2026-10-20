// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen in every width × locale project (ADR-0027): an administrator names the server in
// a language, and the tab title and the sidebar call the server by that name instead of its domain at once,
// each step passing axe without scrolling sideways. `vite preview` has no API, so each test plays the server of
// a signed-in administrator with page.route; names change with the locale, so the structure is checked by role
// and by what the server sends.
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

/** The signed-in administrator's account, as GET /accounts/verify_credentials answers, with manage_settings. */
const administrator = {
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
  permissions: ["manage_settings"],
};

// The longest name the server takes, in one word, to show that the sidebar wraps it rather than scroll.
const longName = `Birds${"-and-more".repeat(9)}-of-the-${"x".repeat(5)}`;

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/** Plays the API of a signed-in administrator; GET /instance names the server by its English name. Returns the changes. */
async function administratorServer(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  const names = new Map<string, string>([["en", "my example"]]);
  const changes: string[] = [];
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow admin" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api\/v1/, "");
    if (request.headers()["authorization"] !== "Bearer e2e-token" && path !== "/instance") return json(route, { type: "about:blank", title: "Unauthorized", status: 401 }, 401);
    const key = `${request.method()} ${path}`;
    if (key === "GET /instance") {
      return json(route, {
        domain: "my.example",
        name: names.get("en") ?? "my.example",
        version: "0.0.0",
        source_url: "https://github.com/best-friends/my",
        registrations: { mode: "approval" },
        limits: { max_post_characters: 500 },
        timelines: { local: true, federated: true },
        rules: [],
      });
    }
    if (key === "GET /accounts/verify_credentials") return json(route, administrator);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /admin/instance_texts") {
      return json(route, { items: [...names].map(([textLocale, body]) => ({ key: "name", locale: textLocale, body, revised_at: "2026-10-16T00:00:00Z" })) });
    }
    const set = /^PUT \/admin\/instance_texts\/name\/([^/]+)$/.exec(key);
    if (set) {
      const textLocale = decodeURIComponent(set[1] ?? "");
      const { body } = (request.postDataJSON() ?? {}) as { body: string };
      changes.push(`${textLocale} ${body}`);
      if (body === "") names.delete(textLocale);
      else names.set(textLocale, body);
      return json(route, { key: "name", locale: textLocale, body, revised_at: "2026-10-17T00:00:00Z" });
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return changes;
}

test.describe("page /admin/settings", () => {
  test.beforeEach(async ({ page }) => {
    await administratorServer(page);
    await page.goto("/admin/settings");
    await expect(page.getByRole("main").getByRole("textbox").first()).toHaveValue("my example");
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });
});

test.describe("server settings", () => {
  test("names the server in English, and the tab title and the sidebar call it by the new name", async ({ page, makeAxeBuilder }) => {
    const changes = await administratorServer(page);
    await page.goto("/admin/settings");
    const main = page.getByRole("main");
    const english = main.getByRole("textbox").first();
    await expect(english).toHaveValue("my example");
    await expect(page).toHaveTitle(/my example/);
    await expect(main.getByRole("region")).toMatchAriaSnapshot(`
      - region:
        - heading [level=2]
        - paragraph: /my\\.example/
        - text: /.+/
        - textbox: my example
        - text: /.+/
        - textbox
        - button
        - status
    `);

    await english.fill(longName);
    await main.getByRole("button").last().click();
    await expect(main.getByRole("status")).not.toBeEmpty();
    expect(changes).toEqual([`en ${longName}`]);
    await expect(page).toHaveTitle(new RegExp(longName));
    if ((page.viewportSize()?.width ?? 0) >= 1024) {
      // The sidebar shows the server's name above the list from lg on.
      await expect(page.getByRole("navigation").getByText(longName)).toBeVisible();
    }
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
