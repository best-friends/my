// SPDX-License-Identifier: AGPL-3.0-only
// Shared E2E fixtures and assertions (ADR-0027). Specs import `test` and `expect` from here, not from
// @playwright/test, so every spec gets the same axe configuration.
import AxeBuilder from "@axe-core/playwright";
import { test as base, expect, type Page } from "@playwright/test";

/** The axe rule tags for WCAG 2.2 AA (ADR-0027, web-client.md Y1). */
export const axeTags = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"] as const;

type Fixtures = {
  /** Returns an AxeBuilder for the page, limited to `axeTags`; chain `.include()` / `.exclude()` to scope it. */
  makeAxeBuilder: () => AxeBuilder;
};

export const test = base.extend<Fixtures>({
  makeAxeBuilder: async ({ page }, use) => {
    await use(() => new AxeBuilder({ page }).withTags([...axeTags]));
  },
});

export { expect };

/** One axe violation reduced to what identifies it: the rule and the elements it fails on. */
type ViolationFingerprint = { rule: string; targets: string[] };

/**
 * Scans the page with axe and asserts that it has no violations. The assertion compares fingerprints
 * (rule ID and target selectors) rather than the full axe result, so a failure lists exactly which rule
 * failed on which elements and stays readable.
 */
export async function expectNoAxeViolations(makeAxeBuilder: () => AxeBuilder): Promise<void> {
  const results = await makeAxeBuilder().analyze();
  const fingerprints: ViolationFingerprint[] = results.violations.map((violation) => ({
    rule: violation.id,
    targets: violation.nodes.map((node) => node.target.join(" ")),
  }));
  expect(fingerprints, "axe violations (rule and target selectors)").toEqual([]);
}

/** Asserts that the page does not scroll horizontally at the current viewport width (web-client.md D3). */
export async function expectNoHorizontalScroll(page: Page): Promise<void> {
  const { scrollWidth, clientWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(scrollWidth, `scrollWidth ${scrollWidth}px exceeds clientWidth ${clientWidth}px`).toBeLessThanOrEqual(
    clientWidth,
  );
}
