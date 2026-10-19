// SPDX-License-Identifier: AGPL-3.0-only
// Going offline and back in every width × locale project (web-client.md G17, ADR-0027): the app's one offline bar
// comes and goes as a polite live region, a page opened offline says why it cannot load instead of loading
// forever, and back online it loads by itself. `vite preview` has no API, so the spec answers GET /instance and
// the timelines with `page.route`. `context.setOffline` takes the network away; a route still answers then, so the
// spec's routes fail their requests while offline, as the network would.
// Texts come from the project's catalog, so the spec finds the same elements in every locale.
import { readFileSync } from "node:fs";
import type { BrowserContext, Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, openFromNavigation as openFromNavigationItem, test } from "./fixtures";

type Instance = components["schemas"]["Instance"];
type Status = components["schemas"]["Status"];
type StatusPage = components["schemas"]["StatusPage"];
type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale. */
function message(key: string): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text;
}

const bob: Status["account"] = {
  id: "0192f0c4-7a8e-7000-8000-000000000002",
  username: "bob",
  acct: "bob",
  domain: "my.example",
  uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000002",
  display_name: "Bob",
  note: "",
  fields: [],
  url: "https://my.example/@bob",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

const post: Status = {
  id: "0192f0c4-7a8e-7000-8000-0000000000c1",
  uri: "https://my.example/ap/statuses/0192f0c4-7a8e-7000-8000-0000000000c1",
  url: "https://my.example/@bob/0192f0c4-7a8e-7000-8000-0000000000c1",
  account: bob,
  content: "<p>A post to read while online.</p>",
  spoiler_text: "",
  sensitive: false,
  visibility: "public",
  language: "en",
  created_at: new Date(Date.now() - 7 * 60_000).toISOString(),
  media_attachments: [],
  mentions: [],
  tags: [],
  replies_count: 0,
  boosts_count: 0,
  favourites_count: 0,
};

const instance: Instance = {
  domain: "my.example",
  name: "my example",
  version: "0.0.0-e2e",
  source_url: "https://github.com/best-friends/my",
  registrations: { mode: "open" },
  limits: { max_post_characters: 500 },
  timelines: { local: true, federated: true },
  sends_email: false,
  rules: [],
};

/** Whether the spec has taken the network away. */
let offline = false;

async function setOffline(context: BrowserContext, value: boolean): Promise<void> {
  offline = value;
  await context.setOffline(value);
}

/** Answers the API; the federated timeline fails the first time, so that its page has nothing cached to show. */
async function serveApi(page: Page): Promise<void> {
  const json = (route: Route, body: unknown, status = 200) =>
    offline
      ? route.abort("internetdisconnected")
      : route.fulfill({ status, contentType: status < 400 ? "application/json" : "application/problem+json", body: JSON.stringify(body) });
  let publicRequests = 0;
  await page.route("**/api/v1/instance", (route) => json(route, instance));
  await page.route("**/api/v1/timelines/local*", (route) => json(route, { items: [post] } satisfies StatusPage));
  await page.route("**/api/v1/timelines/public*", (route) => {
    if (!offline) publicRequests++;
    return publicRequests === 1
      ? json(route, { type: "about:blank", title: "Service Unavailable", status: 503 }, 503)
      : json(route, { items: [post] } satisfies StatusPage);
  });
}

/** Opens a page from the navigation, as the user would. */
async function openFromNavigation(page: Page, path: string, hasTouch: boolean): Promise<void> {
  await openFromNavigationItem(page, path, hasTouch);
  await expect(page).toHaveURL(new RegExp(`${path}$`));
}

/** The offline bar: the status region with its message (the announcer's region is another one). */
function offlineBar(page: Page) {
  return page.getByRole("status").filter({ hasText: message("shell.offline") });
}

test.beforeEach(async ({ page, hasTouch }) => {
  offline = false;
  await serveApi(page);
  // Both pages' code is loaded while online, as a Service Worker would serve it offline (e2e/pwa.spec.ts); the
  // federated timeline failed, so it has no posts cached.
  await page.goto("/public");
  await expect(page.getByRole("main").getByRole("button")).toBeVisible();
  await openFromNavigation(page, "/local", hasTouch);
  await expect(page.getByRole("feed").getByRole("article")).toHaveCount(1);
});

test("shows the offline bar while offline and announces it", async ({ page, context, makeAxeBuilder }) => {
  // The live region is in the page, empty, before anything is to be announced.
  const region = page.locator('[role="status"]:not([data-live-announcer])').first();
  await expect(region).toBeAttached();
  await expect(region).toBeEmpty();

  await setOffline(context, true);
  await expect(offlineBar(page)).toBeVisible();
  // A polite live region of the message alone, so a screen reader reads it once it appears.
  await expect(offlineBar(page)).toMatchAriaSnapshot(`
    - status:
      - paragraph: ${JSON.stringify(message("shell.offline"))}
  `);
  // The page read before stays.
  await expect(page.getByRole("feed").getByRole("article")).toHaveCount(1);
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);

  await setOffline(context, false);
  await expect(offlineBar(page)).toHaveCount(0);
  await expect(region).toBeEmpty();
});

test("a page opened offline says why it cannot load, and loads back online", async ({ page, context, hasTouch }) => {
  await setOffline(context, true);
  await expect(offlineBar(page)).toBeVisible();

  await openFromNavigation(page, "/public", hasTouch);
  await expect(page.getByRole("main").getByText(message("api.error.offline"))).toBeVisible();
  await expect(page.getByRole("main").getByRole("progressbar")).toHaveCount(0);

  await setOffline(context, false);
  await expect(offlineBar(page)).toHaveCount(0);
  await expect(page.getByRole("feed").getByRole("article")).toHaveCount(1);
});
