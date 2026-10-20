// SPDX-License-Identifier: AGPL-3.0-only
// Story tests of @my/ui (ADR-0027): every story runs in Chromium through Vitest browser mode and the
// Storybook Vitest addon. A story fails on any axe violation (parameters.a11y.test = 'error' in
// .storybook/preview.tsx) and on a visual difference from its screenshots (.storybook/vitest.setup.ts).
// There is one project per direction × theme, so contrast is checked in both themes and layout in RTL, and
// one per theme with the sample server theme (.storybook/sampleTheme.ts, ADR-0059) in en.
// Screenshot baselines are generated only in the pinned Playwright image (`make ui-snapshots-update`).
import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

/** en runs left to right and ar-XB right to left; ja and en-XA add nothing a snapshot would show. */
const locales = ["en", "ar-XB"] as const;
const themes = ["light", "dark"] as const;
/**
 * The story projects: every locale × theme with the default tokens, and the sample server theme in each theme.
 * A server theme changes tokens, not the direction, so en alone shows it.
 */
const storyProjects = [
  ...locales.flatMap((locale) => themes.map((theme) => ({ locale, theme, instanceTheme: "default" }))),
  ...themes.map((theme) => ({ locale: "en", theme, instanceTheme: "sample" })),
] as const;

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      projects: [
        // Component tests (src/**/*.test.tsx): Testing Library in jsdom, run by `make web-test`
        // (`bun run test:unit`). Like app/web, they never use the `node` environment (web-client.md T6).
        {
          extends: true,
          test: {
            name: "unit",
            environment: "jsdom",
            include: ["src/**/*.test.{ts,tsx}"],
            setupFiles: ["./test/setup.ts"],
            restoreMocks: true,
            // Room for several waits of asyncUtilTimeout (test/setup.ts) on a loaded machine (web-client.md T7).
            testTimeout: 30_000,
          },
        },
        ...storyProjects.map(({ locale, theme, instanceTheme }) => ({
          extends: true,
          plugins: [storybookTest({ configDir: ".storybook", initialGlobals: { locale, theme, instanceTheme } })],
          test: {
            name: instanceTheme === "default" ? `storybook:${locale}:${theme}` : `storybook:${locale}:${theme}:${instanceTheme}`,
            setupFiles: ["./.storybook/vitest.setup.ts"],
            browser: {
              enabled: true,
              headless: true,
              provider: playwright(),
              instances: [{ browser: "chromium" as const }],
            },
          },
        })),
      ],
    },
  }),
);
