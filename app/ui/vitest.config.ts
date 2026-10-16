// SPDX-License-Identifier: AGPL-3.0-only
// Story tests of @my/ui (ADR-0027): every story runs in Chromium through Vitest browser mode and the
// Storybook Vitest addon. A story fails on any axe violation (parameters.a11y.test = 'error' in
// .storybook/preview.tsx) and on a visual difference from its screenshots (.storybook/vitest.setup.ts).
// There is one project per direction × theme, so contrast is checked in both themes and layout in RTL.
// Screenshot baselines are generated only in the pinned Playwright image (`make ui-snapshots-update`).
import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

/** en runs left to right and ar-XB right to left; ja and en-XA add nothing a snapshot would show. */
const locales = ["en", "ar-XB"] as const;
const themes = ["light", "dark"] as const;

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      projects: locales.flatMap((locale) =>
        themes.map((theme) => ({
          extends: true,
          plugins: [storybookTest({ configDir: ".storybook", initialGlobals: { locale, theme } })],
          test: {
            name: `storybook:${locale}:${theme}`,
            setupFiles: ["./.storybook/vitest.setup.ts"],
            browser: {
              enabled: true,
              headless: true,
              provider: playwright(),
              instances: [{ browser: "chromium" as const }],
            },
          },
        })),
      ),
    },
  }),
);
