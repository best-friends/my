// SPDX-License-Identifier: AGPL-3.0-only
// The account search and an account's admin page in every width × locale project (ADR-0027, FR-714, FR-703,
// FR-713): a local account with a long name and a remote one on a long domain, showing the email and IP
// addresses only when asked, lifting an action after its alert dialog, and the kinds of action a remote account
// takes, each passing axe without scrolling sideways. `vite preview` has no API, so each test plays the server of
// a signed-in moderator with page.route; names change with the locale, so the structure is checked by role and by
// what the server sends (names, handles, addresses).
import type { Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Schemas = components["schemas"];

/** The signed-in moderator's account, as GET /accounts/verify_credentials answers, with the permissions of the role. */
const moderator = {
  id: "0192f0c4-7a8e-7000-8000-000000000001",
  username: "alice",
  acct: "alice",
  domain: "my.example",
  uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000001",
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
  permissions: ["manage_reports", "manage_accounts", "view_audit"],
};

/** A local account with a long display name. */
const local: Schemas["Account"] = {
  ...moderator,
  id: "0192f0c4-7a8e-7000-8000-000000000021",
  username: "maximiliana",
  acct: "maximiliana",
  uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000021",
  display_name: "Maximiliana Theodora Wolfharte",
  url: "https://my.example/@maximiliana",
};

/** An account of a server on a long domain. */
const longDomain = "the-longest-community-server-name-in-the-fediverse.social.example.com";
const remote: Schemas["Account"] = {
  ...local,
  id: "0192f0c4-7a8e-7000-8000-000000000031",
  username: "carol",
  acct: `carol@${longDomain}`,
  domain: longDomain,
  uri: `https://${longDomain}/users/carol`,
  display_name: "Carol",
  url: `https://${longDomain}/@carol`,
};

const localAccount: Schemas["AdminAccount"] = {
  id: local.id,
  account: local,
  local: true,
  state: "limited",
  restrictions: ["visibility_ceiling_private", "public_discovery"],
  registration_state: "approved",
};
const remoteAccount: Schemas["AdminAccount"] = { id: remote.id, account: remote, local: false, state: "active", restrictions: [] };

const limit: Schemas["AdminAccountAction"] = {
  id: "0192f0c4-7a8e-7000-8000-0000000d0001",
  account_id: local.id,
  kind: "limit",
  reason_category: "spam",
  public_reason: "Posts the same advertisement in reply to everyone who mentions the server.",
  issuer: "moderator",
  issued_by_account_id: moderator.id,
  notify_target: true,
  state: "active",
  issued_at: "2026-10-10T08:00:00Z",
};

const userAgent = "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro Build/AP2A.240805.005) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Mobile Safari/537.36";
const privateData: Schemas["AdminAccountPrivateData"] = {
  email: "maximiliana.theodora.wolfharte@a-very-long-mail-provider.example.com",
  email_verified: true,
  registration_ip: "2001:db8:85a3:8d3:1319:8a2e:370:7348",
  registered_at: "2026-10-01T00:00:00Z",
  sign_ins: [{ ip: "192.0.2.7", user_agent: userAgent, signed_in_at: "2026-10-16T08:00:00Z" }],
  same_ip_accounts: [{ ip: "192.0.2.7", accounts: [remoteAccount] }],
};

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/**
 * How long the search takes once an action is lifted: a slow server, so that a search shown from its cache
 * before the search comes again would show the account as it was (#471).
 */
const slowSearchMs = 1500;

/** Plays the API of a signed-in moderator; returns the requests to the account operations. */
async function moderatorServer(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  let actions = [limit];
  let slowSearch = false;
  const requests: string[] = [];
  /** The account as it is now: limited while its action is in effect. */
  const current = (account: Schemas["AdminAccount"]): Schemas["AdminAccount"] => ({
    ...account,
    state: account.local && actions.some((action) => action.state === "active") ? "limited" : "active",
    restrictions: account.local && actions.some((action) => action.state === "active") ? account.restrictions : [],
  });
  const detail = (account: Schemas["AdminAccount"]): Schemas["AdminAccountDetail"] => ({
    ...current(account),
    ...(account.local ? { registration: { submitted_at: "2026-10-01T00:00:00Z", state_changed_at: "2026-10-01T01:00:00Z" }, email_verified: true } : {}),
    roles: [],
    actions: [],
    reports: [],
  });
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow admin" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api\/v1/, "");
    if (request.headers()["authorization"] !== "Bearer e2e-token" && path !== "/instance") return json(route, { type: "about:blank", title: "Unauthorized", status: 401 }, 401);
    const key = `${request.method()} ${path}`;
    if (path.startsWith("/admin/account")) requests.push(key);
    if (key === "GET /instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, rules: [] });
    }
    if (key === "GET /accounts/verify_credentials" || key === `GET /accounts/${moderator.id}`) return json(route, moderator);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /admin/counts") return json(route, {});
    if (key === "GET /admin/accounts") {
      if (slowSearch) await new Promise((resolve) => setTimeout(resolve, slowSearchMs));
      return json(route, { items: [current(localAccount), current(remoteAccount)] });
    }
    if (key === `GET /admin/accounts/${local.id}`) return json(route, detail(localAccount));
    if (key === `GET /admin/accounts/${remote.id}`) return json(route, detail(remoteAccount));
    if (key === `GET /admin/accounts/${local.id}/actions`) return json(route, { items: actions });
    if (key === `GET /admin/accounts/${remote.id}/actions`) return json(route, { items: [] });
    if (key === `POST /admin/accounts/${local.id}/reveal`) return json(route, privateData);
    if (key === `POST /admin/account_actions/${limit.id}/lift`) {
      const lifted: Schemas["AdminAccountAction"] = { ...limit, state: "lifted", lifted_at: "2026-10-17T00:00:00Z" };
      actions = [lifted];
      slowSearch = true;
      return json(route, lifted);
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return requests;
}

/** The rows of the results: the items with a heading. */
function rows(page: Page) {
  return page.getByRole("main").getByRole("region").last().getByRole("listitem").filter({ has: page.getByRole("heading", { level: 3 }) });
}

test.describe("page /moderation/accounts", () => {
  test.beforeEach(async ({ page }) => {
    await moderatorServer(page);
    await page.goto("/moderation/accounts");
    await expect(rows(page)).toHaveCount(2);
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });
});

test.describe("page /moderation/accounts/{id}", () => {
  test.beforeEach(async ({ page }) => {
    await moderatorServer(page);
    await page.goto(`/moderation/accounts/${local.id}`);
    await expect(page.getByRole("main").getByRole("heading", { level: 3 }).first()).toBeVisible();
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });
});

test.describe("accounts", () => {
  test("opens an account from the search, shows its email and IP addresses only when asked, and lifts an action after its confirmation", async ({ page, makeAxeBuilder }) => {
    const requests = await moderatorServer(page);
    await page.goto("/moderation/accounts");
    await expect(rows(page)).toHaveCount(2);
    const limitedRow = await rows(page).first().textContent();

    await expect(rows(page).first()).toMatchAriaSnapshot(`
      - listitem:
        - heading /Maximiliana Theodora Wolfharte/ [level=3]:
          - link /Maximiliana Theodora Wolfharte.*@maximiliana/
        - term
        - definition
        - term
        - definition
        - term
        - definition
        - term
        - definition:
          - time
        - term
        - definition
    `);
    await expect(rows(page).last().getByText(`@carol@${longDomain}`)).toBeVisible();

    // By keyboard: the row's link opens the account's page, whose heading takes focus.
    await rows(page).first().getByRole("link").focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/moderation/accounts/${local.id}$`));
    const main = page.getByRole("main");
    await expect(main.getByRole("heading", { level: 1 })).toBeFocused();
    await expect(main.getByRole("heading", { level: 3 }).first()).toBeVisible();
    expect(requests.filter((request) => request.endsWith("/reveal"))).toEqual([]);
    await expect(page.getByText(privateData.email ?? "")).toHaveCount(0);

    // The email and IP addresses, by keyboard: the region's heading takes focus once they show.
    const privateRegion = main.getByRole("region").nth(2);
    await privateRegion.getByRole("button").focus();
    await page.keyboard.press("Enter");
    await expect(privateRegion.getByText(privateData.email ?? "")).toBeVisible();
    await expect(privateRegion.getByRole("heading", { level: 2 })).toBeFocused();
    await expect(privateRegion.getByText(userAgent)).toBeVisible();
    await expect(privateRegion.getByRole("link", { name: /Carol/ })).toHaveAttribute("href", `/moderation/accounts/${remote.id}`);
    expect(requests.filter((request) => request.endsWith("/reveal"))).toEqual([`POST /admin/accounts/${local.id}/reveal`]);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    // The lift of the limit asks first, with the reason's field focused.
    const history = main.getByRole("region").nth(3);
    await history.getByRole("button").click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("textbox")).toBeFocused();
    await expect(dialog).toMatchAriaSnapshot(`
      - alertdialog /@maximiliana/:
        - heading /@maximiliana/ [level=2]
        - paragraph
        - textbox
        - button
        - button
    `);
    // The dialog fades in over the page, and out: axe measures contrast once it has settled.
    await expect(page.locator("[data-entering]")).toHaveCount(0);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    await dialog.locator('button[type="submit"]').click();
    await expect(dialog).toBeHidden();
    await expect(history.getByRole("heading", { level: 3 })).toBeFocused();
    await expect(history.getByRole("status").filter({ hasText: /\S/ })).toBeVisible();
    await expect(history.getByRole("button")).toHaveCount(0);
    expect(requests).toContain(`POST /admin/account_actions/${limit.id}/lift`);
    await expect(page.locator("[data-exiting]")).toHaveCount(0);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    // Back on the search, the account is no longer limited as soon as the results show, though the server
    // answers the search slowly now: the results are not shown from a cache older than the lift.
    await page.locator('main a[href="/moderation/accounts"]').click();
    await expect(page).toHaveURL(/\/moderation\/accounts$/);
    await expect(rows(page)).toHaveCount(2);
    expect(await rows(page).first().textContent()).not.toBe(limitedRow);
  });

  test("offers a remote account only the kinds of action it takes, and no email or IP addresses", async ({ page, makeAxeBuilder }) => {
    await moderatorServer(page);
    await page.goto(`/moderation/accounts/${remote.id}`);
    const main = page.getByRole("main");
    const form = main.locator("form");
    await expect(form.locator('input[type="radio"]')).toHaveCount(3);
    await expect(form.locator('input[type="radio"][value="force_sensitive"]')).toHaveCount(1);
    await expect(form.locator('input[type="radio"][value="limit"]')).toHaveCount(1);
    await expect(form.locator('input[type="radio"][value="suspend"]')).toHaveCount(1);
    await expect(form.locator('input[type="checkbox"]')).toHaveCount(0);
    // Account, restrictions, actions, reports, action: no region of email and IP addresses.
    await expect(main.getByRole("region")).toHaveCount(5);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
