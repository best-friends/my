// SPDX-License-Identifier: AGPL-3.0-only
// The settings pages in every width × locale project (ADR-0027): the display language switches the whole app
// and <html lang dir> at once, form errors are tied to their fields, and the pages pass axe without
// scrolling sideways. `vite preview` has no API, so each test plays a signed-in user's server with
// page.route; names change with the locale, so elements are found by role, structure and autocomplete tokens.
import type { Page, Route } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, localeDir, test } from "./fixtures";

/** The signed-in user's account, as GET /accounts/verify_credentials answers. */
const viewer = {
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
  discoverable: true,
  indexable: true,
  source: { note: "", fields: [] },
  posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] },
  permissions: [],
};

/** Another account, as the lists of the settings show it. */
const bob = { ...viewer, id: "0192f0c4-7a8e-7000-8000-000000000002", username: "bob", acct: "bob@remote.example", display_name: "Bob" };

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(body) });
}

/**
 * Plays the server of a signed-in user whose preferences start in `locale`: the refresh endpoint resumes the
 * session, and the preferences remember their changes. Returns the changes the app sent.
 */
async function signedInServer(page: Page, locale: string) {
  let preferences = {
    locale,
    time_zone: "UTC",
    default_visibility: "public",
    default_post_language: "und",
    require_media_description: false,
    timeline_languages: [] as string[],
  };
  const changes: unknown[] = [];
  await page.route("**/oauth/token/refresh", (route) => json(route, { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace(/^\/api\/v1/, "");
    if (request.headers()["authorization"] !== "Bearer e2e-token" && path !== "/instance") return json(route, { type: "about:blank", title: "Unauthorized", status: 401 }, 401);
    switch (`${request.method()} ${path}`) {
      case "GET /instance":
        return json(route, { domain: "my.example", name: "my example", version: "0.0.0", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, rules: [] });
      case "GET /accounts/verify_credentials":
        return json(route, viewer);
      case "GET /preferences":
        return json(route, preferences);
      case "PATCH /preferences": {
        const change = request.postDataJSON() as Partial<typeof preferences>;
        changes.push(change);
        preferences = { ...preferences, ...change };
        return json(route, preferences);
      }
      case "GET /blocks":
        return json(route, { accounts: [bob] });
      case "GET /mutes":
        return json(route, { mutes: [{ account: bob, notifications: true }] });
      case "GET /follow_requests":
        return json(route, { accounts: [bob] });
      default:
        return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
    }
  });
  return changes;
}

/** The project's locale. */
function projectLocale(): string {
  return test.info().project.use.locale ?? "en";
}

const pages = ["/settings", "/settings/profile", "/settings/preferences", "/settings/follow-requests", "/settings/blocks", "/settings/mutes"] as const;

for (const path of pages) {
  test.describe(`page ${path}`, () => {
    test.beforeEach(async ({ page }) => {
      await signedInServer(page, projectLocale());
      await page.goto(path);
      await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
    });

    test("has no axe violations", async ({ makeAxeBuilder }) => {
      await expectNoAxeViolations(makeAxeBuilder);
    });

    test("does not scroll horizontally", async ({ page }) => {
      await expectNoHorizontalScroll(page);
    });
  });
}

test.describe("display language", () => {
  test("switches the app and <html lang dir> at once, and is saved", async ({ page }) => {
    const locale = projectLocale();
    const changes = await signedInServer(page, locale);
    await page.goto("/settings/preferences");
    const heading = page.getByRole("main").getByRole("heading", { level: 1 });
    await expect(heading).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("lang", locale);
    await expect(page.locator("html")).toHaveAttribute("dir", localeDir(locale));
    const before = await heading.textContent();

    // The form: the display language and time zone selects, the visibility radio group and the languages group.
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - heading [level=1]
      - link
      - paragraph
      - button
      - button
      - radiogroup:
        - radio [checked]
        - radio
        - radio
        - radio
        - radio
      - group:
        - list
    `);

    // The names of the display languages are in their own language, whatever the page's locale.
    const target = locale === "ja" ? { locale: "en", name: "English" } : { locale: "ja", name: "日本語" };
    const select = page.locator('main button[aria-haspopup="listbox"]').first();
    await select.click();
    await page.getByRole("option", { name: target.name }).click();

    await expect(page.locator("html")).toHaveAttribute("lang", target.locale);
    await expect(page.locator("html")).toHaveAttribute("dir", "ltr");
    await expect(heading).not.toHaveText(before ?? "");
    await expect(select).toContainText(target.name);
    expect(changes).toEqual([{ locale: target.locale }]);
  });
});

test.describe("profile form", () => {
  test("ties a validation error to its field and moves focus to it", async ({ page, makeAxeBuilder }) => {
    await signedInServer(page, projectLocale());
    await page.goto("/settings/profile");
    const displayName = page.locator('input[autocomplete="nickname"]');
    await expect(displayName).toBeVisible();

    await displayName.fill("a".repeat(31));
    await page.locator('main button[type="submit"]').click();

    await expect(displayName).toBeFocused();
    await expect(displayName).toHaveAttribute("aria-invalid", "true");
    const describedBy = (await displayName.getAttribute("aria-describedby")) ?? "";
    const descriptions = page.locator(describedBy.split(" ").map((id) => `[id="${id}"]`).join(", "));
    // The description and the error message.
    await expect(descriptions).toHaveCount(2);
    await expect(descriptions.last()).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("asks before leaving with unsaved changes", async ({ page, makeAxeBuilder }) => {
    await signedInServer(page, projectLocale());
    await page.goto("/settings/profile");
    const displayName = page.locator('input[autocomplete="nickname"]');
    await expect(displayName).toBeVisible();
    await displayName.fill("Alice Liddell");
    const settingsLink = page.getByRole("navigation").locator('a[href="/settings"]').first();

    await settingsLink.click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    // The question, what leaving does, and "Keep editing" (focused) before "Discard changes".
    await expect(dialog).toMatchAriaSnapshot(`
      - alertdialog:
        - heading [level=2]
        - paragraph
        - button
        - button
    `);
    await expect(dialog.getByRole("button").first()).toBeFocused();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(page).toHaveURL(/\/settings\/profile$/);
    await expect(displayName).toHaveValue("Alice Liddell");

    await settingsLink.click();
    await dialog.getByRole("button").last().click();
    await expect(page).toHaveURL(/\/settings$/);
  });
});

test.describe("time zone", () => {
  test("names the zones in the display language with their offset, and describes them by their IANA name", async ({ page, makeAxeBuilder }) => {
    await signedInServer(page, projectLocale());
    await page.goto("/settings/preferences");
    const select = page.locator('main button[aria-haspopup="listbox"]').nth(1);
    // The closed select shows the stored UTC's offset and CLDR name, without the description.
    await expect(select).toContainText("+00:00");

    await select.click();
    const tokyo = page.getByRole("option").filter({ hasText: "Asia/Tokyo" });
    await expect(tokyo).toHaveAccessibleName(/\+09:00/);
    await expect(tokyo).not.toHaveAccessibleName(/Asia\/Tokyo/);
    await expect(tokyo).toHaveAccessibleDescription("Asia/Tokyo");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});
