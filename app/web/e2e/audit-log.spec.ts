// SPDX-License-Identifier: AGPL-3.0-only
// The audit log screen in every width × locale project (ADR-0027, FR-704): operations by a moderator with a name in
// right-to-left letters on an account and a long domain, filtered by a row's actor and by the kind of operation, and
// loading more, each passing axe without scrolling sideways, with the filters and pages in the URL. `vite preview`
// has no API, so each test plays the server of a signed-in operator with page.route; names change with the
// locale, so the structure is checked by role and by what the server sends (names, handles, domains).
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

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
    occurred_at: "2026-10-16T09:30:00Z",
  },
  {
    id: "0192f0c4-7a8e-7000-8000-0000000a0002",
    actor_kind: "cli",
    action: "setting_changed",
    target_kind: "setting",
    target_id: "registration_mode",
    target_label: "registration_mode",
    occurred_at: "2026-10-15T09:30:00Z",
  },
];

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/** Plays the API of a signed-in operator: the audit log two entries per page. Returns the audit log requests. */
async function operatorServer(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  const requests: string[] = [];
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
    if (key === "GET /accounts/verify_credentials") return json(route, operator);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /admin/audit_log") {
      requests.push(url.search);
      const actor = url.searchParams.get("actor_account_id");
      const action = url.searchParams.get("action");
      const matching = entries.filter((entry) => (actor === null || entry.actor_account_id === actor) && (action === null || entry.action === action));
      const cursor = url.searchParams.get("cursor");
      const start = cursor === null ? 0 : matching.findIndex((entry) => entry.id === cursor) + 1;
      const items = matching.slice(start, start + 2);
      const last = items.at(-1);
      return json(route, start + 2 < matching.length && last !== undefined ? { items, next_cursor: last.id } : { items });
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
    await listbox.getByRole("option").nth(3).click();
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
});
