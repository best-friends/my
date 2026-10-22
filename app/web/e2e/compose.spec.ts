// SPDX-License-Identifier: AGPL-3.0-only
// The new-post form in every width × locale project (ADR-0027): a sheet
// there (xl). `vite preview` has no API, so the spec answers the client API and the token refresh itself
// (Playwright's request routing), as a signed-in session. The client calls the runtime config's apiOrigin,
// the origin of the page (fixtures.ts); the answers carry CORS headers all the same, as a separate API
// origin needs them (ADR-0016). Names come from the
// project's catalog, so the spec finds the same elements in every locale.
import { readFileSync } from "node:fs";
import type { Page, Route } from "@playwright/test";
import { createIntl } from "react-intl";
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

/**
 * A regular expression source that matches a name ending with `text`: the language select's button, named by
 * the chosen language and then its label. The pseudo-locales' brackets are escaped.
 */
function endingWith(text: string): string {
  return `${text.replaceAll(/[\\^$.*+?()[\]{}|/]/g, "\\$&")}$`;
}

/** A node of an ARIA snapshot template as a quoted YAML scalar, since the names have colons ("Visibility: Public"). */
function ariaNode(node: string): string {
  return `'${node.replaceAll("'", "''")}'`;
}

/** The client message `key` in the project's locale, formatted with `values` as the app formats it (ICU). */
function formatted(key: string, values: Record<string, string | number>): string {
  const locale = test.info().project.use.locale ?? "en";
  return createIntl({ locale, messages: { [key]: message(key) } }).formatMessage({ id: key }, values);
}

/** An account of another server that the instance resolves (WebFinger) when asked to, and that does not follow Alice. */
const carol = {
  id: "0192f0c4-7a8e-7000-8000-000000000003",
  username: "carol",
  acct: "carol@remote.example",
  domain: "remote.example",
  uri: "https://remote.example/users/carol",
  display_name: "Carol",
  note: "",
  fields: [],
  emojis: [],
  url: "https://remote.example/@carol",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

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
        ai_opt_out: false,
        source: { note: "", fields: [] },
        posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
        permissions: [],
      });
    if (path === "/api/v1/instance") {
      return json(route, { domain: "my.example", name: "my example", version: "0.0.0-e2e", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500, require_media_description: false }, timelines: { local: true, federated: true }, rules: [] });
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
        emojis: [],
        replies_count: 0,
        boosts_count: 0,
        favourites_count: 0,
        reactions: [],
      });
    }
    // Carol is found only by resolving her handle, as the server resolves the mentions of a post.
    const query = new URL(request.url()).searchParams;
    if (path === "/api/v1/accounts/lookup" && query.get("acct") === carol.acct && query.get("resolve") === "true") return json(route, carol);
    if (path === "/api/v1/accounts/relationships") {
      return json(
        route,
        query.getAll("id").map((id) => ({
          id,
          following: false,
          requested: false,
          followed_by: false,
          requested_by: false,
          show_boosts: true,
          notify: false,
          blocking: false,
          muting: false,
          muting_notifications: false,
        })),
      );
    }
    return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
  });
  return posted;
}

/** Opens the form from the navigation's button (the tab bar's or the rail's) and returns the form. */
async function openForm(page: Page, hasTouch: boolean) {
  const open = page.getByRole("button", { name: message("compose.open") });
  if (hasTouch) await open.tap();
  else await open.click();
  await expect(page.getByRole("dialog", { name: message("compose.title") })).toBeVisible();
  const form = page.getByRole("form", { name: message("compose.title") });
  await expect(form.getByRole("textbox", { name: message("compose.text.label") })).toBeVisible();
  return form;
}

