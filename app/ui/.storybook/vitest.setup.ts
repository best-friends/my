// SPDX-License-Identifier: AGPL-3.0-only
// Visual snapshots of every story (ADR-0027). After a story has rendered and passed axe, its frame
// is captured at each container width; the project fixes the direction and theme (../vitest.config.ts).
// Baselines live in src/__screenshots__/ and are generated only in the pinned Playwright image
// (`make ui-snapshots-update`), because fonts and anti-aliasing differ between machines.
import { afterEach, expect } from "vitest";
import { page } from "vitest/browser";

/** Container widths in CSS px: a phone column, a tablet column and a desktop main column. */
const widths = [320, 640, 1024] as const;
/** The viewport @storybook/addon-vitest gives a story without a viewport parameter. */
const viewportWidth = 1200;
const viewportHeight = 900;

afterEach(async (context) => {
  // Only story tests carry a composed story; it stays mounted until the next one renders.
  const { story } = context as typeof context & { story?: { id: string; globals: Record<string, unknown> } };
  if (story === undefined || context.task.result?.state === "fail") return;
  const frame = document.querySelector<HTMLElement>("[data-story-frame]");
  if (frame === null) throw new Error(`story ${story.id} rendered no [data-story-frame]; is the preview decorator missing?`);
  const { locale, theme } = story.globals;
  for (const width of widths) {
    frame.style.inlineSize = `${width}px`;
    // The tester page paints only what fits its viewport, so it grows to the whole story. The width stays
    // the 1200 px that the Storybook addon sets; the container width is what the components answer to.
    const bottom = Math.ceil(frame.getBoundingClientRect().bottom + window.scrollY);
    await page.viewport(viewportWidth, Math.max(viewportHeight, bottom + 16));
    await expect.element(page.elementLocator(frame)).toMatchScreenshot(`${story.id}-${String(locale)}-${String(theme)}-${width}`);
  }
  frame.style.inlineSize = "";
});
