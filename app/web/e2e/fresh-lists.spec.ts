// SPDX-License-Identifier: AGPL-3.0-only
// A new post is on the viewer's profile from the moment the profile opens again (web-client.md S1), against a
// real `my serve` on SQLite (e2e/server.ts) in every width × locale project. The profile's route reads its
// list from the cache as it is (S7), so the post fetches again the lists that no page shows before it is
// shown done; a list that it had only marked stale would show the posts from before until a refetch ended.
// Every GET of the client API takes 800 ms more, as over a real network, so that such a list would be seen.
// The user is new to the test (a random name), created with `my admin create` (e2e/server-cli.ts).
import { randomBytes } from "node:crypto";
import { readFileSync } from "node:fs";
import type { Page } from "@playwright/test";
import { axeBuilder, expect, expectNoAxeViolations, expectNoHorizontalScroll, localeDir, openFromNavigation, test } from "./fixtures";
import { ownsServer, runMy } from "./server-cli.ts";
import { serverURL } from "./server-origin.ts";

test.use({ baseURL: serverURL, reducedMotion: "reduce" });

type Catalog = Record<string, { defaultMessage: string }>;

/** The project's locale. */
function projectLocale(): string {
  return test.info().project.use.locale ?? "en";
}

/** The client message `key` in the project's locale. */
function message(key: string): string {
  const locale = projectLocale();
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text;
}

/** How much longer every GET of the client API takes, in ms. */
const latency = 800;

const pageHeading = (page: Page) => page.getByRole("main").getByRole("heading", { level: 1 });

/** Signs in through the server's screen, and makes the project's locale the user's display language. */
async function signIn(page: Page, email: string, password: string): Promise<void> {
  await page.goto("/login");
  await expect(pageHeading(page)).toBeVisible();
  const authorized = page.waitForRequest(
    (request) => new URL(request.url()).pathname.startsWith("/api/") && request.headers()["authorization"]?.startsWith("Bearer ") === true,
  );
  await page.getByRole("main").getByRole("button").click();
  await page.locator('input[type="email"][autocomplete="username"]').fill(email);
  await page.locator('input[type="password"][autocomplete="current-password"]').fill(password);
  await page.locator('main button[type="submit"]').first().click();
  const authorization = (await authorized).headers()["authorization"] ?? "";
  await expect(page).toHaveURL(`${serverURL}/home`);
  const locale = projectLocale();
  const response = await page.request.patch(`${serverURL}/api/v1/preferences`, { headers: { Authorization: authorization }, data: { locale } });
  expect(response.ok(), `PATCH /api/v1/preferences: ${String(response.status())}`).toBe(true);
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("dir", localeDir(locale));
  await expect(pageHeading(page)).toHaveText(message("home.title"));
}

/** Posts `text` from the new-post form (a sheet below 1280 px, the side panel from there) and waits until it is done. */
async function post(page: Page, text: string): Promise<void> {
  if ((page.viewportSize()?.width ?? 0) < 1280) {
    await page.getByRole("button", { name: message("compose.open") }).click();
  }
  const form = page.getByRole("form", { name: message("compose.title") });
  await form.getByRole("textbox", { name: message("compose.text.label") }).fill(text);
  // The toast of an earlier post would pass for this one's: it goes after 5 s, unless the pointer is over it.
  const posted = page.getByRole("region", { name: message("shell.toast.region") }).getByText(message("compose.posted"));
  await page.mouse.move(0, 0);
  await expect(posted).toHaveCount(0, { timeout: 15_000 });
  await form.getByRole("button", { name: message("compose.submit") }).click();
  await expect(posted).toBeVisible({ timeout: latency * 10 });
}

/**
 * Starts watching the page: from now on, every change of the DOM that shows the profile of `username` with its
 * feed of posts but without a post containing `text` counts as a stale list. Returns the count so far.
 */
async function watchForStaleProfile(page: Page, username: string, text: string): Promise<() => Promise<number>> {
  await page.evaluate(
    ([name, wanted]) => {
      const w = window as unknown as { __staleRenders: number };
      w.__staleRenders = 0;
      const check = () => {
        const main = document.querySelector("main");
        const feed = main?.querySelector('[role="feed"]');
        if (main?.querySelector("h1")?.textContent !== name || feed == null) return;
        if (!feed.textContent.includes(wanted)) w.__staleRenders += 1;
      };
      new MutationObserver(check).observe(document.body, { subtree: true, childList: true, characterData: true });
    },
    [username, text] as const,
  );
  return () => page.evaluate(() => (window as unknown as { __staleRenders: number }).__staleRenders);
}

test("a new post is on the profile from the moment it opens again", async ({ page }, info) => {
  test.skip(!ownsServer, "creates its user with bin/my against the server's database");
  test.setTimeout(120_000);
  const touch = info.project.use.hasTouch === true;
  const username = `e2e_fresh_${randomBytes(4).toString("hex")}`;
  const email = `${username}@example.com`;
  const password = `e2e-password-${username}`;
  runMy(["admin", "create", "-username", username, "-email", email], `${password}\n`);
  await signIn(page, email, password);
  await page.route("**/api/v1/**", async (route) => {
    if (route.request().method() === "GET" && !route.request().url().includes("/streaming")) {
      await new Promise((resolve) => setTimeout(resolve, latency));
    }
    await route.continue();
  });

  // The profile has a post, and its list is cached.
  const first = `first ${username}`;
  await post(page, first);
  await openFromNavigation(page, "/profile", touch);
  await expect(pageHeading(page)).toHaveText(username);
  await expect(page.getByRole("feed").getByRole("article").filter({ hasText: first })).toBeVisible();

  // Another post from the home timeline, and the profile again.
  await openFromNavigation(page, "/home", touch);
  await expect(pageHeading(page)).toHaveText(message("home.title"));
  const second = `second ${username}`;
  await post(page, second);
  const staleRenders = await watchForStaleProfile(page, username, second);
  await openFromNavigation(page, "/profile", touch);
  await expect(pageHeading(page)).toHaveText(username);
  const feed = page.getByRole("feed", { name: message("profile.tab.posts") });
  await expect(feed.getByRole("article").filter({ hasText: second })).toBeVisible();
  // Give a stale list the time it would have taken to be replaced.
  await page.waitForTimeout(latency * 2);
  expect(await staleRenders(), "renders of the profile's feed without the new post").toBe(0);

  await expect(feed.getByRole("article").first()).toMatchAriaSnapshot(`
    - article:
      - link
      - link:
        - time
      - paragraph: ${second}
  `);
  await expectNoAxeViolations(() => axeBuilder(page));
  await expectNoHorizontalScroll(page);
});
