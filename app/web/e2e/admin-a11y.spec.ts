// SPDX-License-Identifier: AGPL-3.0-only
// The accessibility regression suite of the staff's screens (ADR-0015, ADR-0027, ADR-0058, web-client.md §14):
// every moderation and server administration screen passes axe without scrolling sideways in every width × locale
// project (the nightly E2E_MATRIX=full runs them all), the list of screens follows the moderation and admin
// routes, and a moderator handles a report with the keyboard alone — the moderation page, the queue, the report,
// the target's history on its moderation page, back to the report, an action that resolves it, and back to the
// queue — with focus on the new page's h1 after every navigation. `vite preview`
// has no API, so each test plays the server of a signed-in moderator with every admin permission with
// page.route; names change with the locale, so the structure is checked by role and by what the server sends.
import { readdirSync } from "node:fs";
import type { Locator, Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Schemas = components["schemas"];

/** The signed-in moderator's account, as GET /accounts/verify_credentials answers, with every admin permission. */
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
  permissions: ["manage_reports", "manage_accounts", "view_audit", "manage_domains", "manage_jobs", "manage_settings"],
};

/** The reported account: a long display name, local. */
const target: Schemas["Account"] = {
  ...moderator,
  id: "0192f0c4-7a8e-7000-8000-000000000021",
  username: "maximiliana",
  acct: "maximiliana",
  uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000021",
  display_name: "Maximiliana Theodora Wolfharte",
  url: "https://my.example/@maximiliana",
};

/** The reporter: the instance actor of a server on a long domain. */
const longDomain = "the-longest-community-server-name-in-the-fediverse.social.example.com";
const reporter: Schemas["Account"] = {
  ...target,
  id: "0192f0c4-7a8e-7000-8000-000000000031",
  username: "instance.actor",
  acct: `instance.actor@${longDomain}`,
  domain: longDomain,
  uri: `https://${longDomain}/actor`,
  display_name: "",
  url: `https://${longDomain}/actor`,
};

const postId = "0192f0c4-7a8e-7000-8000-0000000000b1";
const post: Schemas["Status"] = {
  id: postId,
  uri: `https://my.example/ap/statuses/${postId}`,
  url: `https://my.example/@maximiliana/${postId}`,
  account: target,
  content: "<p>Buy cheap followers at https://spam.example/now — limited offer!</p>",
  spoiler_text: "",
  sensitive: false,
  visibility: "public",
  language: "en",
  created_at: "2026-10-16T08:00:00Z",
  media_attachments: [],
  mentions: [],
  tags: [],
  replies_count: 0,
  boosts_count: 0,
  favourites_count: 0,
};

const reportId = "0192f0c4-7a8e-7000-8000-0000000c0002";
/** Another open report, on the same target, that stays open. */
const otherReportId = "0192f0c4-7a8e-7000-8000-0000000c0001";

function reportOf(id: string, overrides: Partial<Schemas["AdminReportDetail"]> = {}): Schemas["AdminReportDetail"] {
  return {
    id,
    category: "spam",
    comment: "This account posts the same advertisement in reply to everyone who mentions the server, many times a day.",
    forwarded: false,
    filed_at: "2026-10-16T09:00:00Z",
    state: "open",
    state_changed_at: "2026-10-16T09:00:00Z",
    reporter_account_id: reporter.id,
    reporter_account: reporter,
    uri: `https://${longDomain}/flags/1`,
    target_account_id: target.id,
    target_account: target,
    status_ids: [postId],
    rule_ids: [],
    statuses: [{ status: post }],
    notes: [{ id: "0192f0c4-7a8e-7000-8000-0000000e0001", author_account_id: moderator.id, body: "Same as last week.", created_at: "2026-10-16T10:00:00Z" }],
    target_history: { reports_count: 2, actions_count: 1 },
    ...overrides,
  };
}

