// SPDX-License-Identifier: AGPL-3.0-only
// Real-time updates in every width × locale project (FR-207, ADR-0027, web-client.md Y7): a new post from
// GET /api/v1/streaming waits behind the "N new posts" button and is not inserted until the button is
// pressed, and a notification raises the unread badge. `vite preview` has no API, so the spec answers the
// OAuth refresh and the client API with `page.route`; each answer to the stream is one response body
// that ends, so the client also reconnects throughout, as it does after the server ends a stream.
import type { Page } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Status = components["schemas"]["Status"];
type Notification = components["schemas"]["Notification"];

const bob: Status["account"] = {
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

function status(n: number, text: string): Status {
  const id = `0192f0c4-7a8e-7000-8000-${String(n).padStart(12, "0")}`;
  return {
    id,
    uri: `https://my.example/ap/statuses/${id}`,
    account: bob,
    content: `<p>${text}</p>`,
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: new Date(Date.now() - (100 - n) * 60_000).toISOString(),
    media_attachments: [],
    mentions: [],
    tags: [],
    emojis: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
  };
}

const loaded = [status(2, "The newest post when the page opened."), status(1, "An older post.")];
const arriving = status(3, "A post that arrived through the stream, with a long unbroken address https://example.com/a/very/long/path/that/must/wrap/before/it/scrolls/the/page");
const notification: Notification = { id: "0192f0c4-7a8e-7000-8000-0000000000c1", type: "follow", created_at: "2026-10-16T10:00:00Z", account: bob };

/** One SSE event as the server writes it. */
const sse = (event: string, data: unknown) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;

/**
 * A signed-in user's server: the home timeline, the read marker, and the stream, whose first response
 * delivers `events` and whose later ones (the reconnects) deliver nothing new.
 */
async function signedInServer(page: Page, events: string, unreadFromStream = 0) {
  const json = (body: unknown) => ({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  let connections = 0;
  await page.route("**/oauth/token/refresh", (route) => route.fulfill(json({ access_token: "e2e-token", token_type: "Bearer", expires_in: 3600 })));
  await page.route("**/api/v1/timelines/home*", (route) => route.fulfill(json({ items: loaded })));
  // Once the first stream has sent its notification, the server counts it as unread too: the client reads the
  // marker again after every reconnect.
  await page.route("**/api/v1/notifications/marker", (route) =>
    route.fulfill(json({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a1", unread_count: 1 + (connections > 0 ? unreadFromStream : 0) })),
  );
  await page.route("**/api/v1/streaming", async (route) => {
    connections += 1;
    await route.fulfill({ status: 200, contentType: "text/event-stream", body: `: connected\n\n${connections === 1 ? events : ""}` });
  });
}

test.describe("new posts in the home timeline", () => {
  test.beforeEach(async ({ page }) => {
    await signedInServer(page, sse("update", arriving));
    await page.goto("/home");
  });

  test("wait behind the button and are not inserted on their own", async ({ page, makeAxeBuilder }) => {
    const feed = page.getByRole("feed");
    const button = page.getByRole("main").getByRole("status").getByRole("button");
    await expect(button).toBeVisible();
    await expect(button).toHaveAccessibleName(/1/);
    // The stream has reconnected at least once since, and the feed still shows what it loaded.
    await expect(feed.getByRole("article")).toHaveCount(2);
    await expect(feed.getByRole("article").first()).toContainText("The newest post when the page opened.");
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - heading [level=1]
      - status:
        - button
      - paragraph
      - feed:
        - article
        - article
    `);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("show at the top when the button is pressed, with focus on the heading", async ({ page, makeAxeBuilder }) => {
    const feed = page.getByRole("feed");
    await page.getByRole("main").getByRole("status").getByRole("button").click();
    await expect(feed.getByRole("article")).toHaveCount(3);
    await expect(feed.getByRole("article").first()).toHaveAttribute("aria-posinset", "1");
    await expect(feed.getByRole("article").first()).toContainText("arrived through the stream");
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();
    await expect(page.getByRole("main").getByRole("status")).toBeEmpty();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

test("a notification from the stream raises the unread badge", async ({ page }) => {
  await signedInServer(page, sse("notification", notification), 1);
  await page.goto("/home");
  const link = page.getByRole("navigation").locator('a[href="/notifications"]');
  await expect(link).toHaveAccessibleName(/2/);
  await expect(link.locator('[aria-hidden="true"]', { hasText: "2" })).toBeVisible();
  await expectNoHorizontalScroll(page);
});
