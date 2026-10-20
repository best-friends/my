// SPDX-License-Identifier: AGPL-3.0-only
// The app shell in every width × locale project (ADR-0027): the layout of each page, its landmarks, and
// what a navigation does to focus and the document title (web-client.md Y5). Names change with the
// locale, so elements are found by role and position, and texts are compared with each other.
import type { Page } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, imageVariants, test } from "./fixtures";

/** Every page of the shell, in the order of the navigation. */
const pages = ["/home", "/local", "/notifications", "/search", "/profile", "/settings"] as const;

/** The pages that phones reach from the menu's sheet instead of the tab bar (ADR-0039). */
const menuPages: ReadonlySet<string> = new Set(["/profile", "/settings"]);

/** Whether the project's viewport has the phones' tab bar rather than the rail (Tailwind's md). */
function hasTabBar(page: Page): boolean {
  return (page.viewportSize()?.width ?? 0) < 768;
}

/** The items of the navigation that the project's width shows; the others are hidden (display: none). */
function shownItems(page: Page) {
  return page.getByRole("navigation").getByRole("listitem").filter({ visible: true });
}

/** The instance name in index.html's runtime config. */
const instanceName = "my example";

/** `text` as a literal in a regular expression. */
function escapeRegExp(text: string): string {
  return text.replaceAll(/[.*+?^${}()|[\]\\]/g, String.raw`\$&`);
}

for (const path of pages) {
  test.describe(`page ${path}`, () => {
    test.beforeEach(async ({ page }) => {
      await page.goto(path);
      await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
    });

    test("has no axe violations", async ({ makeAxeBuilder }) => {
      await expectNoAxeViolations(makeAxeBuilder);
    });

    test("does not scroll horizontally", async ({ page }) => {
      await expectNoHorizontalScroll(page);
    });

    test("is titled after its heading and marks its navigation item as current", async ({ page }) => {
      const heading = await page.getByRole("heading", { level: 1 }).textContent();
      expect(heading).toBeTruthy();
      await expect(page).toHaveTitle(new RegExp(`${escapeRegExp(heading ?? "")}.*${escapeRegExp(instanceName)}`));
      const current = page.locator('nav a[aria-current="page"]').filter({ visible: true });
      if (hasTabBar(page) && menuPages.has(path)) {
        // On phones the page is in the menu's sheet, which marks it; no item of the tab bar is current.
        await expect(current).toHaveCount(0);
        await page.getByRole("navigation").getByRole("button").click();
        const sheet = page.getByRole("dialog");
        await expect(sheet.locator(`a[href="${path}"]`)).toHaveAttribute("aria-current", "page");
        await expect(sheet.locator('a[aria-current="page"]')).toHaveCount(1);
      } else {
        // On phones /local is the tab bar's Timelines, which links to it.
        await expect(current).toHaveCount(1);
        await expect(current).toHaveAttribute("href", path);
      }
    });
  });
}

// The sign-in page is outside the navigation; vite preview has no OAuth endpoints, so no session resumes.
test.describe("page /login", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("offers the one button to the server's sign-in screen", async ({ page }) => {
    await expect(page.getByRole("main").getByRole("button")).toHaveCount(1);
  });
});

test.describe("layout", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/home");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("exposes the skip link, the navigation, and the main landmark with one level-one heading", async ({ page }) => {
    // Phones: Home, Timelines, Notifications, Search and the Menu button (ADR-0039). From md: Home, Local,
    // Notifications, Search, Profile and Settings (the instance answers nothing here, so no Federated).
    const items = hasTabBar(page) ? ["link", "link", "link", "link", "button"] : ["link", "link", "link", "link", "link", "link"];
    await expect(page.locator("body")).toMatchAriaSnapshot(`
      - link
      - navigation:
        - list:
          - /children: equal
${items.map((role) => `          - listitem:\n            - ${role}`).join("\n")}
      - main:
        - heading [level=1]
    `);
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
  });

  test("puts the navigation on the bottom edge of phones and on the inline-start edge of larger screens", async ({ page }) => {
    const viewport = page.viewportSize();
    const box = await page.getByRole("navigation").boundingBox();
    expect(viewport).not.toBeNull();
    expect(box).not.toBeNull();
    if (viewport === null || box === null) return;
    if (viewport.width < 768) {
      expect(box.y + box.height).toBeCloseTo(viewport.height, 0);
      expect(box.width).toBeCloseTo(viewport.width, 0);
    } else {
      expect(box.height).toBeCloseTo(viewport.height, 0);
      const rtl = (await page.locator("html").getAttribute("dir")) === "rtl";
      expect(rtl ? viewport.width - (box.x + box.width) : box.x).toBeCloseTo(0, 0);
    }
  });

  test("shows the icon of every navigation item at one size", async ({ page }) => {
    const icons = shownItems(page).locator("svg");
    await expect(icons).toHaveCount(hasTabBar(page) ? 5 : 6);
    for (const icon of await icons.all()) {
      expect(await icon.boundingBox()).toMatchObject({ width: 24, height: 24 });
    }
  });

  test("shows the column about the server only on wide screens", async ({ page }) => {
    const wide = (page.viewportSize()?.width ?? 0) >= 1280;
    await expect(page.getByRole("complementary")).toHaveCount(wide ? 1 : 0);
  });
});

