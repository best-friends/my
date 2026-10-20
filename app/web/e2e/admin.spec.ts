// SPDX-License-Identifier: AGPL-3.0-only
// The admin screens in every width × locale project (ADR-0027): the remote servers and their history, resuming
// deliveries, and running a failed job again, each passing axe without scrolling sideways. `vite preview` has
// no API, so each test plays the server of a signed-in operator with page.route; names change with the
// locale, so the structure is checked by role, the names in the project's locale catalog and the identifiers the
// server sends (domains, job types).
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

/** The signed-in operator's account, as GET /accounts/verify_credentials answers, with both admin permissions. */
const operator = {
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
  permissions: ["manage_domains", "manage_jobs"],
};

// A long domain and a long payload, to show that they wrap at 320 px.
const longDomain = "a-very-long-subdomain-of-a-remote-server-for-testing.social.example";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/** Plays the admin API of a signed-in operator; returns the jobs run again and the servers marked available. */
async function operatorServer(page: Page) {
  const locale = test.info().project.use.locale ?? "en";
  let paused: Record<string, unknown> = { domain: longDomain, state: "unavailable", since: "2026-10-10T08:00:00Z", failure_days: 7, last_failure_day: "2026-10-10" };
  let events = [{ id: "0192f0c4-7a8e-7000-8000-0000000000e1", kind: "marked_unavailable", reason: "failure_days", occurred_at: "2026-10-10T08:00:00Z" }];
  let dead = [
    {
      id: "0192f0c4-7a8e-7000-8000-0000000000d1",
      type: "federation.deliver",
      payload: JSON.stringify({ delivery_id: "0192f0c4-7a8e-7000-8000-0000000000aa", inbox: `https://${longDomain}/users/someone-with-a-long-name/inbox` }),
      idempotency_key: "deliver:0192f0c4-7a8e-7000-8000-0000000000aa",
      attempts: 9,
      last_error: "502 Bad Gateway",
      created_at: "2026-10-13T10:00:00Z",
      failed_at: "2026-10-17T10:00:00Z",
    },
    { id: "0192f0c4-7a8e-7000-8000-0000000000d0", type: "kv.cleanup", payload: "null", attempts: 3, last_error: "db down", created_at: "2026-10-16T10:00:00Z", failed_at: "2026-10-16T11:00:00Z" },
  ];
  const actions: string[] = [];
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow admin" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace(/^\/api\/v1/, "");
    if (request.headers()["authorization"] !== "Bearer e2e-token" && path !== "/instance") return json(route, { type: "about:blank", title: "Unauthorized", status: 401 }, 401);
    const key = `${request.method()} ${path}`;
    if (key === "GET /instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, rules: [] });
    }
    if (key === "GET /accounts/verify_credentials") return json(route, operator);
    if (key === "GET /preferences") {
      return json(route, { locale, time_zone: "UTC", default_visibility: "public", default_post_language: "und", require_media_description: false, timeline_languages: [] });
    }
    if (key === "GET /admin/instances") {
      return json(route, { items: [paused, { domain: "flaky.example", state: "available", failure_days: 2, last_failure_day: "2026-10-16" }] });
    }
    if (key === `GET /admin/instances/${longDomain}`) return json(route, paused);
    if (key === `GET /admin/instances/${longDomain}/availability_events`) return json(route, { items: events });
    if (key === `POST /admin/instances/${longDomain}/mark_available`) {
      actions.push(key);
      paused = { domain: longDomain, state: "available", since: "2026-10-17T00:00:00Z", failure_days: 0 };
      events = [{ id: "0192f0c4-7a8e-7000-8000-0000000000e2", kind: "marked_available", reason: "operator", occurred_at: "2026-10-17T00:00:00Z" }, ...events];
      return json(route, paused);
    }
    if (key === "GET /admin/dead_jobs") return json(route, { items: dead });
    const retry = /^POST \/admin\/dead_jobs\/([^/]+)\/retry$/.exec(key);
    if (retry) {
      actions.push(key);
      dead = dead.filter((job) => job.id !== retry[1]);
      return route.fulfill({ status: 204, headers: { "Cache-Control": "no-store" } });
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return actions;
}

const pages = ["/settings", "/moderation/instances", `/moderation/instances/${longDomain}`, "/admin/dead-jobs"] as const;

for (const path of pages) {
  test.describe(`page ${path}`, () => {
    test.beforeEach(async ({ page }) => {
      await operatorServer(page);
      await page.goto(path);
      await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
      await expect(page.getByRole("main").getByRole("heading", { level: 2 }).first()).toBeVisible();
    });

    test("has no axe violations", async ({ makeAxeBuilder }) => {
      await expectNoAxeViolations(makeAxeBuilder);
    });

    test("does not scroll horizontally", async ({ page }) => {
      await expectNoHorizontalScroll(page);
    });
  });
}

test.describe("remote servers", () => {
  test("lists the servers, opens one with its history, and resumes deliveries to it", async ({ page, makeAxeBuilder }) => {
    const actions = await operatorServer(page);
    await page.goto("/moderation/instances");
    const main = page.getByRole("main");

    await expect(main).toMatchAriaSnapshot(`
      - heading [level=1]
      - link
      - tablist:
        - tab [selected]
        - tab
        - tab
      - tabpanel:
        - list:
          - listitem:
            - heading "${longDomain}" [level=2]:
              - link "${longDomain}"
            - term
            - definition
            - term
            - definition:
              - time
            - term
            - definition
            - term
            - definition:
              - time
          - listitem:
            - heading "flaky.example" [level=2]:
              - link "flaky.example"
            - term
            - definition
            - term
            - definition
            - term
            - definition
            - term
            - definition:
              - time
    `);

    await main.getByRole("link", { name: longDomain }).click();
    await expect(main.getByRole("heading", { level: 1, name: longDomain })).toBeFocused();
    const resume = main.getByRole("button", { name: message("admin.instance.mark_available"), exact: true });
    await resume.click();
    await expect(resume).toBeHidden();
    await expect(main.getByRole("region", { name: message("admin.instance.history.title"), exact: true }).getByRole("listitem")).toHaveCount(2);
    expect(actions).toEqual([`POST /admin/instances/${longDomain}/mark_available`]);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

test.describe("failed jobs", () => {
  test("runs a job again: it leaves the list and focus moves to the next job", async ({ page, makeAxeBuilder }) => {
    const actions = await operatorServer(page);
    await page.goto("/admin/dead-jobs");
    const main = page.getByRole("main");
    await expect(main.getByRole("heading", { level: 2, name: "federation.deliver" })).toBeVisible();

    await expect(main.getByRole("list").first()).toMatchAriaSnapshot(`
      - list:
        - listitem:
          - heading "federation.deliver" [level=2]
          - button
          - term
          - definition:
            - time
          - term
          - definition:
            - time
          - term
          - definition
          - term
          - definition:
            - code
          - term
          - definition: 502 Bad Gateway
          - term
          - definition:
            - code
        - listitem:
          - heading "kv.cleanup" [level=2]
          - button
          - term
          - definition:
            - time
          - term
          - definition:
            - time
          - term
          - definition
          - term
          - definition: db down
          - term
          - definition:
            - code: "null"
    `);

    await main.getByRole("button", { name: /federation\.deliver/ }).click();
    await expect(main.getByRole("heading", { level: 2, name: "federation.deliver" })).toBeHidden();
    await expect(main.getByRole("button", { name: /kv\.cleanup/ })).toBeFocused();
    expect(actions).toEqual(["POST /admin/dead_jobs/0192f0c4-7a8e-7000-8000-0000000000d1/retry"]);
    await expectNoAxeViolations(makeAxeBuilder);
  });
});