/** The target's earlier moderation action, in its history. */
const warning: Schemas["AdminAccountAction"] = {
  id: "0192f0c4-7a8e-7000-8000-0000000d0000",
  account_id: target.id,
  kind: "warn",
  reason_category: "spam",
  public_reason: "Posts the same advertisement in reply to everyone who mentions the server.",
  issuer: "moderator",
  issued_by_account_id: moderator.id,
  notify_target: true,
  state: "active",
  issued_at: "2026-10-09T00:00:00Z",
};

const appealId = "0192f0c4-7a8e-7000-8000-0000000f0001";
const appeal: Schemas["AdminAppeal"] = {
  id: appealId,
  account_id: target.id,
  text: `I only posted the link to my own blog: https://birds.example/${"very-long-path-".repeat(6)}end.`,
  state: "pending",
  submitted_at: "2026-10-16T09:00:00Z",
  action: { ...warning, id: "0192f0c4-7a8e-7000-8000-0000000d0101", kind: "suspend", issued_at: "2026-10-15T09:00:00Z" },
};

const registration = {
  user_id: "0192f0c4-7a8e-7000-8000-0000000000b2",
  account_id: "0192f0c4-7a8e-7000-8000-0000000000c2",
  username: "bob",
  state: "pending",
  since: "2026-10-16T09:00:00Z",
  email: "bob.with.a.long.address@birds.example",
  email_verified: true,
  reason: `I would like to post about birds. My blog is at https://birds.example/${"very-long-path-".repeat(6)}end.`,
  submitted_at: "2026-10-16T09:00:00Z",
};

const domainAction = {
  id: "0192f0c4-7a8e-7000-8000-0000000000a2",
  domain: longDomain,
  kind: "limit",
  public_comment: "Spam from every account of this server, reported many times by people here, with no answer from its moderators.",
  private_note: "See the reports.",
  obfuscate: true,
  state: "active",
  issued_at: "2026-10-12T08:00:00Z",
  expires_at: "2026-11-11T08:00:00Z",
};

const signupBlock = {
  id: "0192f0c4-7a8e-7000-8000-0000000001a2",
  kind: "email_domain",
  value: "xn--r8jz45g.jp",
  severity: "reject",
  comment: "Disposable addresses used for many accounts that posted the same advertisement on the same day.",
  state: "active",
  created_at: "2026-10-12T08:00:00Z",
};

const auditEntry = {
  id: "0192f0c4-7a8e-7000-8000-0000000a0004",
  actor_kind: "user",
  actor_account_id: moderator.id,
  actor_account: moderator,
  action: "account_action_issued",
  target_kind: "account",
  target_id: target.id,
  target_label: `@${target.acct}`,
  occurred_at: "2026-10-17T09:30:00Z",
};

const instance = { domain: longDomain, state: "unavailable", since: "2026-10-10T08:00:00Z", failure_days: 7, last_failure_day: "2026-10-10" };

const deadJob = {
  id: "0192f0c4-7a8e-7000-8000-0000000000d1",
  type: "federation.deliver",
  payload: JSON.stringify({ delivery_id: "0192f0c4-7a8e-7000-8000-0000000000aa", inbox: `https://${longDomain}/users/someone-with-a-long-name/inbox` }),
  idempotency_key: "deliver:0192f0c4-7a8e-7000-8000-0000000000aa",
  attempts: 9,
  last_error: "502 Bad Gateway",
  created_at: "2026-10-13T10:00:00Z",
  failed_at: "2026-10-17T10:00:00Z",
};

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/**
 * How long the lists of reports take once an action has resolved one: a slow server, so that a queue shown from
 * its cache before the list comes again would show the resolved report (#464).
 */
const slowListMs = 1500;

