// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen in every width × locale project (ADR-0027): an administrator names the server in
// a language, and the tab title and the sidebar call the server by that name instead of its domain at once, and
// changes the registration mode with the keyboard, each step passing axe without scrolling sideways. `vite preview` has no API, so each test plays the server of
// a signed-in administrator with page.route; names change with the locale, so the structure is checked by role,
// by the names in the project's locale catalog and by what the server sends.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale. */
function message(key: string): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text;
}

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
  ai_opt_out: false,
  source: { note: "", fields: [] },
  posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
  permissions: ["manage_settings"],
};

// The longest name the server takes, in one word, to show that the sidebar wraps it rather than scroll.
const longName = `Birds${"-and-more".repeat(9)}-of-the-${"x".repeat(5)}`;

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/** The instance settings of the form, at their defaults, as GET /admin/settings lists them; `stored` replaces values. */
function instanceSettings(stored: Readonly<Record<string, string>>) {
  const items = [
    { key: "registration_mode", kind: "enum", default: "approval", choices: ["open", "approval", "invite", "closed"] },
    { key: "default_locale", kind: "locale", default: "en" },
    { key: "local_timeline_enabled", kind: "bool", default: "true" },
    { key: "federated_timeline_enabled", kind: "bool", default: "true" },
    { key: "require_media_description", kind: "bool", default: "false" },
    { key: "remote_media_policy", kind: "enum", default: "none", choices: ["none", "avatars", "all"] },
    { key: "remote_media_cache_ttl", kind: "int", default: "86400", minimum: 60, maximum: 31536000 },
    { key: "authorized_fetch", kind: "bool", default: "false" },
    { key: "domain_actions_visibility", kind: "enum", default: "none", choices: ["none", "users", "all"] },
    { key: "domain_actions_rationale_visibility", kind: "enum", default: "none", choices: ["none", "users", "all"] },
    { key: "search_indexing", kind: "enum", default: "deny", choices: ["allow", "deny"] },
    { key: "ai_usage", kind: "enum", default: "deny", choices: ["allow", "deny_training", "deny"] },
    { key: "indexable_default", kind: "bool", default: "false" },
    { key: "security_contact", kind: "uri", default: "" },
    { key: "link_cards_enabled", kind: "bool", default: "true" },
  ];
  return { items: items.map((item) => ({ ...item, value: stored[item.key] ?? item.default, stored: item.key in stored })) };
}

/**
 * Plays the API of a signed-in administrator; GET /instance names the server by its English name. Returns the
 * changes of the names (`<locale> <name>`) and of the settings (`<key>=<value>`).
 */
async function administratorServer(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  const names = new Map<string, string>([["en", "my example"]]);
  const settings: Record<string, string> = {};
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
        limits: { max_post_characters: 500, require_media_description: false },
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
    if (key === "GET /admin/settings") return json(route, instanceSettings(settings));
    if (key === "PATCH /admin/settings") {
      const { values } = (request.postDataJSON() ?? {}) as { values: Record<string, string> };
      for (const [setting, value] of Object.entries(values)) {
        changes.push(`${setting}=${value}`);
        settings[setting] = value;
      }
      return json(route, instanceSettings(settings));
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
    const name = main.getByRole("region", { name: message("admin.settings.name.title"), exact: true });
    await expect(name).toMatchAriaSnapshot(`
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
    await name.getByRole("button").click();
    await expect(name.getByRole("status")).not.toBeEmpty();
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

test.describe("server policies", () => {
  test("takes invites only, with the keyboard", async ({ page, makeAxeBuilder }) => {
    const changes = await administratorServer(page);
    await page.goto("/admin/settings");
    const main = page.getByRole("main");
    await expect(main.getByRole("textbox").first()).toHaveValue("my example");
    const registration = main.getByRole("radiogroup", { name: message("admin.settings.registration.mode"), exact: true });
    // The radio group holds the saved mode (approval); Down Arrow selects the next one, invite, in every direction.
    await registration.getByRole("radio", { checked: true }).focus();
    await page.keyboard.press("ArrowDown");
    await expect(registration.getByRole("radio").nth(2)).toBeChecked();
    const save = main.getByRole("button", { name: message("admin.settings.policies.save"), exact: true });
    await save.focus();
    await page.keyboard.press("Enter");
    await expect(main.getByRole("status").last()).not.toBeEmpty();
    expect(changes).toEqual(["registration_mode=invite"]);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
