// SPDX-License-Identifier: AGPL-3.0-only
// The profile pages in every width × locale project (FR-204, FR-301 to FR-303, ADR-0027): the header, the
// follow button and its confirmation, the menu, the tabs and the follow lists. The built client runs on
// `vite preview`, which has no API: the specs answer the OAuth refresh and the client API with
// `page.route`, as a signed-in user's server would. Names change with the locale, so elements are found by
// role and position.
import type { Page } from "@playwright/test";
import type { components } from "../src/lib/api/schema.gen.ts";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Account = components["schemas"]["Account"];
type Relationship = components["schemas"]["Relationship"];
type Status = components["schemas"]["Status"];

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

/** Pictures as data URLs, so that no request leaves the page. */
const avatar = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><circle cx="24" cy="24" r="24" fill="#8f4f6d"/></svg>')}`;
const header = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 100"><rect width="300" height="100" fill="#4f6d8f"/></svg>')}`;

/** A profile that tests the layout: a long name, a long unbroken link, fields in two scripts, and images. */
const bob: Account = {
  ...alice,
  id: "0192f0c4-7a8e-7000-8000-000000000002",
  username: "bartholomew_montgomery",
  acct: "bartholomew_montgomery",
  display_name: "Bartholomew Montgomery-Fitzwilliam the Third ✨",
  note: '<p>Baker. <a href="https://example.com/a/very/long/path/that/does/not/break/anywhere/on/its/own">https://example.com/a/very/long/path/that/does/not/break/anywhere/on/its/own</a></p>',
  fields: [
    { name: "Site", value: '<a href="https://bakery.example">bakery.example</a>' },
    { name: "パン屋", value: "<p>毎朝焼いています</p>" },
  ],
  url: "https://my.example/@bartholomew_montgomery",
  locked: true,
  followers_count: 1234,
  following_count: 1,
  avatar_url: avatar,
  header_url: header,
};

function post(n: number): Status {
  return {
    id: `0192f0c4-7a8e-7000-8000-${String(n).padStart(12, "0")}`,
    uri: `https://my.example/ap/statuses/${String(n)}`,
    account: bob,
    content: `<p>Fresh bread number ${String(n)}.</p>`,
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: new Date(Date.now() - n * 60 * 60_000).toISOString(),
    media_attachments: [],
    mentions: [],
    tags: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
  };
}

const none: Relationship = {
  id: bob.id,
  following: false,
  requested: false,
  followed_by: true,
  requested_by: false,
  show_boosts: true,
  notify: false,
  blocking: false,
  muting: false,
  muting_notifications: false,
};

