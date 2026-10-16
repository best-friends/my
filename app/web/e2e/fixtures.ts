// SPDX-License-Identifier: AGPL-3.0-only
// Shared E2E fixtures and assertions (ADR-0027). Specs import `test` and `expect` from here, not from
// @playwright/test, so every spec gets the same axe configuration.
import AxeBuilder from "@axe-core/playwright";
import { test as base, expect, type Page } from "@playwright/test";
import { previewOrigin } from "./server-origin.ts";

/** The axe rule tags for WCAG 2.2 AA (ADR-0027, web-client.md Y1). */
export const axeTags = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"] as const;

type Fixtures = {
  /** Returns an AxeBuilder for the page, limited to `axeTags`; chain `.include()` / `.exclude()` to scope it. */
  makeAxeBuilder: () => AxeBuilder;
  /**
   * Serves the HTML documents of `vite preview` (the client) with the project's locale in
   * <html lang dir>, as `my serve` writes the instance's default locale into its shell (ADR-0028);
   * `vite preview` serves index.html as it is. The runtime config's `apiOrigin` becomes the preview's own
   * origin, which the run picks (server-origin.ts), so that the client's API and OAuth requests stay on it,
   * where specs can answer them with `page.route`. The build's asset links are relative (Vite's base "./"); they
   * become links from the root, as `my serve` writes them, which pages below the root (/settings/profile) need.
   * Pages of a real `my serve` are left alone.
   */
  shellLocale: string;
};

/** Pseudo-locale and right-to-left languages the projects use; the rest are left to right. */
const rtlLocales: ReadonlySet<string> = new Set(["ar-XB"]);

/** The text direction of a project locale. */
export const localeDir = (locale: string): "ltr" | "rtl" => (rtlLocales.has(locale) ? "rtl" : "ltr");

export const test = base.extend<Fixtures>({
  makeAxeBuilder: async ({ page }, use) => {
    await use(() => new AxeBuilder({ page }).withTags([...axeTags]));
  },
  shellLocale: [
    async ({ page, locale }, use) => {
      const lang = locale ?? "en";
      // A real server (E2E_BASE_URL, or the `my serve` of the server-rendered pages) writes lang and dir itself.
      if (process.env["E2E_BASE_URL"] !== undefined) {
        await use(lang);
        return;
      }
      await page.route((url) => url.origin === previewOrigin, async (route) => {
        if (route.request().resourceType() !== "document") {
          await route.fallback();
          return;
        }
        const response = await route.fetch();
        const body = (await response.text())
          .replace(/<html [^>]*>/, `<html lang="${lang}" dir="${localeDir(lang)}">`)
          .replace(/"apiOrigin":"[^"]*"/, `"apiOrigin":"${previewOrigin}"`)
          .replaceAll('="./assets/', '="/assets/');
        await route.fulfill({ response, body });
      });
      await use(lang);
    },
    { auto: true },
  ],
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