/** Plays the API of a signed-in moderator with every admin permission; returns the account actions taken. */
async function moderatorServer(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  let reports = [reportOf(reportId), reportOf(otherReportId, { filed_at: "2026-10-15T09:00:00Z", comment: "" })];
  let actions = [warning];
  let slowLists = false;
  const issued: string[] = [];
  const adminAccount = (): Schemas["AdminAccountDetail"] => ({
    id: target.id,
    account: target,
    local: true,
    state: "active",
    restrictions: [],
    registration_state: "approved",
    registration: { submitted_at: "2026-10-01T00:00:00Z", state_changed_at: "2026-10-01T01:00:00Z" },
    email_verified: true,
    roles: [],
    actions,
    reports,
  });
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
    if (key === "GET /accounts/verify_credentials" || key === `GET /accounts/${moderator.id}`) return json(route, moderator);
    if (key === `GET /accounts/${target.id}`) return json(route, target);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /admin/counts") {
      return json(route, { open_reports: reports.filter((item) => item.state === "open").length, pending_registrations: 1, pending_appeals: 1 });
    }
    if (key === "GET /admin/reports") {
      if (slowLists) await new Promise((resolve) => setTimeout(resolve, slowListMs));
      const state = url.searchParams.get("state");
      return json(route, { items: reports.filter((item) => state === null || item.state === state) });
    }
    const report = /^GET \/admin\/reports\/([^/]+)$/.exec(key);
    if (report) {
      const found = reports.find((item) => item.id === report[1]);
      return found === undefined ? json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404) : json(route, found);
    }
    if (key === "GET /admin/accounts") return json(route, { items: [{ id: target.id, account: target, local: true, state: "active", restrictions: [], registration_state: "approved" }] });
    if (key === `GET /admin/accounts/${target.id}`) return json(route, adminAccount());
    if (key === `GET /admin/accounts/${target.id}/actions`) return json(route, { items: actions });
    if (key === `POST /admin/accounts/${target.id}/actions`) {
      const body = request.postDataJSON() as Schemas["CreateAccountActionRequest"];
      issued.push(`${body.kind} ${body.report_id ?? ""}`);
      const action: Schemas["AdminAccountAction"] = {
        id: "0192f0c4-7a8e-7000-8000-0000000d0001",
        account_id: target.id,
        kind: body.kind,
        reason_category: body.reason_category,
        issuer: "moderator",
        issued_by_account_id: moderator.id,
        notify_target: body.notify_target,
        state: "active",
        issued_at: "2026-10-17T00:00:00Z",
        ...(body.report_id === undefined ? {} : { report_id: body.report_id }),
      };
      actions = [action, ...actions];
      reports = reports.map((item) => (item.id === body.report_id ? { ...item, state: "resolved", outcome: "actioned", state_changed_at: "2026-10-17T00:00:00Z" } : item));
      slowLists = true;
      return json(route, action);
    }
    if (key === "GET /admin/appeals") {
      const state = url.searchParams.get("state");
      return json(route, { items: state === null || state === "pending" ? [appeal] : [] });
    }
    if (key === `GET /admin/appeals/${appealId}`) return json(route, appeal);
    if (key === "GET /admin/registrations") return json(route, { items: [registration] });
    if (key === "GET /admin/domain_actions") return json(route, { items: [domainAction] });
    if (key === "GET /admin/signup_blocks") return json(route, { items: [signupBlock] });
    if (key === "GET /admin/audit_log") return json(route, { items: [auditEntry] });
    if (key === "GET /admin/instances") return json(route, { items: [instance] });
    if (key === `GET /admin/instances/${longDomain}`) return json(route, instance);
    if (key === `GET /admin/instances/${longDomain}/availability_events`) {
      return json(route, { items: [{ id: "0192f0c4-7a8e-7000-8000-0000000000e1", kind: "marked_unavailable", reason: "failure_days", occurred_at: "2026-10-10T08:00:00Z" }] });
    }
    if (key === "GET /admin/dead_jobs") return json(route, { items: [deadJob] });
    if (key === "GET /admin/instance_texts") return json(route, { items: [documentText, ...instanceNames] });
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return issued;
}

/** The terms of service in English, as GET /admin/instance_texts lists them. */
const documentText: Schemas["AdminInstanceText"] = { key: "terms", locale: "en", body: "# Accounts\n\nBe kind.", revised_at: "2026-10-12T08:00:00Z" };
/** The server's names: in English, and in Arabic (a language without a catalog), which reads right to left. */
const instanceNames: Schemas["AdminInstanceText"][] = [
  { key: "name", locale: "ar", body: "مثال", revised_at: "2026-10-16T00:00:00Z" },
  { key: "name", locale: "en", body: "my example", revised_at: "2026-10-16T00:00:00Z" },
];

