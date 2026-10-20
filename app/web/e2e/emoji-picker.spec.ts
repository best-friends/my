// SPDX-License-Identifier: AGPL-3.0-only
// The custom emoji picker of the new-post form in every width × locale project (ADR-0051 §4, ADR-0027): open, it
// passes axe without scrolling sideways, its grid is reached with Tab from the search field and moves with the
// arrow keys in the writing direction, and the emoji picked with Enter goes into the text. `vite preview` has no
// API, so the spec answers the client API and the token refresh as a signed-in session (as compose.spec.ts does).
// Names come from the project's catalog.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale. */
function message(key: string): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text;
}

// Plain squares stand in for the emoji images, served with the page (the CSP allows data: images).
const orange = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGUlEQVR4nGN4ViH3nxLMMGrAqAGjBgwXAwAvJnsfj06SzAAAAABJRU5ErkJggg==";
const blue = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGUlEQVR4nGPQ6LnznxLMMGrAqAGjBgwXAwBfb48fVw/D1AAAAABJRU5ErkJggg==";

/** The emoji the server offers: two without a category, and a category with a long name. */
const emojis = [
  { shortcode: "wave", url: orange, static_url: orange, aliases: ["hello"] },
  { shortcode: "thanks", url: blue, static_url: blue, aliases: [] },
  { shortcode: "blobcat", url: blue, static_url: orange, category: "Blobs and other round creatures of the fediverse", aliases: [] },
  { shortcode: "blobfox", url: orange, static_url: orange, category: "Blobs and other round creatures of the fediverse", aliases: ["fox"] },
];

const account = {
  id: "0192f0c4-7a8e-7000-8000-000000000001",
  username: "alice",
  acct: "alice",
  display_name: "Alice",
  note: "",
  fields: [],
  emojis: [],
  url: "https://my.example/@alice",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

/** Answers the client API as a signed-in session that offers `emojis`. */
async function serveApi(page: Page): Promise<void> {
  const pageOrigin = new URL(test.info().project.use.baseURL ?? page.url()).origin;
  const cors = {
    "Access-Control-Allow-Origin": pageOrigin,
    "Access-Control-Allow-Credentials": "true",
    "Access-Control-Allow-Headers": "Authorization, Content-Type, Accept-Language, Idempotency-Key, X-My-Token-Refresh",
    "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE",
  };
  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: status < 400 ? "application/json" : "application/problem+json", headers: cors, body: JSON.stringify(body) });
  await page.route(`${pageOrigin}/**`, (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (!path.startsWith("/api/") && !path.startsWith("/oauth/")) return route.fallback();
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: cors });
    if (path === "/oauth/token/refresh") return json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow" });
    if (path === "/api/v1/accounts/verify_credentials") {
      return json(route, {
        ...account,
        discoverable: true,
        indexable: true,
        ai_opt_out: false,
        source: { note: "", fields: [] },
        posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
        permissions: [],
      });
    }
    if (path === "/api/v1/instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0-e2e", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, rules: [] });
    }
    if (path === "/api/v1/preferences") {
      return json(route, { locale: test.info().project.use.locale ?? "en", time_zone: "UTC", default_visibility: "public", default_post_language: "en", require_media_description: false, timeline_languages: [] });
    }
    if (path === "/api/v1/custom_emojis") return json(route, { items: emojis });
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
}

/** Opens the form (the sheet, or the side panel from Tailwind's xl) and returns it. */
async function openForm(page: Page, hasTouch: boolean) {
  if ((page.viewportSize()?.width ?? 0) < 1280) {
    const open = page.getByRole("button", { name: message("compose.open") });
    if (hasTouch) await open.tap();
    else await open.click();
  }
  const form = page.getByRole("form", { name: message("compose.title") });
  await expect(form.getByRole("textbox", { name: message("compose.text.label") })).toBeVisible();
  return form;
}

test.describe("the custom emoji picker of the new-post form", () => {
  test("has no axe violations and does not scroll horizontally while open", async ({ page, makeAxeBuilder, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    await form.getByRole("button", { name: message("emoji.picker.button") }).click();
    const picker = page.getByRole("dialog", { name: message("emoji.picker.title") });
    await expect(picker.getByRole("grid", { name: message("emoji.picker.grid") })).toBeVisible();
    await expect(picker.getByRole("searchbox", { name: message("emoji.picker.search") })).toBeFocused();

    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  test("picks an emoji with the keyboard alone and puts it into the text", async ({ page, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    const text = form.getByRole("textbox", { name: message("compose.text.label") });
    await text.fill("Hi");
    await form.getByRole("button", { name: message("emoji.picker.button") }).focus();
    await page.keyboard.press("Enter");

    const picker = page.getByRole("dialog", { name: message("emoji.picker.title") });
    await expect(picker.getByRole("searchbox", { name: message("emoji.picker.search") })).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(picker.getByRole("row", { name: ":wave:" })).toBeFocused();
    // The next emoji is at the end of the line in the writing direction.
    const rtl = (await page.evaluate(() => document.documentElement.dir)) === "rtl";
    await page.keyboard.press(rtl ? "ArrowLeft" : "ArrowRight");
    await expect(picker.getByRole("row", { name: ":thanks:" })).toBeFocused();
    await page.keyboard.press("Enter");

    await expect(picker).toBeHidden();
    await expect(text).toHaveValue("Hi :thanks: ");
    await expect(text).toBeFocused();
  });

  test("finds an emoji by another name", async ({ page, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    await form.getByRole("button", { name: message("emoji.picker.button") }).click();
    const picker = page.getByRole("dialog", { name: message("emoji.picker.title") });
    await picker.getByRole("searchbox", { name: message("emoji.picker.search") }).fill("fox");

    await expect(picker.getByRole("row", { name: ":blobfox:" })).toBeVisible();
    await expect(picker.getByRole("row", { name: ":wave:" })).toHaveCount(0);
    await expectNoHorizontalScroll(page);
  });
});
