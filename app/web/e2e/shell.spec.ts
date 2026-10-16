// SPDX-License-Identifier: AGPL-3.0-only
// The app shell in every width × locale project (ADR-0027): the layout of each page, its landmarks, and
// what a navigation does to focus and the document title (web-client.md Y5). Names change with the
// locale, so elements are found by role and position, and texts are compared with each other.
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

/** Every page of the shell, in the order of the navigation. */
const pages = ["/home", "/local", "/notifications", "/profile", "/settings"] as const;

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
      const links = page.getByRole("navigation").getByRole("link");
      await expect(links.nth(pages.indexOf(path))).toHaveAttribute("aria-current", "page");
      await expect(page.locator('nav a[aria-current="page"]')).toHaveCount(1);
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

  test("exposes the skip link, the navigation with five links, and the main landmark with one level-one heading", async ({ page }) => {
    await expect(page.locator("body")).toMatchAriaSnapshot(`
      - link
      - navigation:
        - list:
          - /children: equal
          - listitem:
            - link
          - listitem:
            - link
          - listitem:
            - link
          - listitem:
            - link
          - listitem:
            - link
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

  test("shows the column about the server only on wide screens", async ({ page }) => {
    const wide = (page.viewportSize()?.width ?? 0) >= 1280;
    await expect(page.getByRole("complementary")).toHaveCount(wide ? 1 : 0);
  });
});

test.describe("navigation", () => {
  test("moves focus to the new page's heading and retitles the document", async ({ page, hasTouch }) => {
    await page.goto("/home");
    const firstHeading = await page.getByRole("heading", { level: 1 }).textContent();
    await expect(page.getByRole("heading", { level: 1 })).not.toBeFocused();

    const local = page.getByRole("navigation").getByRole("link").nth(1);
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

    await page.getByRole("navigation").getByRole("link").nth(4).focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/settings$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();

    await page.goBack();
    await expect(page).toHaveURL(/\/home$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();
    await expect(page.getByRole("navigation").getByRole("link").nth(0)).toHaveAttribute("aria-current", "page");
  });
});

test("an unknown address shows the not-found page inside the layout", async ({ page, makeAxeBuilder }) => {
  await page.goto("/no-such-page");
  await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByRole("navigation")).toBeVisible();
  await expect(page.getByRole("main").getByRole("link")).toHaveAttribute("href", "/home");
  await expectNoAxeViolations(makeAxeBuilder);
});
