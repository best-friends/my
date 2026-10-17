// SPDX-License-Identifier: AGPL-3.0-only
// The new-post form in every width × locale project (ADR-0027): a sheet below 1280 px, a side panel from
// there (xl). `vite preview` has no API, so the spec answers the client API and the token refresh itself
// (Playwright's request routing), as a signed-in session. The client calls the runtime config's apiOrigin,
// the origin of the page (fixtures.ts); the answers carry CORS headers all the same, as a separate API
// origin needs them (ADR-0016). Names come from the
// project's catalog, so the spec finds the same elements in every locale.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

type Catalog = Record<string, { defaultMessage: string }>;

/** The client message `key` in the project's locale, with its `{name}` arguments replaced. */
function message(key: string, values: Record<string, string> = {}): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/client.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${key} in ${locale}`);
  return text.replaceAll(/\{(\w+)\}/g, (_, name: string) => values[name] ?? `{${name}}`);
}

const account = {
  id: "0192f0c4-7a8e-7000-8000-000000000001",
  username: "alice",
  acct: "alice",
  display_name: "Alice",
  note: "",
  fields: [],
  url: "https://my.example/@alice",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

type Posted = { body: Record<string, unknown>; idempotencyKey: string | undefined };

/** Answers the client API as a signed-in session; the first post fails with a 503 and the next ones succeed. */
async function serveApi(page: Page): Promise<Posted[]> {
  const posted: Posted[] = [];
  const pageOrigin = new URL(test.info().project.use.baseURL ?? page.url()).origin;
  const apiOrigin = pageOrigin;
  const cors = {
    "Access-Control-Allow-Origin": pageOrigin,
    "Access-Control-Allow-Credentials": "true",
    "Access-Control-Allow-Headers": "Authorization, Content-Type, Accept-Language, Idempotency-Key, X-My-Token-Refresh",
    "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE",
  };
  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: status < 400 ? "application/json" : "application/problem+json", headers: cors, body: JSON.stringify(body) });
  await page.route(`${apiOrigin}/**`, async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    // The page and its assets come from the same origin.
    if (!path.startsWith("/api/") && !path.startsWith("/oauth/")) return route.fallback();
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: cors });
    if (path === "/oauth/token/refresh") return json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow" });
    if (path === "/api/v1/accounts/verify_credentials") return json(route, {
        ...account,
        discoverable: true,
        indexable: true,
        source: { note: "", fields: [] },
        posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
        permissions: [],
      });
    if (path === "/api/v1/instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0-e2e", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 } });
    }
    if (path === "/api/v1/preferences") {
      // The user's locale is the project's: the app displays itself in the signed-in user's locale.
      return json(route, { locale: test.info().project.use.locale ?? "en", time_zone: "UTC", default_visibility: "public", default_post_language: "en", require_media_description: false, timeline_languages: [] });
    }
    if (path === "/api/v1/statuses" && request.method() === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      posted.push({ body, idempotencyKey: await request.headerValue("Idempotency-Key") ?? undefined });
      if (posted.length === 1) return json(route, { type: "about:blank", title: "Service Unavailable", status: 503 }, 503);
      return json(route, {
        id: "0192f0c4-7a8e-7000-8000-0000000000a1",
        uri: "https://my.example/users/alice/statuses/1",
        account,
        content: `<p>${String(body["text"])}</p>`,
        spoiler_text: "",
        sensitive: false,
        visibility: body["visibility"],
        language: body["language"],
        created_at: "2026-10-16T00:00:00Z",
        media_attachments: [],
        mentions: [],
        tags: [],
        replies_count: 0,
        boosts_count: 0,
        favourites_count: 0,
      });
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return posted;
}

/** Whether the project's viewport shows the form as a side panel (Tailwind's xl). */
function isWide(page: Page): boolean {
  return (page.viewportSize()?.width ?? 0) >= 1280;
}

/** Opens the form (the sheet, or the panel that is always there) and returns the form. */
async function openForm(page: Page, hasTouch: boolean) {
  if (!isWide(page)) {
    const open = page.getByRole("button", { name: message("compose.open") });
    if (hasTouch) await open.tap();
    else await open.click();
    await expect(page.getByRole("dialog", { name: message("compose.title") })).toBeVisible();
  } else {
    await expect(page.getByRole("region", { name: message("compose.title") })).toBeVisible();
  }
  const form = page.getByRole("form", { name: message("compose.title") });
  await expect(form.getByRole("textbox", { name: message("compose.text.label") })).toBeVisible();
  return form;
}

test.describe("the new-post form", () => {
  test("has no axe violations and does not scroll horizontally, with the visibility choices open too", async ({ page, makeAxeBuilder, hasTouch }) => {
    const posted = await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    // React Aria hides the native checkbox under its drawing; the label is what the user presses.
    await form.getByText(message("compose.warning.toggle")).click();
    await expect(form.getByRole("textbox", { name: message("compose.warning.label") })).toBeVisible();

    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await form.getByRole("button", { name: message("compose.visibility.button", { visibility: message("compose.visibility.public") }) }).click();
    await expect(page.getByRole("radiogroup", { name: message("compose.visibility.title") })).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    expect(posted).toEqual([]);
  });

  test("exposes the fields in order", async ({ page, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);

    await expect(form).toMatchAriaSnapshot(`
      - form:
        - checkbox
        - textbox
        - button
        - button
        - button
    `);
  });

  test("keeps the draft when posting fails and posts it again with the same Idempotency-Key", async ({ page, hasTouch }) => {
    const posted = await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    const text = form.getByRole("textbox", { name: message("compose.text.label") });

    await text.fill("Hello from the E2E");
    await form.getByRole("button", { name: message("compose.submit") }).click();

    await expect(form.getByText(message("compose.failed", { reason: "Service Unavailable" }))).toBeVisible();
    await expect(text).toHaveValue("Hello from the E2E");

    await form.getByRole("button", { name: message("compose.submit") }).click();

    await expect(page.getByRole("region", { name: message("shell.toast.region") }).getByText(message("compose.posted"))).toBeVisible();
    expect(posted).toHaveLength(2);
    expect(posted[1]?.idempotencyKey).toBeTruthy();
    expect(posted[1]?.idempotencyKey).toBe(posted[0]?.idempotencyKey);
    expect(posted[1]?.body).toEqual({ text: "Hello from the E2E", visibility: "public", language: "en" });
    await expectNoHorizontalScroll(page);
  });
});
