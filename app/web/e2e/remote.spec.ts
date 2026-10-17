// SPDX-License-Identifier: AGPL-3.0-only
// Accounts and posts of other servers in every width × locale project (FR-006, ADR-0027): the full handle,
// the links to the profile and the post on their own server, the content warning, and the pictures that
// come through the media proxy, one without a description. `vite preview` has no API, so the spec answers
// the token refresh and the client API with `page.route`, as a signed-in user's server would. Names come
// from the project's catalog, so the spec finds the same elements in every locale.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Account = components["schemas"]["Account"];
type MediaAttachment = components["schemas"]["MediaAttachment"];
type Status = components["schemas"]["Status"];
type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale, with its `{host}` replaced. */
function message(key: string, host: string): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text.replace("{host}", host);
}

const alice: Account = {
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

/** A long host that has to fit a 320 px card. */
const host = "a-rather-long-community-server-name.social.example.com";

const carol: Account = {
  id: "0192f0c4-7a8e-7000-8000-000000000003",
  username: "carol",
  acct: `carol@${host}`,
  domain: host,
  uri: `https://${host}/users/carol`,
  display_name: "Carol",
  note: "<p>Painter on another server.</p>",
  fields: [],
  url: `https://${host}/@carol`,
  locked: false,
  bot: false,
  created_at: "2026-09-01T00:00:00Z",
};

/** A picture as a data URL, so that no request leaves the page; it stands in for the media proxy URL. */
const picture = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><rect width="160" height="90" fill="#4f6d8f"/></svg>')}`;

/** An image of a remote post: no renditions here, the proxy's URL as `url` and `preview_url`. */
function remoteMedia(id: string, description: string | undefined): MediaAttachment {
  return {
    id: `0192f0c4-7a8e-7000-8000-0000000003${id}`,
    type: "image",
    state: "ready",
    meta: { focus: { x: 0, y: 0 } },
    url: picture,
    preview_url: picture,
    remote_url: `https://${host}/files/${id}.png`,
    ...(description === undefined ? {} : { description }),
  };
}

const postId = "0192f0c4-7a8e-7000-8000-0000000003c1";
const post: Status = {
  id: postId,
  uri: `https://${host}/users/carol/statuses/1`,
  url: `https://${host}/@carol/1`,
  account: carol,
  content: "<p>Two new paintings, finished this week.</p>",
  spoiler_text: "Paintings",
  sensitive: false,
  visibility: "public",
  language: "en",
  created_at: new Date(Date.now() - 60 * 60_000).toISOString(),
  media_attachments: [remoteMedia("d1", "A blue square on canvas"), remoteMedia("d2", undefined)],
  mentions: [],
  tags: [],
  replies_count: 0,
  boosts_count: 0,
  favourites_count: 0,
  favourited: false,
  boosted: false,
  bookmarked: false,
};

/** Answers the client API as Alice's session, with Carol's profile and post. */
async function serveApi(page: Page): Promise<void> {
  const json = (route: Route, body: unknown, statusCode = 200) =>
    route.fulfill({ status: statusCode, contentType: statusCode < 400 ? "application/json" : "application/problem+json", body: JSON.stringify(body) });
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600 }));
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    if (path === "/accounts/verify_credentials") return json(route, alice);
    if (path === "/instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0-e2e", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, rules: [] });
    }
    if (path === "/preferences") {
      // The user's locale is the project's: the app displays itself in the signed-in user's locale.
      return json(route, { locale: test.info().project.use.locale ?? "en", time_zone: "UTC", default_visibility: "public", default_post_language: "en", require_media_description: false, timeline_languages: [] });
    }
    if (path === "/notifications/marker") return json(route, { last_read_id: "0", unread_count: 0 });
    if (path === "/timelines/home") return json(route, { items: [] });
    if (path === "/accounts/lookup" && url.searchParams.get("acct") === carol.acct) return json(route, { ...carol, followers_count: 3, following_count: 4 });
    if (path === "/accounts/relationships") {
      return json(route, [
        { id: carol.id, following: false, requested: false, followed_by: false, requested_by: false, show_boosts: true, notify: false, blocking: false, muting: false, muting_notifications: false },
      ]);
    }
    if (path === `/accounts/${carol.id}/statuses`) return json(route, { items: [post] });
    if (path === `/statuses/${postId}`) return json(route, post);
    if (path === `/statuses/${postId}/context`) return json(route, { ancestors: [], descendants: [] });
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
}

test.describe("a profile of another server", () => {
  test.beforeEach(async ({ page }) => {
    await serveApi(page);
    await page.goto(`/@${carol.acct}`);
    await expect(page.getByRole("feed").getByRole("article")).toBeVisible();
  });

  test("links to the profile and the post on their server, without axe violations or sideways scrolling", async ({ page, makeAxeBuilder }) => {
    const main = page.getByRole("main");
    await expect(main.getByText(`@${carol.acct}`).first()).toBeVisible();
    // The host inside the sentence is wrapped in directional isolates (web-client.md Y10).
    await expect(main.getByRole("link", { name: message("profile.original", `\u2068${host}\u2069`) })).toHaveAttribute("href", carol.url);
    const article = page.getByRole("feed").getByRole("article");
    await expect(article.getByRole("link", { name: message("status.original", host) })).toHaveAttribute("href", post.url ?? "");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("shows the post's pictures behind its content warning, marking the one without a description", async ({ page, makeAxeBuilder }) => {
    const article = page.getByRole("feed").getByRole("article");
    // The first collapsed button is the content warning's; the menu button of the actions is collapsed too.
    await article.getByRole("button", { expanded: false }).first().click();
    const images = article.getByRole("img");
    await expect(images).toHaveCount(2);
    await expect(images.first()).toHaveAttribute("alt", "A blue square on canvas");
    await expect(images.first()).toHaveAttribute("src", picture);
    // The badge's text is the picture's name.
    const badge = (await images.nth(1).getAttribute("alt")) ?? "";
    expect(badge).not.toBe("");
    await expect(article.getByText(badge, { exact: true })).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("is the heading, the full handle, the link to the original profile, and the post with its link", async ({ page }) => {
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - heading "Carol" [level=1]
      - paragraph: "@carol@${host}"
      - paragraph:
        - link /${host.replaceAll(".", "\\.")}/
      - paragraph: Painter on another server.
      - tabpanel:
        - feed:
          - article:
            - link "Carol"
            - link /Carol/
            - link:
              - time
            - paragraph: Paintings
            - button
            - link /${host.replaceAll(".", "\\.")}/
    `);
  });
});

test.describe("a post of another server", () => {
  test("has its link on its page, without axe violations or sideways scrolling", async ({ page, makeAxeBuilder }) => {
    await serveApi(page);
    await page.goto(`/@${carol.acct}/${postId}`);
    const article = page.getByRole("main").getByRole("article");
    await expect(article).toBeVisible();
    await expect(article.getByRole("link", { name: message("status.original", host) })).toHaveAttribute("href", post.url ?? "");
    // The first collapsed button is the content warning's; the menu button of the actions is collapsed too.
    await article.getByRole("button", { expanded: false }).first().click();
    await expect(article.getByRole("img")).toHaveCount(2);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
