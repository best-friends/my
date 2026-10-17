// SPDX-License-Identifier: AGPL-3.0-only
// The federated timeline in every width × locale project (FR-202, ADR-0027): its navigation item, which
// the instance setting federated_timeline_enabled takes away, and its feed of posts of this server and of
// another one. `vite preview` has no client API, so the spec answers GET /instance and the timeline with
// `page.route`, shaped like api/openapi.yaml. Names change with the locale, so the navigation item is
// found by its address and the feed by the page heading.
import type { Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Account = components["schemas"]["Account"];
type Instance = components["schemas"]["Instance"];
type Status = components["schemas"]["Status"];
type StatusPage = components["schemas"]["StatusPage"];

function account(username: string, displayName: string, domain: string): Account {
  const local = domain === "my.example";
  return {
    id: `0192f0c4-7a8e-7000-8000-${local ? "000000000002" : "000000000003"}`,
    username,
    acct: local ? username : `${username}@${domain}`,
    domain,
    uri: `https://${domain}/users/${username}`,
    display_name: displayName,
    note: "",
    fields: [],
    url: `https://${domain}/@${username}`,
    locked: false,
    bot: false,
    created_at: "2026-10-01T00:00:00Z",
  };
}

const bob = account("bob", "Bob", "my.example");
/** A long host that has to fit a 320 px card. */
const carol = account("carol", "Carol", "a-rather-long-community-server-name.social.example.com");

function status(n: number, author: Account): Status {
  return {
    id: `0192f0c4-7a8e-7000-8000-${String(n).padStart(12, "0")}`,
    uri: `https://${author.domain}/statuses/${String(n)}`,
    url: `https://${author.domain}/@${author.username}/${String(n)}`,
    account: author,
    content: `<p>Post number ${String(n)} on the federated timeline.</p>`,
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: new Date(Date.now() - n * 7 * 60_000).toISOString(),
    media_attachments: [],
    mentions: [],
    tags: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
  };
}

function instance(federated: boolean): Instance {
  return {
    domain: "my.example",
    name: "my example",
    version: "0.0.0-e2e",
    source_url: "https://github.com/best-friends/my",
    registrations: { mode: "open" },
    limits: { max_post_characters: 500 },
    timelines: { local: true, federated },
    rules: [],
  };
}

/** Answers GET /instance and the federated timeline, which is a 404 while it is off. */
async function serveApi(page: Page, federated: boolean): Promise<void> {
  const json = (route: Route, body: unknown, statusCode = 200) =>
    route.fulfill({ status: statusCode, contentType: statusCode < 400 ? "application/json" : "application/problem+json", body: JSON.stringify(body) });
  await page.route("**/api/v1/instance", (route) => json(route, instance(federated)));
  await page.route("**/api/v1/timelines/public*", (route) =>
    federated
      ? json(route, { items: [status(1, bob), status(2, carol), status(3, carol)] } satisfies StatusPage)
      : json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404),
  );
}

/** The navigation item of the federated timeline. */
function federatedLink(page: Page) {
  return page.getByRole("navigation").locator('a[href="/public"]');
}

test.describe("the federated timeline while the server serves it", () => {
  test.beforeEach(async ({ page }) => {
    await serveApi(page, true);
    await page.goto("/home");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("opens from the navigation as a feed named by the page heading", async ({ page, hasTouch }) => {
    const link = federatedLink(page);
    await expect(link).toBeVisible();
    if (hasTouch) await link.tap();
    else await link.click();

    await expect(page).toHaveURL(/\/public$/);
    // Where focus goes after a navigation is the shell's, and e2e/shell.spec.ts checks it.
    const heading = page.getByRole("heading", { level: 1 });
    await expect(link).toHaveAttribute("aria-current", "page");
    const name = (await heading.textContent()) ?? "";
    const feed = page.getByRole("feed", { name });
    await expect(feed.getByRole("article")).toHaveCount(3);
    await expect(feed).toMatchAriaSnapshot(`
      - feed:
        - article:
          - link /Bob/
          - link /Bob/
          - link:
            - time
          - paragraph
        - article:
          - link /Carol/
          - link /Carol/
          - link:
            - time
          - paragraph
          - link /a-rather-long-community-server-name/
        - article:
          - link /Carol/
          - link /Carol/
          - link:
            - time
          - paragraph
          - link /a-rather-long-community-server-name/
    `);
  });

  test("has no axe violations and does not scroll horizontally", async ({ page, makeAxeBuilder }) => {
    await page.goto("/public");
    await expect(page.getByRole("feed").getByRole("article")).toHaveCount(3);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("fits all seven items of the navigation in its width", async ({ page }) => {
    const nav = page.getByRole("navigation");
    await expect(nav.getByRole("link")).toHaveCount(7);
    const navBox = await nav.boundingBox();
    expect(navBox).not.toBeNull();
    for (const link of await nav.getByRole("link").all()) {
      const box = await link.boundingBox();
      expect(box).not.toBeNull();
      if (box === null || navBox === null) continue;
      expect(box.x).toBeGreaterThanOrEqual(navBox.x - 0.5);
      expect(box.x + box.width).toBeLessThanOrEqual(navBox.x + navBox.width + 0.5);
    }
    await expectNoHorizontalScroll(page);
  });
});

test.describe("the federated timeline while the server has turned it off", () => {
  test.beforeEach(async ({ page }) => {
    await serveApi(page, false);
  });

  test("is not in the navigation", async ({ page }) => {
    const answered = page.waitForResponse("**/api/v1/instance");
    await page.goto("/home");
    await answered;
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    // The local timeline is there; the federated one never appears.
    await expect(page.getByRole("navigation").locator('a[href="/local"]')).toBeVisible();
    await expect(federatedLink(page)).toHaveCount(0);
  });

  test("says so at its address, without axe violations", async ({ page, makeAxeBuilder }) => {
    await page.goto("/public");
    await expect(page.getByRole("main").getByRole("link")).toHaveAttribute("href", "/home");
    await expect(page.getByRole("feed")).toHaveCount(0);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
