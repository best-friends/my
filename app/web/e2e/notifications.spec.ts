// SPDX-License-Identifier: AGPL-3.0-only
// The notifications page and the unread badge in every width × locale project (FR-205, ADR-0027). The
// built client runs on `vite preview`, which has no API: the specs answer the OAuth refresh and the client
// API with `page.route`, as a signed-in user's server would. Names change with the locale, so elements are
// found by role and position.
import type { Page } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

const bob = {
  id: "0192f0c4-7a8e-7000-8000-000000000002",
  username: "bob",
  acct: "bob",
  display_name: "Bob",
  note: "",
  fields: [],
  url: "https://my.example/@bob",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

const mentionId = "0192f0c4-7a8e-7000-8000-0000000000a3";
const postId = "0192f0c4-7a8e-7000-8000-0000000000b1";

/** Newest first: a mention (unread), a boost of a post behind a content warning (unread), a follow (read). */
const notifications = [
  { id: mentionId, type: "mention", created_at: "2026-10-16T09:00:00Z", account: bob, status_id: postId },
  { id: "0192f0c4-7a8e-7000-8000-0000000000a2", type: "boost", created_at: "2026-10-16T08:00:00Z", account: bob, status_id: postId },
  { id: "0192f0c4-7a8e-7000-8000-0000000000a1", type: "follow", created_at: "2026-10-15T09:00:00Z", account: { ...bob, display_name: "ボブ・とても長い表示名の人です ✨✨✨✨✨" } },
];

const post = {
  id: postId,
  uri: `https://my.example/users/bob/statuses/${postId}`,
  url: `https://my.example/@bob/${postId}`,
  account: bob,
  content: "<p>@alice https://example.com/a/very/long/path/that/must/wrap/somewhere/before/it/scrolls/the/page/sideways</p>",
  spoiler_text: "",
  sensitive: false,
  visibility: "public",
  language: "en",
  created_at: "2026-10-16T09:00:00Z",
  media_attachments: [],
  mentions: [],
  tags: [],
  replies_count: 0,
  boosts_count: 0,
  favourites_count: 0,
};

/** Answers the session refresh and the notification operations; returns the read marker updates it got. */
async function signedInServer(page: Page) {
  const updates: string[] = [];
  let marker: { last_read_id: string; unread_count: number } = { last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a1", unread_count: 2 };
  const json = (body: unknown) => ({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  await page.route("**/oauth/token/refresh", (route) => route.fulfill(json({ access_token: "e2e-token", token_type: "Bearer", expires_in: 3600 })));
  await page.route("**/api/v1/notifications/marker", async (route) => {
    if (route.request().method() === "PUT") {
      const { last_read_id: id } = route.request().postDataJSON() as { last_read_id: string };
      updates.push(id);
      marker = { last_read_id: id, unread_count: 0 };
    }
    await route.fulfill(json(marker));
  });
  await page.route(
    (url) => url.pathname === "/api/v1/notifications",
    (route) => {
      const types = new URL(route.request().url()).searchParams.getAll("types");
      return route.fulfill(json({ items: notifications.filter((n) => types.length === 0 || types.includes(n.type)) }));
    },
  );
  await page.route("**/api/v1/statuses/*", (route) => route.fulfill(json(post)));
  return { updates };
}

test.describe("notifications", () => {
  let server: Awaited<ReturnType<typeof signedInServer>>;
  test.beforeEach(async ({ page }) => {
    server = await signedInServer(page);
  });

  test("the navigation shows the unread count and names its link with it", async ({ page }) => {
    await page.goto("/");
    const link = page.getByRole("navigation").getByRole("link").nth(2);
    await expect(link).toHaveAccessibleName(/2/);
    await expect(link.locator('[aria-hidden="true"]', { hasText: "2" })).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  test("the page lists the notifications, passes axe, and does not scroll sideways", async ({ page, makeAxeBuilder }) => {
    await page.goto("/notifications");
    const feed = page.getByRole("feed");
    await expect(feed.getByRole("article")).toHaveCount(3);
    // The post of the mention has loaded, and the boosted post is behind its content warning.
    await expect(feed.getByRole("article").first().getByRole("link")).toHaveAttribute("href", post.url);
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - heading [level=1]
      - tablist:
        - tab [selected]
        - tab
        - tab
        - tab
        - tab
      - tabpanel:
        - feed:
          - article
          - article
          - article
    `);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("showing the list marks the newest notification read and clears the badge", async ({ page }) => {
    await page.goto("/notifications");
    await expect(page.getByRole("feed").getByRole("article")).toHaveCount(3);
    await expect.poll(() => server.updates).toEqual([mentionId]);
    await expect(page.getByRole("navigation").getByRole("link").nth(2).locator('[aria-hidden="true"]', { hasText: "2" })).toHaveCount(0);
  });

  test("the type filter works from the keyboard and lives in the URL", async ({ page }) => {
    await page.goto("/notifications");
    const tabs = page.getByRole("tab");
    await expect(tabs.first()).toHaveAttribute("aria-selected", "true");
    await tabs.first().focus();
    const rtl = (await page.locator("html").getAttribute("dir")) === "rtl";
    await page.keyboard.press(rtl ? "ArrowLeft" : "ArrowRight");
    await expect(page).toHaveURL(/\/notifications\?filter=mentions$/);
    await expect(tabs.nth(1)).toBeFocused();
    await expect(tabs.nth(1)).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("feed").getByRole("article")).toHaveCount(1);

    await page.reload();
    await expect(page.getByRole("tab").nth(1)).toHaveAttribute("aria-selected", "true");
  });
});
