// SPDX-License-Identifier: AGPL-3.0-only
// Storybook 10 for @my/ui (ADR-0027). The Vite config is ../vite.config.ts; the stories double as the
// accessibility and visual tests (../vitest.config.ts).
import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  framework: "@storybook/react-vite",
  stories: ["../src/**/*.stories.tsx"],
  addons: ["@storybook/addon-a11y", "@storybook/addon-vitest"],
  core: { disableTelemetry: true },
};

export default config;