/** Answers the session refresh and the account operations as Alice's server; returns the changes it got. */
async function signedInServer(page: Page) {
  const changes: string[] = [];
  let relationship = none;
  const json = (body: unknown) => ({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  await page.route("**/oauth/token/refresh", (route) => route.fulfill(json({ access_token: "e2e-token", token_type: "Bearer", expires_in: 3600 })));
  await page.route("**/api/v1/accounts/verify_credentials", (route) => route.fulfill(json(alice)));
  await page.route("**/api/v1/notifications/marker", (route) => route.fulfill(json({ unread_count: 0 })));
  await page.route("**/api/v1/accounts/lookup*", (route) => {
    const acct = new URL(route.request().url()).searchParams.get("acct");
    return acct === bob.acct
      ? route.fulfill(json(bob))
      : route.fulfill({ status: 404, contentType: "application/problem+json", body: JSON.stringify({ type: "about:blank", title: "Not Found", status: 404 }) });
  });
  await page.route("**/api/v1/accounts/relationships*", (route) => route.fulfill(json([relationship])));
  await page.route(`**/api/v1/accounts/${bob.id}/statuses*`, (route) =>
    route.fulfill(json({ items: [post(1), post(2), { ...post(3), boost: post(30), content: "" }] })),
  );
  await page.route(`**/api/v1/accounts/${bob.id}/followers*`, (route) => route.fulfill(json({ accounts: [alice, { ...bob, id: "0192f0c4-7a8e-7000-8000-000000000003" }] })));
  for (const action of ["follow", "unfollow", "mute", "unmute", "block", "unblock"]) {
    await page.route(`**/api/v1/accounts/${bob.id}/${action}`, (route) => {
      changes.push(action);
      relationship = {
        ...relationship,
        ...(action === "follow" && { requested: true }),
        ...(action === "unfollow" && { following: false, requested: false }),
        ...(action === "mute" && { muting: true, muting_notifications: true }),
        ...(action === "unmute" && { muting: false, muting_notifications: false }),
        ...(action === "block" && { blocking: true, following: false, requested: false, followed_by: false }),
        ...(action === "unblock" && { blocking: false }),
      };
      return route.fulfill(json(relationship));
    });
  }
  return { changes };
}

const profile = `/@${bob.acct}`;

test.describe("a profile", () => {
  let server: Awaited<ReturnType<typeof signedInServer>>;
  test.beforeEach(async ({ page }) => {
    server = await signedInServer(page);
    await page.goto(profile);
    await expect(page.getByRole("feed")).toBeVisible();
    // The relationship has loaded: the follow button is there.
    await expect(page.getByRole("main").getByRole("button").first()).toBeVisible();
  });

  test("has no axe violations and does not scroll sideways", async ({ page, makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("shows the header image and the avatar, both decorative", async ({ page }) => {
    // The header's images come before the tabs; the posts below show the avatar again.
    const images = page.getByRole("main").locator("header").first().getByRole("presentation");
    await expect(images).toHaveCount(2);
    for (const [index, src] of [header, avatar].entries()) {
      const image = images.nth(index);
      await expect(image).toHaveAttribute("src", src);
      await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    }
  });

  test("is a heading, the relationship buttons, the profile and its posts in tabs", async ({ page }) => {
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - button
      - button
      - heading /Bartholomew/ [level=1]
      - paragraph: "@bartholomew_montgomery@my.example"
      - list:
        - listitem
        - listitem
      - paragraph:
        - link
      - term: Site
      - definition:
        - link
      - term: パン屋
      - definition:
        - paragraph: 毎朝焼いています
      - paragraph
      - list:
        - listitem:
          - link /1/
        - listitem:
          - link /1,?234/
      - tablist:
        - tab [selected]
        - tab
        - tab
      - tabpanel:
        - heading [level=2]
        - paragraph
        - feed:
          - article
          - article
          - article
    `);
  });

  test("sends a follow request and withdraws it after asking, with the focus kept on the button", async ({ page, makeAxeBuilder }) => {
    const follow = page.getByRole("main").getByRole("button").first();
    await follow.click();
    await expect.poll(() => server.changes).toEqual(["follow"]);
    // The same button now says the request is pending, and pressing it asks before withdrawing it.
    await follow.click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    await dialog.getByRole("button").last().click();
    await expect(dialog).toBeHidden();
    await expect.poll(() => server.changes).toEqual(["follow", "unfollow"]);
    await expect(follow).toBeFocused();
  });

  test("mutes from the menu, which works from the keyboard", async ({ page, makeAxeBuilder }) => {
    const more = page.getByRole("main").getByRole("button").nth(1);
    await more.focus();
    await page.keyboard.press("Enter");
    const menu = page.getByRole("menu");
    await expect(menu.getByRole("menuitem").first()).toBeFocused();
    await expectNoAxeViolations(makeAxeBuilder);
    await page.keyboard.press("Enter");
    await expect(menu).toBeHidden();
    await expect.poll(() => server.changes).toEqual(["mute"]);
    await expect(more).toBeFocused();
  });

  test("keeps the selected tab in the URL", async ({ page }) => {
    await page.getByRole("tab").nth(1).click();
    await expect(page).toHaveURL(/\?tab=replies$/);
    await expect(page.getByRole("tab").nth(1)).toHaveAttribute("aria-selected", "true");
    await page.reload();
    await expect(page.getByRole("tab").nth(1)).toHaveAttribute("aria-selected", "true");
  });
});

test.describe("the followers of an account", () => {
  test.beforeEach(async ({ page }) => {
    await signedInServer(page);
    await page.goto(`${profile}/followers`);
    await expect(page.getByRole("main").getByRole("list")).toBeVisible();
  });

  test("link to the followers' profiles, pass axe and do not scroll sideways", async ({ page, makeAxeBuilder }) => {
    const links = page.getByRole("main").getByRole("list").getByRole("link");
    await expect(links).toHaveCount(2);
    await expect(links.first()).toHaveAttribute("href", "/@alice");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

test("an unknown handle is not found", async ({ page, makeAxeBuilder }) => {
  await signedInServer(page);
  await page.goto("/@nobody");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByRole("main").getByRole("link")).toHaveAttribute("href", "/home");
  await expectNoAxeViolations(makeAxeBuilder);
});
