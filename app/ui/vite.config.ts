// SPDX-License-Identifier: AGPL-3.0-only
// Vite config of the kit's own tooling: Storybook (@storybook/react-vite loads this file) and the
// story tests (vitest.config.ts extends it). Components compile as they do in app/web: React Compiler
// through @rolldown/plugin-babel with the preset from @vitejs/plugin-react, and Tailwind.
import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), babel({ presets: [reactCompilerPreset()] }), tailwindcss()],
});