test.describe("the sidebar of desktops", () => {
  /** A long name, which the sidebar cuts to one line; the link is still named by all of it. */
  const displayName = "Bartholomew Montgomery-Fitzwilliam the Third";
  const avatar = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><circle cx="24" cy="24" r="24" fill="#8f4f6d"/></svg>')}`;

  test.beforeEach(async ({ page }) => {
    // vite preview has no API: answer the session refresh and the viewer's account as a signed-in user's server.
    const json = (body: unknown) => ({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
    await page.route("**/oauth/token/refresh", (route) => route.fulfill(json({ access_token: "e2e-token", token_type: "Bearer", expires_in: 3600 })));
    await page.route("**/api/v1/accounts/verify_credentials", (route) =>
      route.fulfill(
        json({
          id: "0192f0c4-7a8e-7000-8000-000000000001",
          username: "alice",
          acct: "alice",
          domain: "my.example",
          uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000001",
          display_name: displayName,
          note: "",
          fields: [],
          url: "https://my.example/@alice",
          locked: false,
          bot: false,
          created_at: "2026-10-01T00:00:00Z",
          avatar: imageVariants(avatar),
        }),
      ),
    );
    await page.route("**/api/v1/notifications/marker", (route) => route.fulfill(json({ unread_count: 0 })));
    await page.goto("/search");
    await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("names the server above the list and the signed-in account below it, from lg only", async ({ page, makeAxeBuilder }) => {
    const viewport = page.viewportSize();
    const navigation = page.getByRole("navigation");
    const viewer = navigation.locator('a[href="/@alice"]');
    if ((viewport?.width ?? 0) < 1024) {
      await expect(navigation.getByText(instanceName)).toBeHidden();
      await expect(viewer).toBeHidden();
      return;
    }
    await expect(viewer).toBeVisible();
    // The list's links keep their order; the server's name comes before the list and the account after it.
    await expect(navigation).toMatchAriaSnapshot(`
      - navigation:
        - paragraph: ${instanceName}
        - list:
          - /children: equal
${Array.from({ length: 6 }, () => "          - listitem:\n            - link").join("\n")}
        - link "${displayName} @alice"
    `);
    await expect(viewer).not.toHaveAttribute("aria-current");
    // The account is always in view, at the bottom of the sidebar, inside it.
    const box = await viewer.boundingBox();
    const nav = await navigation.boundingBox();
    expect(box).not.toBeNull();
    expect(nav).not.toBeNull();
    if (viewport === null || box === null || nav === null) return;
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
    expect(box.y + box.height).toBeGreaterThan(viewport.height - 80);
    expect(box.x).toBeGreaterThanOrEqual(nav.x);
    expect(box.x + box.width).toBeLessThanOrEqual(nav.x + nav.width);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });

  // The sidebar scrolls vertically (overflow-y: auto, so overflow-x is auto too): content wider than its box,
  // by a single pixel, shows a horizontal scrollbar at its bottom. Desktop widths beyond the matrix are checked
  // from one project per locale.
  test("scrolls sideways neither itself nor the page at desktop widths", async ({ page }) => {
    test.skip(page.viewportSize()?.width !== 1440, "resizes the viewport itself; runs once per locale");
    const navigation = page.getByRole("navigation");
    for (const width of [1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await expect(navigation.locator('a[href="/@alice"]')).toBeVisible();
      await expectNoHorizontalScroll(page);
      // The sidebar and every element in it that can scroll sideways: none is wider inside than its box. Names cut
      // to one line (overflow: hidden) are wider inside on purpose and never scroll.
      const overflowing = await navigation.evaluate((nav) =>
        [nav, ...nav.querySelectorAll<HTMLElement>("*")]
          .filter((element) => element === nav || ["auto", "scroll"].includes(getComputedStyle(element).overflowX))
          .filter((element) => element.scrollWidth > element.clientWidth)
          .map((element) => `${element.tagName.toLowerCase()} scrollWidth ${element.scrollWidth}px > clientWidth ${element.clientWidth}px`),
      );
      expect(overflowing, `at ${width}px`).toEqual([]);
    }
  });
});

test.describe("navigation", () => {
  test("moves focus to the new page's heading and retitles the document", async ({ page, hasTouch }) => {
    await page.goto("/home");
    const firstHeading = await page.getByRole("heading", { level: 1 }).textContent();
    await expect(page.getByRole("heading", { level: 1 })).not.toBeFocused();

    // The tab bar's Timelines on phones, Local from md: both link to the local timeline.
    const local = page.getByRole("navigation").locator('a[href="/local"]').filter({ visible: true });
    if (hasTouch) await local.tap();
    else await local.click();

    await expect(page).toHaveURL(/\/local$/);
    const heading = page.getByRole("heading", { level: 1 });
    await expect(heading).toBeFocused();
    const text = (await heading.textContent()) ?? "";
    expect(text).not.toBe(firstHeading);
    await expect(page).toHaveTitle(new RegExp(`${escapeRegExp(text)}.*${escapeRegExp(instanceName)}`));
  });

  test("works from the keyboard, and back and forward keep focus on the page's heading", async ({ page }) => {
    await page.goto("/home");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    // The skip link comes first and moves focus to the main landmark.
    await page.keyboard.press("Tab");
    await expect(page.getByRole("link").first()).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("main")).toBeFocused();

    await page.getByRole("navigation").locator('a[href="/search"]').focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/search$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();

    await page.goBack();
    await expect(page).toHaveURL(/\/home$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();
    await expect(page.getByRole("navigation").locator('a[href="/home"]')).toHaveAttribute("aria-current", "page");
  });
});

test.describe("the menu of the phones' tab bar", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(!hasTabBar(page), "the rail from md has every item and no menu");
    await page.goto("/home");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("opens a sheet with the profile and the settings from the keyboard, and Esc gives focus back", async ({ page, makeAxeBuilder }) => {
    const menu = page.getByRole("navigation").getByRole("button");
    await menu.focus();
    await page.keyboard.press("Enter");

    const sheet = page.getByRole("dialog");
    await expect(sheet).toMatchAriaSnapshot(`
      - dialog:
        - heading [level=2]
        - button
        - list:
          - listitem:
            - link
          - listitem:
            - link
        - radiogroup:
          - radio [checked]
          - radio
          - radio
    `);
    await expect(sheet.getByRole("link")).toHaveText([/./, /./]);
    await expect(sheet.locator('a[href="/profile"]')).toBeVisible();
    await expect(sheet.locator('a[href="/settings"]')).toBeVisible();
    await expect(sheet.locator('a[aria-current="page"]')).toHaveCount(0);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);

    await page.keyboard.press("Escape");
    await expect(sheet).toBeHidden();
    await expect(menu).toBeFocused();
  });

  test("closes when one of its links is followed, and focus moves to the new page's heading", async ({ page, hasTouch }) => {
    const menu = page.getByRole("navigation").getByRole("button");
    if (hasTouch) await menu.tap();
    else await menu.click();
    const settings = page.getByRole("dialog").locator('a[href="/settings"]');
    if (hasTouch) await settings.tap();
    else await settings.click();

    await expect(page).toHaveURL(/\/settings$/);
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();
  });
});

test("an unknown address shows the not-found page inside the layout", async ({ page, makeAxeBuilder }) => {
  await page.goto("/no-such-page");
  await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByRole("navigation")).toBeVisible();
  await expect(page.getByRole("main").getByRole("link")).toHaveAttribute("href", "/home");
  await expectNoAxeViolations(makeAxeBuilder);
});
