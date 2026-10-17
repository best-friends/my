// SPDX-License-Identifier: AGPL-3.0-only
// The notifications page and the unread badge in every width × locale project (FR-205, ADR-0027). The
// built client runs on `vite preview`, which has no API: the specs answer the OAuth refresh and the client
// API with `page.route`, as a signed-in user's server would. Names change with the locale, so elements are
// found by role and position.
import type { Page } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, expectTabWhollyVisible, test } from "./fixtures";

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

/** Newest first: a mention (unread), a boost of a post behind a content warning (unread), a follow (read). */
const notifications = [
  { id: mentionId, type: "mention", created_at: "2026-10-16T09:00:00Z", account: bob, status: post },
  {
    id: "0192f0c4-7a8e-7000-8000-0000000000a2",
    type: "boost",
    created_at: "2026-10-16T08:00:00Z",
    account: bob,
    status: { ...post, spoiler_text: "spoilers" },
  },
  { id: "0192f0c4-7a8e-7000-8000-0000000000a1", type: "follow", created_at: "2026-10-15T09:00:00Z", account: { ...bob, display_name: "ボブ・とても長い表示名の人です ✨✨✨✨✨" } },
];

/**
 * Answers the session refresh and the notification operations; returns the read marker updates it got and
 * the posts requested on their own, which should be none: the notifications embed them.
 */
async function signedInServer(page: Page) {
  const updates: string[] = [];
  const statusRequests: string[] = [];
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
  await page.route("**/api/v1/statuses/*", (route) => {
    statusRequests.push(route.request().url());
    return route.fulfill({ status: 404 });
  });
  return { updates, statusRequests };
}

test.describe("notifications", () => {
  let server: Awaited<ReturnType<typeof signedInServer>>;
  test.beforeEach(async ({ page }) => {
    server = await signedInServer(page);
  });

  test("the navigation shows the unread count and names its link with it", async ({ page }) => {
    await page.goto("/");
    const link = page.getByRole("navigation").locator('a[href="/notifications"]');
    await expect(link).toHaveAccessibleName(/2/);
    await expect(link.locator('[aria-hidden="true"]', { hasText: "2" })).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  test("the page lists the notifications, passes axe, and does not scroll sideways", async ({ page, makeAxeBuilder }) => {
    await page.goto("/notifications");
    const feed = page.getByRole("feed");
    await expect(feed.getByRole("article")).toHaveCount(3);
    // The post of the mention is shown, and the boosted post is behind its content warning.
    // Its link opens the post's page in the app.
    await expect(feed.getByRole("article").first().getByRole("link")).toHaveAttribute("href", `/@bob/${postId}`);
    await expect(feed.getByRole("article").nth(1).getByRole("button")).toBeVisible();
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
    expect(server.statusRequests).toEqual([]);
  });

  test("showing the list marks the newest notification read and clears the badge", async ({ page }) => {
    await page.goto("/notifications");
    await expect(page.getByRole("feed").getByRole("article")).toHaveCount(3);
    await expect.poll(() => server.updates).toEqual([mentionId]);
    await expect(page.getByRole("navigation").locator('a[href="/notifications"]').locator('[aria-hidden="true"]', { hasText: "2" })).toHaveCount(0);
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

  // On a phone the type tabs do not all fit: the list scrolls sideways and shows each tab whole once reached.
  test("every type tab is reached whole from the keyboard, also when it starts selected", async ({ page }) => {
    await page.goto("/notifications");
    const tabs = page.getByRole("tab");
    await expect(tabs).toHaveCount(5);
    await tabs.first().focus();
    const rtl = (await page.locator("html").getAttribute("dir")) === "rtl";
    for (let index = 1; index < 5; index += 1) {
      await page.keyboard.press(rtl ? "ArrowLeft" : "ArrowRight");
      await expect(tabs.nth(index)).toBeFocused();
      await expectTabWhollyVisible(tabs.nth(index));
    }
    await expect(page).toHaveURL(/\/notifications\?filter=favourites$/);
    await page.reload();
    await expect(tabs.last()).toHaveAttribute("aria-selected", "true");
    await expectTabWhollyVisible(tabs.last());
    await expectNoHorizontalScroll(page);
  });

  test("the type tabs scroll sideways under a finger, and a tap selects the one reached", async ({ page }, testInfo) => {
    test.skip(testInfo.project.use.hasTouch !== true, "touch only on the phone widths");
    await page.goto("/notifications");
    const list = page.getByRole("tablist");
    const favourites = page.getByRole("tab").last();
    await expect(page.getByRole("tab").first()).toHaveAttribute("aria-selected", "true");
    // Swipe towards the inline start, which brings the tabs at the inline end into view.
    const box = await list.boundingBox();
    if (box === null) throw new Error("the tab list is not rendered");
    const rtl = (await page.locator("html").getAttribute("dir")) === "rtl";
    // Playwright has only tap, so the swipe is raw touch events, which Chromium turns into a scroll.
    const cdp = await page.context().newCDPSession(page);
    const y = box.y + box.height / 2;
    const from = rtl ? box.x + 20 : box.x + box.width - 20;
    const step = (rtl ? box.width - 40 : 40 - box.width) / 10;
    await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: from, y }] });
    for (let move = 1; move <= 10; move += 1) {
      await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: from + step * move, y }] });
    }
    await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    // The list scrolled: the tabs at the inline start are now behind the fade of that edge.
    if (await list.evaluate((element) => element.scrollWidth > element.clientWidth)) await expect(list).toHaveAttribute("data-overflow-start");
    await expectTabWhollyVisible(favourites);
    await favourites.tap();
    await expect(favourites).toHaveAttribute("aria-selected", "true");
    await expect(page).toHaveURL(/\/notifications\?filter=favourites$/);
    await expectTabWhollyVisible(favourites);
  });
});
