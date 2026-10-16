// SPDX-License-Identifier: AGPL-3.0-only
// Vitest runs component tests on Node.js in jsdom (ADR-0025, ADR-0027). It extends vite.config.ts, so the
// React Compiler preset transforms the code under test as it does in the build. The preset applies to the
// client environment only, which is why tests never use the `node` environment (web-client.md T6).
import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      include: ["src/**/*.test.{ts,tsx}"],
      setupFiles: ["./src/test/setup.ts"],
      restoreMocks: true,
      unstubGlobals: true,
    },
  }),
);
