// SPDX-License-Identifier: AGPL-3.0-only
// The audit log screen in every width × locale project (ADR-0027, FR-704): operations by a moderator with a name in
// right-to-left letters on an account and a long domain, filtered by a row's actor and by the kind of operation, and
// loading more, each passing axe without scrolling sideways, with the filters and pages in the URL. `vite preview`
// has no API, so each test plays the server of a signed-in operator with page.route; names change with the
// locale, so the structure is checked by role and by what the server sends (names, handles, domains).
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale (only messages without arguments). */
function message(key: string): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text;
}

/** The signed-in operator's account, as GET /accounts/verify_credentials answers, with view_audit. */
const operator = {
  id: "0192f0c4-7a8e-7000-8000-000000000001",
  username: "alice",
  acct: "alice",
  display_name: "Alice",
  note: "",
  fields: [],
  url: "https://my.example/@alice",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
  discoverable: true,
  indexable: true,
  source: { note: "", fields: [] },
  posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
  permissions: ["view_audit"],
};

/** A moderator whose name is in right-to-left letters. */
const moderator = {
  id: "0192f0c4-7a8e-7000-8000-000000000022",
  username: "abdulrahman",
  acct: "abdulrahman",
  display_name: "عبد الرحمن بن محمد الهاشمي",
  note: "",
  fields: [],
  url: "https://my.example/@abdulrahman",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

const longDomain = "a-very-long-subdomain-of-a-server-with-many-accounts.social.example";
const targetHandle = `@maximiliana_theodora@${longDomain}`;

type Entry = Record<string, unknown> & { id: string; action: string; actor_account_id?: string };

const entries: Entry[] = [
  {
    id: "0192f0c4-7a8e-7000-8000-0000000a0004",
    actor_kind: "user",
    actor_account_id: moderator.id,
    actor_account: moderator,
    action: "account_action_issued",
    target_kind: "account",
    target_id: "0192f0c4-7a8e-7000-8000-000000000031",
    target_label: targetHandle,
    target_exists: true,
    occurred_at: "2026-10-17T09:30:00Z",
  },
  {
    id: "0192f0c4-7a8e-7000-8000-0000000a0003",
    actor_kind: "user",
    actor_account_id: moderator.id,
    actor_account: moderator,
    action: "domain_action_issued",
    target_kind: "domain",
    target_id: longDomain,
    target_label: longDomain,
    target_exists: true,
    occurred_at: "2026-10-16T09:30:00Z",
  },
  {
    id: "0192f0c4-7a8e-7000-8000-0000000a0002",
    actor_kind: "cli",
    action: "setting_changed",
    target_kind: "setting",
    target_id: "registration_mode",
    target_label: "registration_mode",
    target_exists: true,
    occurred_at: "2026-10-15T09:30:00Z",
  },
];

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/**
 * How long the audit log takes once a domain action is lifted: a slow server, so that a log shown from its cache
 * before the log comes again would show it without the lift (#475).
 */
const slowLogMs = 1500;

type DomainAction = Record<string, unknown> & { id: string; state: string };

/** The limit on the long domain that the log's domain_action_issued entry records, in effect. */
const domainLimit: DomainAction = { id: "0192f0c4-7a8e-7000-8000-0000000000a2", domain: longDomain, kind: "limit", obfuscate: false, state: "active", issued_at: "2026-10-16T09:30:00Z" };

/**
 * Plays the API of a signed-in operator: the audit log two entries per page. With `domains`, the operator also
 * manages domains: the limit on the long domain is listed, and lifting it records the lift and makes the log slow.
 * Returns the audit log requests.
 */
async function operatorServer(page: Page, { domains = false }: { domains?: boolean } = {}) {
  const locale = test.info().project.use.locale ?? "en";
  const requests: string[] = [];
  let log = entries;
  let limit = domainLimit;
  let slowLog = false;
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow admin" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api\/v1/, "");
    if (request.headers()["authorization"] !== "Bearer e2e-token" && path !== "/instance") return json(route, { type: "about:blank", title: "Unauthorized", status: 401 }, 401);
    const key = `${request.method()} ${path}`;
    if (key === "GET /instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, rules: [] });
    }
    if (key === "GET /accounts/verify_credentials") return json(route, domains ? { ...operator, permissions: ["view_audit", "manage_domains"] } : operator);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /admin/audit_log") {
      requests.push(url.search);
      if (slowLog) await new Promise((resolve) => setTimeout(resolve, slowLogMs));
      const actor = url.searchParams.get("actor_account_id");
      const action = url.searchParams.get("action");
      const matching = log.filter((entry) => (actor === null || entry.actor_account_id === actor) && (action === null || entry.action === action));
      const cursor = url.searchParams.get("cursor");
      const start = cursor === null ? 0 : matching.findIndex((entry) => entry.id === cursor) + 1;
      const items = matching.slice(start, start + 2);
      const last = items.at(-1);
      return json(route, start + 2 < matching.length && last !== undefined ? { items, next_cursor: last.id } : { items });
    }
    if (domains && key === "GET /admin/domain_actions") return json(route, { items: url.searchParams.get("active") === "true" && limit.state !== "active" ? [] : [limit] });
    if (domains && key === `POST /admin/domain_actions/${domainLimit.id}/lift`) {
      limit = { ...limit, state: "lifted", lifted_at: "2026-10-17T10:00:00Z" };
      const lift: Entry = { ...entries[1], id: "0192f0c4-7a8e-7000-8000-0000000a0005", actor_account_id: operator.id, actor_account: operator, action: "domain_action_lifted", occurred_at: "2026-10-17T10:00:00Z" };
      log = [lift, ...log];
      slowLog = true;
      return json(route, limit);
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return requests;
}

/** The rows of the list of operations: the items with a heading. */
function rows(page: Page) {
  return page.getByRole("main").getByRole("listitem").filter({ has: page.getByRole("heading", { level: 3 }) });
}

test.describe("page /admin/audit-log", () => {
  test.beforeEach(async ({ page }) => {
    await operatorServer(page);
    await page.goto("/admin/audit-log");
    await expect(rows(page)).toHaveCount(2);
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });
});

test.describe("audit log", () => {
  test("lists operations, filters them by a row's actor and by kind in the URL, and loads more", async ({ page, makeAxeBuilder }) => {
    const requests = await operatorServer(page);
    await page.goto("/admin/audit-log");
    await expect(rows(page)).toHaveCount(2);

    await expect(rows(page).first()).toMatchAriaSnapshot(`
      - listitem:
        - heading [level=3]
        - term
        - definition: /${moderator.display_name}.*@${moderator.acct}/
        - term
        - definition:
          - link "${targetHandle}"
        - term
        - definition:
          - time
        - list:
          - listitem:
            - link /${moderator.display_name}/
          - listitem:
            - link /${targetHandle.replaceAll(".", "\\.")}/
    `);
    await expect(rows(page).first().getByRole("link", { name: targetHandle, exact: true })).toHaveAttribute("href", "/admin/accounts/0192f0c4-7a8e-7000-8000-000000000031");

    // Load more: the pages stay in the URL, and a reload shows them again.
    await page.getByRole("main").getByRole("button").last().click();
    await expect(rows(page)).toHaveCount(3);
    await expect(page).toHaveURL(/[?&]pages=2/);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    await page.reload();
    await expect(rows(page)).toHaveCount(3);

    // The row's link to its actor's operations, by keyboard: the row stays, and focus with it.
    const actorLink = rows(page).first().getByRole("link", { name: new RegExp(moderator.display_name) });
    await actorLink.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`[?&]actor=${moderator.id}`));
    await expect(page).not.toHaveURL(/pages=/);
    await expect(rows(page)).toHaveCount(2);
    await expect(rows(page).first().getByRole("listitem").first().getByRole("link")).toBeFocused();

    // The kind of operation, with the select-only combobox and the keyboard.
    const filters = page.getByRole("main").getByRole("region").first();
    await filters.getByRole("button").first().focus();
    await page.keyboard.press("Enter");
    const listbox = page.getByRole("listbox");
    await expect(listbox).toBeVisible();
    await listbox.getByRole("option", { name: message("admin.audit_log.action.domain_action_issued"), exact: true }).click();
    await expect(page).toHaveURL(/[?&]action=domain_action_issued/);
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page).first().getByRole("link").first()).toHaveAttribute("href", `/admin/domain-actions?filter=all&domain=${longDomain}`);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    expect(requests).toEqual([
      "?limit=40",
      "?limit=40&cursor=0192f0c4-7a8e-7000-8000-0000000a0003",
      "?limit=40",
      "?limit=40&cursor=0192f0c4-7a8e-7000-8000-0000000a0003",
      `?limit=40&actor_account_id=${moderator.id}`,
      `?limit=40&actor_account_id=${moderator.id}&action=domain_action_issued`,
    ]);
  });

  test("shows a lift made on the domain actions at once when going back, however slowly the log comes", async ({ page, makeAxeBuilder }) => {
    await operatorServer(page, { domains: true });
    await page.goto("/admin/audit-log");
    await expect(rows(page)).toHaveCount(2);

    // The domain action's row links to the domain's actions; lifting the limit there is recorded in the log.
    await rows(page).nth(1).getByRole("link").first().click();
    await expect(page).toHaveURL(/\/admin\/domain-actions\?/);
    const lift = page.getByRole("main").getByRole("button").filter({ hasText: new RegExp(`^${message("admin.domain_actions.lift").replace(/[\\^$.*+?()[\]{}|]/g, "\\$&")}$`) });
    await expect(lift).toHaveCount(1);
    await lift.click();
    await expect(lift).toHaveCount(0);

    // Back on the log, the lift heads it as soon as the operations show, though the server answers the log
    // slowly now: the log is not shown from a cache older than the lift.
    await page.goBack();
    await expect(page).toHaveURL(/\/admin\/audit-log$/);
    await expect(rows(page)).toHaveCount(2);
    expect(await rows(page).first().getByRole("heading", { level: 3 }).textContent()).toBe(message("admin.audit_log.action.domain_action_lifted"));
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
