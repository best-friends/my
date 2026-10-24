// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152: the browser harness of the textarea autocomplete prototype, one project per engine.
// Run one engine at a time: `bunx vitest run -c vitest.spike.config.ts --project spike:chromium`.
// SPIKE_DSF=2 runs it with a device scale factor of 2 (the 200 % zoom case).
import { playwright } from "@vitest/browser-playwright";
import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

const engines = ["chromium", "firefox", "webkit"] as const;
const scale = Number(process.env["SPIKE_DSF"] ?? "1");

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      projects: engines.map((browser) => ({
        extends: true,
        test: {
          name: `spike:${browser}`,
          include: ["src/spike1152/**/*.browser.test.tsx"],
          setupFiles: ["./src/spike1152/setup.ts"],
          testTimeout: 60_000,
          fileParallelism: false,
          browser: {
            enabled: true,
            headless: true,
            provider: playwright({ contextOptions: { deviceScaleFactor: scale } }),
            instances: [{ browser }],
            viewport: { width: 1024, height: 900 },
          },
        },
      })),
    },
  }),
);