/**
 * Every staff screen, by the name of its route file in src/routes, with an address that opens it and a text of
 * the server's answer that shows once it has loaded (or, for a screen that shows it only as the value of a field,
 * that field's value). A new moderation or admin route fails the test below until it is listed.
 */
const adminPages: Readonly<Record<string, { path: string; ready: string | { value: string } }>> = {
  "moderation.index": { path: "/moderation", ready: "" },
  "admin.index": { path: "/admin", ready: "" },
  "moderation.reports": { path: "/moderation/reports", ready: `@${target.acct}` },
  "moderation.reports_.$id": { path: `/moderation/reports/${reportId}`, ready: "Same as last week." },
  "moderation.accounts": { path: "/moderation/accounts", ready: target.display_name },
  "moderation.accounts_.$id": { path: `/moderation/accounts/${target.id}`, ready: target.display_name },
  "moderation.appeals": { path: "/moderation/appeals", ready: target.display_name },
  "moderation.appeals_.$id": { path: `/moderation/appeals/${appealId}`, ready: target.display_name },
  "moderation.registrations": { path: "/moderation/registrations", ready: registration.email },
  "moderation.domain-actions": { path: "/moderation/domain-actions", ready: domainAction.public_comment },
  "moderation.signup-blocks": { path: "/moderation/signup-blocks", ready: signupBlock.comment },
  "moderation.audit-log": { path: "/moderation/audit-log", ready: auditEntry.target_label },
  "moderation.instances": { path: "/moderation/instances", ready: longDomain },
  "moderation.instances_.$domain": { path: `/moderation/instances/${longDomain}`, ready: longDomain },
  "admin.dead-jobs": { path: "/admin/dead-jobs", ready: deadJob.type },
  "admin.documents": { path: "/admin/documents?locale=en", ready: { value: documentText.body } },
  "admin.settings": { path: "/admin/settings", ready: "my.example" },
};

test("the suite lists every moderation and admin route", () => {
  const routes = readdirSync(new URL("../src/routes", import.meta.url))
    .filter((name) => /^(moderation|admin)\..+\.tsx$/.test(name) && !name.endsWith(".test.tsx"))
    .map((name) => name.replace(/\.tsx$/, ""));
  expect(Object.keys(adminPages).toSorted()).toEqual(routes.toSorted());
});

for (const [route, { path, ready }] of Object.entries(adminPages)) {
  test.describe(`staff page ${path} (${route})`, () => {
    test.beforeEach(async ({ page }) => {
      await moderatorServer(page);
      await page.goto(path);
      const main = page.getByRole("main");
      await expect(main.getByRole("heading", { level: 1 })).toBeVisible();
      // The pages of the two areas list links only; the other screens show what the server sent.
      if (typeof ready !== "string") await expect(main.getByRole("textbox").first()).toHaveValue(ready.value);
      else if (ready === "") await expect(main.getByRole("link").first()).toBeVisible();
      else await expect(main.getByText(ready).first()).toBeVisible();
    });

    test("has no axe violations", async ({ makeAxeBuilder }) => {
      await expectNoAxeViolations(makeAxeBuilder);
    });

    test("does not scroll horizontally", async ({ page }) => {
      await expectNoHorizontalScroll(page);
    });
  });
}

/** Whether `locator` or an element in it (a radio of a radio group) has focus. */
function hasFocus(locator: Locator): Promise<boolean> {
  return locator.evaluate((element) => element.contains(document.activeElement));
}

/**
 * Presses Tab (Shift+Tab with `backward`) until `locator` has focus, as a keyboard user moves through the page;
 * fails if it is not reached within 200 presses, which would mean it is out of the tab order.
 */
