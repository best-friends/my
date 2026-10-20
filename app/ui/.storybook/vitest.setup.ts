// SPDX-License-Identifier: AGPL-3.0-only
// Visual snapshots of every story (ADR-0027) not tagged `no-screenshot`. After a story has rendered and passed axe, its frame
// is captured at each container width; the project fixes the direction, the theme and the instance theme
// (../vitest.config.ts).
// Baselines live in src/__screenshots__/ and are generated only in the pinned Playwright image
// (`make ui-snapshots-update`), because fonts and anti-aliasing differ between machines.
// The capture runs with prefers-reduced-motion: reduce, and first checks that nothing in the frame
// still animates then: every animation sits under motion-safe: (web-client.md D6), and a spinner
// cannot keep the screenshot from settling. Stories render and run play functions without it.
// The Playwright provider types cdp() with Chromium's DevTools protocol.
import type {} from "@vitest/browser-playwright";
import { afterEach, expect } from "vitest";
import { cdp, page } from "vitest/browser";

/** Container widths in CSS px: a phone column, a tablet column and a desktop main column. */
const widths = [320, 640, 1024] as const;
/** The viewport @storybook/addon-vitest gives a story without a viewport parameter. */
const viewportWidth = 1200;
const viewportHeight = 900;

/** Sets Chromium's prefers-reduced-motion to reduce, or back to the browser's default. */
async function emulateReducedMotion(reduce: boolean) {
  await cdp().send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-reduced-motion", value: reduce ? "reduce" : "" }] });
}

/** Names a running animation for the failure message: the CSS animation or property and its element. */
function describeAnimation(animation: Animation): string {
  const name = animation instanceof CSSAnimation ? animation.animationName : animation instanceof CSSTransition ? animation.transitionProperty : animation.id;
  const target = animation.effect instanceof KeyframeEffect ? animation.effect.target : null;
  return `${name} on ${target?.outerHTML.slice(0, 120) ?? "an unknown element"}`;
}

afterEach(async (context) => {
  // Only story tests carry a composed story; it stays mounted until the next one renders.
  const { story } = context as typeof context & { story?: { id: string; tags: readonly string[]; globals: Record<string, unknown> } };
  if (story === undefined || context.task.result?.state === "fail") return;
  // A story tagged `no-screenshot` exists for its play function (a long, virtualized list); it still
  // renders and passes axe, and the same component's other stories carry its screenshots.
  if (story.tags.includes("no-screenshot")) return;
  const frame = document.querySelector<HTMLElement>("[data-story-frame]");
  if (frame === null) throw new Error(`story ${story.id} rendered no [data-story-frame]; is the preview decorator missing?`);
  const { locale, theme, instanceTheme } = story.globals;
  // The default theme keeps the names without a suffix; a server theme adds its name.
  const suffix = typeof instanceTheme === "string" && instanceTheme !== "default" ? `-${instanceTheme}` : "";
  await emulateReducedMotion(true);
  try {
    // Transitions already in flight (such as the color change when the preview applies the theme)
    // run to their end even after the media change, so wait for them; none can start under reduce.
    const inFlight = frame.getAnimations({ subtree: true }).filter((animation) => animation instanceof CSSTransition);
    await Promise.allSettled(inFlight.map((transition) => transition.finished));
    const running = frame
      .getAnimations({ subtree: true })
      .filter((animation) => animation.playState === "running")
      .map(describeAnimation);
    expect(running, `story ${story.id} still animates with prefers-reduced-motion: reduce`).toEqual([]);
    for (const width of widths) {
      frame.style.inlineSize = `${width}px`;
      // Open popovers and tooltips are positioned against their trigger, and React Aria repositions them on
      // window resize; the trigger moves with the frame width (in RTL it sits at the inline end).
      window.dispatchEvent(new Event("resize"));
      // The tester page paints only what fits its viewport, so it grows to the whole story. The width stays
      // the 1200 px that the Storybook addon sets; the container width is what the components answer to.
      const bottom = Math.ceil(frame.getBoundingClientRect().bottom + window.scrollY);
      await page.viewport(viewportWidth, Math.max(viewportHeight, bottom + 16));
      await expect.element(page.elementLocator(frame)).toMatchScreenshot(`${story.id}-${String(locale)}-${String(theme)}${suffix}-${width}`);
    }
  } finally {
    frame.style.inlineSize = "";
    await emulateReducedMotion(false);
  }
});
