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
  emojis: [],
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
 * What the played server is like beyond the defaults: the viewer's permissions, whether it sends email, and
 * whether the viewer has chosen their time zone (UTC) yet.
 */
type ServerOptions = { permissions?: string[]; sendsEmail?: boolean; timeZoneChosen?: boolean };

/**
 * Plays the server of a signed-in user whose preferences start in `locale`: the refresh endpoint resumes the
 * session, and the preferences remember their changes (across reloads too). Returns the changes the app sent.
 */
async function signedInServer(page: Page, locale: string, { permissions = [], sendsEmail = true, timeZoneChosen = true }: ServerOptions = {}) {
  let preferences = {
    locale,
    time_zone: "UTC",
    time_zone_chosen: timeZoneChosen,
    default_visibility: "public",
    default_post_language: "und",
    require_media_description: false,
    email_moderator_notifications: false,
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
        return json(route, { domain: "my.example", name: "my example", version: "0.0.0", source_url: "https://github.com/best-friends/my", registrations: { mode: "open" }, limits: { max_post_characters: 500 }, timelines: { local: true, federated: true }, sends_email: sendsEmail, rules: [] });
      case "GET /accounts/verify_credentials":
        return json(route, { ...viewer, permissions });
      case "GET /preferences":
        return json(route, preferences);
      case "PATCH /preferences": {
        const change = request.postDataJSON() as Partial<typeof preferences>;
        changes.push(change);
        preferences = { ...preferences, ...change, time_zone_chosen: preferences.time_zone_chosen || change.time_zone !== undefined };
        return json(route, preferences);
      }
      case "GET /blocks":
        return json(route, { accounts: [bob] });
      case "GET /mutes":
        return json(route, { mutes: [{ account: bob, notifications: true }] });
      case "GET /follow_requests":
        return json(route, { accounts: [bob] });
      case "GET /two_factor":
        return json(route, { enabled: false });
      case "POST /two_factor/totp/setup":
        return json(route, totpSetup);
      default:
        return json(route, { type: "about:blank", title: "Not Found", status: 404 }, 404);
    }
  });
  return changes;
}

// The browser is in the time zone of the played preferences (UTC), unless a test says otherwise.
test.use({ timezoneId: "UTC" });

/** The project's locale. */
function projectLocale(): string {
  return test.info().project.use.locale ?? "en";
}

const pages = [
  "/settings",
  "/settings/profile",
  "/settings/preferences",
  "/settings/follow-requests",
  "/settings/blocks",
  "/settings/mutes",
  "/settings/two-factor",
] as const;

/** A TOTP setup whose QR code is the three finder patterns of a 21-module code with its quiet zone. */
const totpSetup = {
  secret: "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP",
  uri: "otpauth://totp/my.example:alice@my.example?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP",
  qr_code: { size: 29, path: "M4 4h7v7h-7zM18 4h7v7h-7zM4 18h7v7h-7z" },
  setup_token: "e2e-setup",
  expires_at: "2026-10-20T00:15:00Z",
};

