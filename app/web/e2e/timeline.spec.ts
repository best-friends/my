// SPDX-License-Identifier: AGPL-3.0-only
// The timelines in every width × locale project (ADR-0027): the feed of the local and hashtag timelines,
// its paging and keys, and the empty state. `vite preview` has no client API, so the spec answers the
// timeline operations itself with pages shaped like api/openapi.yaml's StatusPage.
import type { Page } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Status = components["schemas"]["Status"];
type StatusPage = components["schemas"]["StatusPage"];

const account: Status["account"] = {
  id: "0192f0c4-7a8e-7000-8000-000000000002",
  username: "bob",
  acct: "bob",
  domain: "my.example",
  uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000002",
  display_name: "Bob",
  note: "",
  fields: [],
  emojis: [],
  url: "https://my.example/@bob",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

/** A picture as a data URL, so that no request leaves the page. */
const picture = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><rect width="160" height="90" fill="#4f6d8f"/></svg>')}`;
/** An avatar, also a data URL. */
const avatar = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><circle cx="24" cy="24" r="24" fill="#8f4f6d"/></svg>')}`;

function status(n: number, overrides: Partial<Status> = {}): Status {
  return {
    id: `0192f0c4-7a8e-7000-8000-${String(n).padStart(12, "0")}`,
    uri: `https://my.example/ap/statuses/${String(n)}`,
    account,
    content: `<p>Post number ${String(n)} on the local timeline.</p>`,
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: new Date(Date.now() - n * 7 * 60_000).toISOString(),
    media_attachments: [],
    mentions: [],
    tags: [],
    emojis: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
    reactions: [],
    ...overrides,
  };
}

/** The first page: plain posts, a long unbroken text, a boost, a content warning and pictures. */
function firstPage(): StatusPage {
  return {
    items: [
      status(1, {
        account: { ...account, display_name: "Bartholomew Montgomery-Fitzwilliam the Third", acct: "bartholomew_montgomery_fitzwilliam@social.example.com" },
        content: '<p>A long link: <a href="https://example.com/a/very/long/path/that/does/not/break/anywhere/on/its/own">https://example.com/a/very/long/path/that/does/not/break/anywhere/on/its/own</a></p>',
      }),
      status(2, { account: { ...account, username: "carol", acct: "carol", display_name: "Carol" }, content: "", boost: status(20) }),
      status(3, { spoiler_text: "Food", sensitive: true, media_attachments: [media("m1", "A cake")] }),
      status(4, { media_attachments: [media("m2", "A blue square"), media("m3", undefined)] }),
      status(5, { account: { ...account, avatar_url: avatar } }),
    ],
    next_cursor: "page-2",
  };
}

function media(id: string, description: string | undefined): Status["media_attachments"][number] {
  const variant = { url: picture, content_type: "image/svg+xml", width: 160, height: 90, size: 200 };
  return {
    id: `0192f0c4-7a8e-7000-8000-0000000000${id}`,
    type: "image",
    state: "ready",
    meta: { focus: { x: 0, y: 0 } },
    variants: { full: variant, thumbnail: variant },
    url: picture,
    preview_url: picture,
    ...(description === undefined ? {} : { description }),
  };
}

/** Answers the timeline operations: two pages for the local timeline, none for hashtags. */
async function serveTimelines(page: Page): Promise<void> {
  await page.route("**/api/v1/timelines/local*", async (route) => {
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    const body: StatusPage = cursor === null ? firstPage() : { items: [status(6), status(7)] };
    await route.fulfill({ json: body });
  });
  await page.route("**/api/v1/timelines/tag/*", async (route) => {
    await route.fulfill({ json: { items: [] } satisfies StatusPage });
  });
}

test.describe("the local timeline", () => {
  test.beforeEach(async ({ page }) => {
    await serveTimelines(page);
    await page.goto("/local");
    await expect(page.getByRole("feed")).toBeVisible();
  });

  test("has no axe violations, also with the hidden content shown", async ({ page, makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
    const warned = page.getByRole("feed").getByRole("article").nth(2);
    await warned.getByRole("button", { expanded: false }).click();
    // The second button is the toggle of the sensitive media, inside the content that is now shown.
    const reveal = warned.getByRole("button").nth(1);
    await expect(reveal).toHaveAttribute("aria-pressed", "false");
    await reveal.click();
    await expect(warned.getByRole("img")).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("is a feed of articles named by the page heading", async ({ page }) => {
    const heading = (await page.getByRole("heading", { level: 1 }).textContent()) ?? "";
    await expect(page.getByRole("feed", { name: heading })).toBeVisible();
    const articles = page.getByRole("feed").getByRole("article");
    // The second page may have loaded already (its end is in view on tall screens); the first page comes first.
    await expect(articles.first()).toHaveAttribute("aria-posinset", "1");
    // Each post links its author's name to the author's profile (the picture's link to it is hidden), and
    // a boost its booster's name.
    await expect(page.getByRole("feed")).toMatchAriaSnapshot(`
      - feed:
        - article:
          - link /Bartholomew/
          - link:
            - time
          - paragraph
        - article:
          - paragraph:
            - link /Carol/
          - link /Bob/
          - link:
            - time
          - paragraph
        - article:
          - link /Bob/
          - link:
            - time
          - paragraph: Food
          - button [expanded=false]
        - article:
          - link /Bob/
          - link:
            - time
          - paragraph
          - img "A blue square"
          - img
        - article:
          - link /Bob/
          - link:
            - time
          - paragraph
    `);
  });

  test("shows the author's avatar, and the first letter of the name without one", async ({ page }) => {
    const articles = page.getByRole("feed").getByRole("article");
    // The avatar is decorative (alt="") inside a link hidden from the tree: the author's name is next to it.
    const image = articles.nth(4).getByRole("presentation", { includeHidden: true });
    await expect(image).toHaveAttribute("src", avatar);
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    await expect(articles.nth(0).getByRole("presentation", { includeHidden: true })).toHaveCount(0);
    await expect(articles.nth(0).getByText("B", { exact: true })).toBeVisible();
  });

  test("stops the keyboard once per profile: the avatar is out of the tab order", async ({ page }) => {
    const article = page.getByRole("feed").getByRole("article").nth(1);
    const links = article.getByRole("link");
    // After the article itself (the feed pattern): the booster's name, the author's name, the time.
    await article.focus();
    for (const n of [0, 1, 2]) {
      await page.keyboard.press("Tab");
      await expect(links.nth(n)).toBeFocused();
    }
    await expect(links).toHaveCount(3);
    await page.keyboard.press("Shift+Tab");
    await page.keyboard.press("Shift+Tab");
    await expect(links.nth(0)).toBeFocused();
  });

  test("loads the next page at the end, and then knows its size", async ({ page }) => {
    const articles = page.getByRole("feed").getByRole("article");
    // The feed loads the next page when its end comes near the viewport.
    await expect(async () => {
      await page.evaluate(() => {
        window.scrollTo(0, document.documentElement.scrollHeight);
      });
      await expect(articles.last()).toHaveAttribute("aria-posinset", "7", { timeout: 500 });
    }).toPass();
    await expect(articles.first()).toHaveAttribute("aria-setsize", "7");
    await expect(page.getByRole("feed")).toHaveAttribute("aria-busy", "false");
    await expectNoHorizontalScroll(page);
  });

  test("moves between articles with Page Down and Page Up", async ({ page }) => {
    const articles = page.getByRole("feed").getByRole("article");
    await articles.first().focus();
    await page.keyboard.press("PageDown");
    await expect(articles.nth(1)).toBeFocused();
    await page.keyboard.press("PageUp");
    await expect(articles.first()).toBeFocused();
  });
});

test.describe("the hashtag timeline", () => {
  test.beforeEach(async ({ page }) => {
    await serveTimelines(page);
    await page.goto("/tags/cats");
    await expect(page.getByRole("main").getByRole("heading", { level: 2 })).toBeVisible();
  });

  test("is titled after the hashtag and shows the empty state", async ({ page }) => {
    await expect(page.getByRole("heading", { level: 1 })).toContainText("#cats");
    await expect(page.getByRole("main").getByRole("heading", { level: 2 })).toContainText("#cats");
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });
});

test("the root address opens the home timeline, which offers to sign in without a session", async ({ page, makeAxeBuilder }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.getByRole("main").getByRole("link")).toHaveAttribute("href", "/login?return_to=%2Fhome");
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);
});
