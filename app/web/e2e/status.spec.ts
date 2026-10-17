// SPDX-License-Identifier: AGPL-3.0-only
// A post's page and the actions on it in every width × locale project (FR-106 to FR-108, FR-113,
// ADR-0027): the thread under its headings, the toggles and their rollback, the menu and the delete
// confirmation. `vite preview` has no API, so the spec answers the token refresh and the client API with
// `page.route`, as a signed-in user's server would. Names come from the project's catalog, so the spec
// finds the same elements in every locale.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Status = components["schemas"]["Status"];
type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale. */
function message(key: string): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text;
}

const alice: Status["account"] = {
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
};
const bob: Status["account"] = { ...alice, id: "0192f0c4-7a8e-7000-8000-000000000002", username: "bob", acct: "bob", display_name: "Bob", url: "https://my.example/@bob" };

const postId = "0192f0c4-7a8e-7000-8000-0000000000c1";

function status(id: string, account: Status["account"], content: string, overrides: Partial<Status> = {}): Status {
  return {
    id,
    uri: `https://my.example/ap/statuses/${id}`,
    url: `https://my.example/@${account.acct}/${id}`,
    account,
    content,
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: new Date(Date.now() - 60 * 60_000).toISOString(),
    media_attachments: [],
    mentions: [],
    tags: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
    favourited: false,
    boosted: false,
    bookmarked: false,
    ...overrides,
  };
}

const ancestor = status("0192f0c4-7a8e-7000-8000-0000000000c0", alice, "<p>Is anyone around for a long walk this weekend?</p>");
const post = status(
  postId,
  bob,
  "<p>Yes! A long link to the route: https://example.com/a/very/long/path/that/must/wrap/somewhere/before/it/scrolls/the/page/sideways</p>",
  { in_reply_to_id: ancestor.id, replies_count: 1, boosts_count: 3, favourites_count: 12 },
);
const reply = status("0192f0c4-7a8e-7000-8000-0000000000c2", alice, "<p>Great, see you there.</p>", { in_reply_to_id: postId });

/** Answers the client API as Alice's session; the first favourite fails with a 503 and the next ones succeed. */
async function serveApi(page: Page): Promise<void> {
  let favourites = 0;
  const json = (route: Route, body: unknown, statusCode = 200) =>
    route.fulfill({ status: statusCode, contentType: statusCode < 400 ? "application/json" : "application/problem+json", body: JSON.stringify(body) });
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600 }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    if (path === "/accounts/verify_credentials") return json(route, alice);
    if (path === "/instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0-e2e", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 } });
    }
    if (path === "/preferences") {
      // The user's locale is the project's: the app displays itself in the signed-in user's locale.
      return json(route, { locale: test.info().project.use.locale ?? "en", time_zone: "UTC", default_visibility: "public", default_post_language: "en", require_media_description: false, timeline_languages: [] });
    }
    if (path === "/notifications/marker") return json(route, { last_read_id: "0", unread_count: 0 });
    if (path === "/timelines/home") return json(route, { items: [] });
    if (path === `/statuses/${postId}/context`) return json(route, { ancestors: [ancestor], descendants: [reply] });
    if (path === `/statuses/${postId}`) return json(route, post);
    if (path === `/statuses/${postId}/favourite`) {
      const on = request.method() === "POST";
      if (on && favourites++ === 0) return json(route, { type: "about:blank", title: "Service Unavailable", status: 503 }, 503);
      return json(route, { ...post, favourited: on, favourites_count: post.favourites_count + (on ? 1 : 0) });
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
}

/** The article of the post the page is about. */
function focusedPost(page: Page) {
  return page.getByRole("main").getByRole("article").nth(1);
}

test.describe("a post's page", () => {
  test.beforeEach(async ({ page }) => {
    await serveApi(page);
    await page.goto(`/@bob/${postId}`);
    await expect(page.getByRole("heading", { level: 1, name: message("status.title") })).toBeVisible();
    await expect(page.getByRole("region", { name: message("status.thread.replies") }).getByRole("article")).toBeVisible();
  });

  test("has no axe violations and does not scroll horizontally", async ({ page, makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("shows the thread under its headings, with the actions of each post", async ({ page }) => {
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - heading [level=1]
      - region:
        - heading [level=2]
        - list:
          - listitem:
            - article:
              - link /Alice/
              - link /Alice/
              - link:
                - time
              - paragraph
              - button
              - button [pressed=false]
              - button [pressed=false]
              - button [pressed=false]
              - button
      - article:
        - link /Bob/
        - link /Bob/
        - time
        - paragraph
        - button
        - button [pressed=false]
        - button [pressed=false]
        - button [pressed=false]
        - button
      - region:
        - heading [level=2]
        - list:
          - listitem:
            - article
    `);
  });

  test("sets a failed favourite back and keeps the next one, with the menu and the confirmation axe-clean", async ({ page, makeAxeBuilder }) => {
    const article = focusedPost(page);
    const favourite = article.getByRole("button", { name: message("status.action.favourite") });

    await favourite.click();
    // The first one fails: the toast says so and the button is set back.
    await expect(page.getByRole("region", { name: message("shell.toast.region") }).getByText(/Service Unavailable/)).toBeVisible();
    await expect(favourite).toHaveAttribute("aria-pressed", "false");

    await favourite.click();
    await expect(favourite).toHaveAttribute("aria-pressed", "true");

    await article.getByRole("button", { name: message("status.action.more") }).click();
    await expect(page.getByRole("menu")).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

test("the viewer's own post asks before it is deleted", async ({ page, makeAxeBuilder }) => {
  await serveApi(page);
  const own = status("0192f0c4-7a8e-7000-8000-0000000000e1", alice, "<p>My own post</p>");
  await page.route(`**/api/v1/statuses/${own.id}`, (route) =>
    route.request().method() === "DELETE" ? route.fulfill({ status: 204 }) : route.fulfill({ json: own }),
  );
  await page.route(`**/api/v1/statuses/${own.id}/context`, (route) => route.fulfill({ json: { ancestors: [], descendants: [] } }));
  await page.goto(`/@alice/${own.id}`);

  await page.getByRole("main").getByRole("article").getByRole("button", { name: message("status.action.more") }).click();
  await page.getByRole("menuitem", { name: message("status.action.delete") }).click();
  const dialog = page.getByRole("alertdialog", { name: message("status.delete.title") });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: message("status.delete.cancel") })).toBeFocused();
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);

  await dialog.getByRole("button", { name: message("status.delete.confirm") }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.getByRole("region", { name: message("shell.toast.region") }).getByText(message("status.delete.done"))).toBeVisible();
});