test("two-factor setup shows the QR code and the key without axe violations or horizontal scroll", async ({ page, makeAxeBuilder }) => {
  await signedInServer(page, projectLocale());
  await page.goto("/settings/two-factor");
  await page.getByRole("main").getByRole("button").first().click();
  await expect(page.getByRole("main").getByRole("img")).toBeVisible();
  await expect(page.getByRole("main").getByRole("textbox")).toBeVisible();
  await expectNoAxeViolations(makeAxeBuilder);
  await expectNoHorizontalScroll(page);
});

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

    // The form: the display language and time zone selects, the theme and visibility radio groups and the languages group.
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
      - radiogroup:
        - radio [checked]
        - radio
        - radio
        - radio
        - radio
      - group:
        - list
    `);
    // A viewer without moderator permissions has no moderator notifications to switch.
    await expect(page.getByRole("main").getByRole("switch")).toHaveCount(0);

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

test.describe("theme", () => {
  test("switches at once, passes axe in both themes, and stays after a reload without reaching the server", async ({ page, makeAxeBuilder }) => {
    await page.emulateMedia({ colorScheme: "light" });
    const changes = await signedInServer(page, projectLocale());
    await page.goto("/settings/preferences");
    // The theme is the first radio group: the OS setting (checked), light and dark.
    const theme = page.getByRole("main").getByRole("radiogroup").first();
    await expect(theme.getByRole("radio").first()).toBeChecked();
    const html = page.locator("html");
    await expect(html).not.toHaveAttribute("data-theme");

    // The APG radio group: the arrow keys move and check (Up is the previous one in both directions, and wraps).
    await theme.getByRole("radio").first().focus();
    await page.keyboard.press("ArrowUp");
    await expect(theme.getByRole("radio").nth(2)).toBeChecked();
    await expect(html).toHaveAttribute("data-theme", "dark");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await page.keyboard.press("ArrowUp");
    await expect(html).toHaveAttribute("data-theme", "light");
    await expectNoAxeViolations(makeAxeBuilder);

    await page.reload();
    await expect(page.getByRole("main").getByRole("radiogroup").first().getByRole("radio").nth(1)).toBeChecked();
    await expect(html).toHaveAttribute("data-theme", "light");
    expect(changes).toEqual([]);
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
    // Phones reach the settings from the menu's sheet (ADR-0039), which stays open while the question is asked.
    const nav = page.getByRole("navigation");
    let settingsLink = nav.locator('a[href="/settings"]').filter({ visible: true });
    if ((await settingsLink.count()) === 0) {
      await nav.getByRole("button").click();
      settingsLink = page.getByRole("dialog").locator('a[href="/settings"]');
    }

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

test.describe("moderator notifications by email", () => {
  test("a moderator switches them on, and they stay on after a reload", async ({ page, makeAxeBuilder }) => {
    const changes = await signedInServer(page, projectLocale(), { permissions: ["manage_reports"] });
    await page.goto("/settings/preferences");
    const toggle = page.getByRole("main").getByRole("switch");
    await expect(toggle).not.toBeChecked();
    // The section of the moderator notifications follows the languages group, under its own heading.
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - group:
        - list
      - region:
        - heading [level=2]
        - switch
    `);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    // Space toggles a switch (WAI-ARIA APG, Switch pattern); the input under the label takes the focus.
    await toggle.focus();
    await page.keyboard.press("Space");
    await expect(toggle).toBeChecked();
    await expect.poll(() => changes).toEqual([{ email_moderator_notifications: true }]);

    await page.reload();
    await expect(page.getByRole("main").getByRole("switch")).toBeChecked();
  });

  test("says why there is no switch when the server sends no email", async ({ page, makeAxeBuilder }) => {
    await signedInServer(page, projectLocale(), { permissions: ["manage_accounts"], sendsEmail: false });
    await page.goto("/settings/preferences");
    const section = page.getByRole("main").getByRole("region");
    await expect(section).toMatchAriaSnapshot(`
      - region:
        - heading [level=2]
        - paragraph
    `);
    await expect(section.getByRole("switch")).toHaveCount(0);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
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

test.describe("time zone of the device", () => {
  test.use({ timezoneId: "Asia/Tokyo" });

  test("a new account takes the browser's time zone", async ({ page, makeAxeBuilder }) => {
    const changes = await signedInServer(page, projectLocale(), { timeZoneChosen: false });
    await page.goto("/settings/preferences");

    await expect.poll(() => changes).toEqual([{ time_zone: "Asia/Tokyo" }]);
    const select = page.locator('main button[aria-haspopup="listbox"]').nth(1);
    await expect(select).toContainText("+09:00");
    // The zone is the browser's, so no note tells of a difference: the two selects are the only buttons.
    await expect(page.getByRole("main").getByRole("button")).toHaveCount(2);
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("tells that the chosen zone is not the browser's, and switches to it from the keyboard", async ({ page, makeAxeBuilder }) => {
    const changes = await signedInServer(page, projectLocale());
    await page.goto("/settings/preferences");
    const select = page.locator('main button[aria-haspopup="listbox"]').nth(1);
    await expect(select).toContainText("+00:00");
    // The display language and time zone selects, then the note of the browser's zone with its button.
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - paragraph
      - button
      - button
      - paragraph: /\\+09:00/
      - button
      - radiogroup
    `);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    const useDevice = page.getByRole("main").getByRole("button").nth(2);
    await useDevice.focus();
    await page.keyboard.press("Enter");

    await expect.poll(() => changes).toEqual([{ time_zone: "Asia/Tokyo" }]);
    await expect(select).toContainText("+09:00");
    await expect(select).toBeFocused();
    await expect(page.getByRole("main").getByRole("button")).toHaveCount(2);
  });
});