test.describe("the new-post form", () => {
  test("has no axe violations and does not scroll horizontally, with the visibility choices open too", async ({ page, makeAxeBuilder, hasTouch }) => {
    const posted = await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    await form.getByRole("button", { name: message("compose.warning.toggle") }).click();
    await expect(form.getByRole("textbox", { name: message("compose.warning.label") })).toBeVisible();

    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await form.getByRole("button", { name: message("compose.visibility.button", { visibility: message("compose.visibility.public") }) }).click();
    await expect(page.getByRole("radiogroup", { name: message("compose.visibility.title") })).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    expect(posted).toEqual([]);
  });

  test("exposes the text, then the tools with their current values, then the submit button", async ({ page, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);

    await expect(form).toMatchAriaSnapshot(`
      - form:
        - ${ariaNode(`textbox ${JSON.stringify(message("compose.text.label"))}`)}
        - ${ariaNode(`button ${JSON.stringify(message("compose.media.add"))}`)}
        - ${ariaNode(`button ${JSON.stringify(message("compose.poll.toggle"))} [pressed=false]`)}
        - ${ariaNode(`button ${JSON.stringify(message("compose.warning.toggle"))} [pressed=false]`)}
        - ${ariaNode(`button ${JSON.stringify(message("compose.visibility.button", { visibility: message("compose.visibility.public") }))}`)}
        - ${ariaNode(`button /${endingWith(message("compose.language.label"))}/`)}
        - ${ariaNode(`button ${JSON.stringify(message("compose.submit"))} [disabled]`)}
    `);
  });

  test("keeps its tools level, and the submit button on the first screen of a 320 × 568 sheet", async ({ page, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);

    // Every tool is as tall as the others at the project's width (no field with a label beside one without).
    const tools = [
      form.getByRole("button", { name: message("compose.media.add") }),
      form.getByRole("button", { name: message("compose.poll.toggle") }),
      form.getByRole("button", { name: message("compose.warning.toggle") }),
      form.getByRole("button", { name: message("compose.visibility.button", { visibility: message("compose.visibility.public") }) }),
      form.getByRole("button", { name: new RegExp(endingWith(message("compose.language.label"))) }),
    ];
    const heights = await Promise.all(tools.map(async (tool) => (await boxOf(tool)).height));
    expect(new Set(heights).size).toBe(1);

    await page.setViewportSize({ width: 320, height: 568 });
    await expect(form.getByRole("button", { name: message("compose.submit") })).toBeInViewport({ ratio: 1 });
    await expectNoHorizontalScroll(page);
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

  test("posts a poll from its fields, which have no axe violations and do not scroll horizontally (FR-112)", async ({ page, makeAxeBuilder, hasTouch }) => {
    const posted = await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);
    await form.getByRole("textbox", { name: message("compose.text.label") }).fill("Which one?");
    await form.getByRole("button", { name: message("compose.poll.toggle") }).click();
    const poll = form.getByRole("group", { name: message("compose.poll.legend") });
    await poll.getByRole("textbox", { name: formatted("compose.poll.option", { number: "1" }) }).fill("Tea");
    await poll.getByRole("textbox", { name: formatted("compose.poll.option", { number: "2" }) }).fill("Coffee");
    await poll.getByRole("button", { name: message("compose.poll.option.add") }).click();
    await expect(poll.getByRole("textbox", { name: formatted("compose.poll.option", { number: "3" }) })).toBeVisible();
    await expect(form.getByRole("button", { name: message("compose.media.add") })).toBeDisabled();

    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await poll.getByRole("button", { name: formatted("compose.poll.option.remove", { number: "3" }) }).click();
    // The first post fails (serveApi), the second goes through.
    await form.getByRole("button", { name: message("compose.submit") }).click();
    await expect(form.getByText(message("compose.failed", { reason: "Service Unavailable" }))).toBeVisible();
    await form.getByRole("button", { name: message("compose.submit") }).click();
    await expect(page.getByRole("region", { name: message("shell.toast.region") }).getByText(message("compose.posted"))).toBeVisible();
    expect(posted[1]?.body).toEqual({
      text: "Which one?",
      visibility: "public",
      language: "en",
      poll: { options: ["Tea", "Coffee"], expires_in: 86400 },
    });
  });

  test("warns that a local post does not reach a mentioned account of another server that does not follow the viewer", async ({ page, makeAxeBuilder, hasTouch }) => {
    await serveApi(page);
    await page.goto("/");
    const form = await openForm(page, hasTouch);

    await form.getByRole("button", { name: message("compose.visibility.button", { visibility: message("compose.visibility.public") }) }).click();
    const choices = page.getByRole("radiogroup", { name: message("compose.visibility.title") });
    await choices.getByText(message("compose.visibility.local"), { exact: true }).click();
    await expect(choices.getByRole("radio", { name: message("compose.visibility.local"), exact: true })).toBeChecked();
    await page.getByRole("button", { name: message("compose.visibility.done") }).click();
    await form.getByRole("textbox", { name: message("compose.text.label") }).fill("Hi @carol@remote.example");

    await expect(form.getByText(formatted("compose.mention.unreached", { count: 1, accounts: "@carol@remote.example" }))).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

/** Answers the home timeline with one page of Alice's posts. */
async function serveHomeTimeline(page: Page): Promise<void> {
  const post = (n: number) => ({
    id: `0192f0c4-7a8e-7000-8000-0000000001${String(n).padStart(2, "0")}`,
    uri: `https://my.example/users/alice/statuses/${String(n)}`,
    account,
    content: `<p>Post number ${String(n)} on the home timeline.</p>`,
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: "2026-10-16T00:00:00Z",
    media_attachments: [],
    mentions: [],
    tags: [],
    emojis: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
    reactions: [],
  });
  // Registered after serveApi's route, so Playwright asks this one first.
  await page.route("**/api/v1/timelines/home*", (route) => route.fulfill({ json: { items: Array.from({ length: 6 }, (_, i) => post(i + 1)) } }));
}

type Box = { x: number; y: number; width: number; height: number };

function overlaps(a: Box, b: Box): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

async function boxOf(locator: ReturnType<Page["getByRole"]>): Promise<Box> {
  const box = await locator.boundingBox();
  if (box === null) throw new Error("the element is not visible");
  return box;
}

test.describe("the tab bar's new-post button (web-client.md G4)", () => {
  test.skip(({ viewport }) => (viewport?.width ?? 0) >= 768, "from md the button is in the rail");

  test.beforeEach(async ({ page }) => {
    await serveApi(page);
    await serveHomeTimeline(page);
  });

  test("leaves the actions of the timeline's last post uncovered at the end of the page", async ({ page }) => {
    await page.goto("/home");
    const fab = page.getByRole("button", { name: message("compose.open") });
    await expect(fab).toMatchAriaSnapshot(`- button "${message("compose.open")}"`);
    const last = page.getByRole("article").last();
    await expect(last).toBeVisible();
    await page.evaluate(() => {
      window.scrollTo(0, document.documentElement.scrollHeight);
    });

    const navBox = await boxOf(page.getByRole("navigation", { name: message("shell.nav.label") }));
    const actions = await last.getByRole("button").all();
    expect(actions.length).toBeGreaterThan(0);
    for (const action of actions) {
      const box = await boxOf(action);
      expect(overlaps(box, navBox), "a post action under the tab bar").toBe(false);
    }
  });

  test("and the tab bar take at most half of a 320 × 225 viewport (400% zoom)", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 225 });
    await page.goto("/home");
    const fab = page.getByRole("button", { name: message("compose.open") });
    await expect(fab).toBeVisible();
    await expectNoHorizontalScroll(page);

    // The coral button is inside the bar, so the bar is all that is fixed to the screen.
    const fabBox = await boxOf(fab);
    const navBox = await boxOf(page.getByRole("navigation", { name: message("shell.nav.label") }));
    expect(fabBox.y).toBeGreaterThanOrEqual(navBox.y);
    expect(225 - navBox.y).toBeLessThanOrEqual(225 / 2);
  });

  test("the tab bar and the offline bar take at most half of a 320 × 225 viewport (400% zoom) while offline", async ({
    page,
    context,
    makeAxeBuilder,
  }) => {
    await page.setViewportSize({ width: 320, height: 225 });
    await page.goto("/home");
    await expect(page.getByRole("article").last()).toBeVisible();
    await context.setOffline(true);
    const bar = page.getByRole("status").filter({ hasText: message("shell.offline") });
    await expect(bar).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    // Into the page, as the user reads it: what stays in place there covers the posts.
    await page.evaluate(() => {
      window.scrollTo(0, document.documentElement.scrollHeight / 2);
    });
    const viewport = 225;
    /** How much of the viewport's height the element takes. */
    const shown = (box: Box) => Math.max(0, Math.min(box.y + box.height, viewport) - Math.max(box.y, 0));
    const navBox = await boxOf(page.getByRole("navigation", { name: message("shell.nav.label") }));
    const barBox = await boxOf(bar);
    const bottom = viewport - navBox.y;
    expect(shown(barBox) + bottom).toBeLessThanOrEqual(viewport / 2);
    await context.setOffline(false);
  });

  test("gets focus back when the sheet it opened from the keyboard closes", async ({ page }) => {
    await page.goto("/home");
    const fab = page.getByRole("button", { name: message("compose.open") });
    await fab.focus();
    await page.keyboard.press("Enter");
    const sheet = page.getByRole("dialog", { name: message("compose.title") });
    await expect(sheet).toBeVisible();

    await page.keyboard.press("Escape");

    await expect(sheet).toBeHidden();
    await expect(fab).toBeFocused();
  });
});