async function tabTo(page: Page, locator: Locator, { backward = false } = {}): Promise<void> {
  const limit = 200;
  await expect(locator).toBeVisible();
  for (let presses = 0; presses < limit; presses++) {
    if (await hasFocus(locator)) return;
    await page.keyboard.press(backward ? "Shift+Tab" : "Tab");
  }
  expect(await hasFocus(locator), `not reached with ${String(limit)} presses of ${backward ? "Shift+Tab" : "Tab"}`).toBe(true);
}

/** Asserts the new page's h1 has focus (web-client.md Y5), axe passes, and the page does not scroll sideways. */
async function expectPageReached(page: Page, makeAxeBuilder: Parameters<typeof expectNoAxeViolations>[0]): Promise<void> {
  await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeFocused();
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);
}

test.describe("handling a report with the keyboard alone", () => {
  test("opens a report, reads the target's history, takes an action that resolves it, and returns to the queue", async ({ page, makeAxeBuilder }) => {
    // Every step is many presses of Tab, each checked for focus.
    test.setTimeout(120_000);
    const issued = await moderatorServer(page);
    await page.goto("/moderation");
    const main = page.getByRole("main");
    await expect(main.getByRole("heading", { level: 1 })).toBeVisible();

    // The moderation page leads to the report queue.
    await tabTo(page, main.locator('a[href="/moderation/reports"]'));
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/moderation\/reports$/);
    const queue = main.getByRole("tabpanel");
    await expect(queue.locator(`a[href="/moderation/reports/${reportId}"]`)).toBeVisible();
    await expectPageReached(page, makeAxeBuilder);

    // The queue opens the report.
    await tabTo(page, queue.locator(`a[href="/moderation/reports/${reportId}"]`));
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/moderation/reports/${reportId}$`));
    await expect(main.getByRole("article")).toBeVisible();
    await expectPageReached(page, makeAxeBuilder);

    // The target's history: its action on the report's page, and its moderation page with its reports and actions.
    await expect(main.getByRole("region").filter({ hasText: warning.public_reason ?? "" })).toBeVisible();
    await tabTo(page, main.locator(`a[href="/moderation/accounts/${target.id}"]`));
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/moderation/accounts/${target.id}$`));
    await expect(main.getByText(warning.public_reason ?? "").first()).toBeVisible();
    await expectPageReached(page, makeAxeBuilder);

    // Back to the report from the account's reports.
    await tabTo(page, main.locator(`a[href="/moderation/reports/${reportId}"]`));
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/moderation/reports/${reportId}$`));
    await expect(main.getByRole("article")).toBeVisible();
    await expectPageReached(page, makeAxeBuilder);

    // An action, chosen with the arrow keys in its radio group, resolves the report (FR-710).
    const handling = main.getByRole("region").last();
    const limit = handling.locator('input[type="radio"][value="limit"]');
    await tabTo(page, handling.getByRole("radiogroup"));
    for (let presses = 0; presses < 20 && !(await limit.isChecked()); presses++) await page.keyboard.press("ArrowDown");
    await expect(limit).toBeChecked();
    await expect(limit).toBeFocused();
    await tabTo(page, handling.locator('form button[type="submit"]'));
    await page.keyboard.press("Enter");
    await expect(handling.getByRole("status").filter({ hasText: /\S/ })).toBeVisible();
    await expect(handling.getByRole("heading", { level: 2 })).toBeFocused();
    expect(issued).toEqual([`limit ${reportId}`]);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    // Back on the queue, the report is gone from the open ones as soon as the queue shows, though the server
    // answers the lists slowly now: the queue is not shown from a cache older than the action.
    await tabTo(page, main.locator('a[href="/moderation/reports"]'), { backward: true });
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/moderation\/reports$/);
    await expect(queue.locator(`a[href="/moderation/reports/${otherReportId}"]`)).toBeVisible();
    expect(await queue.locator(`a[href="/moderation/reports/${reportId}"]`).count()).toBe(0);
    await expect(queue.locator('a[href^="/moderation/reports/"]')).toHaveCount(1);
    await expect(queue.locator(`a[href="/moderation/reports/${reportId}"]`)).toHaveCount(0);
    await expectPageReached(page, makeAxeBuilder);
  });
});
