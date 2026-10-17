// SPDX-License-Identifier: AGPL-3.0-only
// A page whose API fails, in every width × locale project (web-client.md G16): the page keeps its heading and
// document title, and its content gives way to what happened, why, and "Try again". `vite preview` has no client
// API, so the spec answers the local timeline itself: with a server error first, then with an empty page.
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";

/** A reason long enough to wrap at every width. */
const reason = "The database did not answer in time while loading the local timeline of this server; try again later.";

test.describe("a page whose API fails", () => {
  /** Whether the local timeline answers with its (empty) first page instead of failing. */
  let answers = false;

  test.beforeEach(async ({ page }) => {
    answers = false;
    await page.route("**/api/v1/timelines/local*", async (route) => {
      if (answers) {
        await route.fulfill({ json: { items: [] } });
        return;
      }
      await route.fulfill({
        status: 500,
        contentType: "application/problem+json",
        json: { type: "about:blank", title: "Internal Server Error", status: 500, detail: reason },
      });
    });
    await page.goto("/local");
    await expect(page.getByRole("main").getByText(reason)).toBeVisible();
  });

  test("keeps the page's heading and title, and says what happened, why and what to do", async ({ page }) => {
    const main = page.getByRole("main");
    const heading = (await main.getByRole("heading", { level: 1 }).textContent()) ?? "";
    expect(heading).not.toBe("");
    expect(await page.title()).toContain(heading);
    // A failure the page opened with is read, not announced.
    await expect(main.getByRole("alert")).toHaveCount(0);
    await expect(main).toMatchAriaSnapshot(`
      - main:
        - heading [level=1]
        - heading [level=2]
        - paragraph
        - button
    `);
  });

  test("has no axe violations", async ({ makeAxeBuilder }) => {
    await expectNoAxeViolations(makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("loads the page on retry and focuses its heading", async ({ page }) => {
    answers = true;
    await page.getByRole("main").getByRole("button").click();

    await expect(page.getByRole("main").getByText(reason)).toHaveCount(0);
    await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeFocused();
  });
});
