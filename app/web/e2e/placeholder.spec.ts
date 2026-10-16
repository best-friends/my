// SPDX-License-Identifier: AGPL-3.0-only
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

test.describe("placeholder page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("./");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("exposes a main landmark with one level-one heading", async ({ page }) => {
    // The heading name is left out so the snapshot holds in every locale.
    await expect(page.locator("body")).toMatchAriaSnapshot(`
      - main:
        - heading [level=1]
    `);
  });
});
