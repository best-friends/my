// SPDX-License-Identifier: AGPL-3.0-only
// Vite 8 build of the web client (ADR-0025). React Compiler runs through @rolldown/plugin-babel
// with the preset from @vitejs/plugin-react, as react.dev/learn/react-compiler/installation describes.
import { fileURLToPath } from "node:url";
import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import { defineConfig, type Plugin, searchForWorkspaceRoot } from "vite";
import { VitePWA } from "vite-plugin-pwa";

/** The shared message catalogs at the repository root (ADR-0009). */
const localesDir = fileURLToPath(new URL("../../locales", import.meta.url));

/**
 * Fails `vite build` when React Compiler compiled no component or hook, so a broken compiler
 * setup cannot ship silently (ADR-0025). Functions the compiler bails out on are reported as
 * warnings; the react-hooks lint rules are what reject them.
 */
function reactCompilerCheck() {
  let compiled = 0;
  const problems: string[] = [];
  const logger = {
    logEvent(filename: string | null, event: { kind: string; fnName?: string | null; reason?: string }) {
      if (event.kind === "CompileSuccess") compiled++;
      else if (event.kind === "CompileError" || event.kind === "PipelineError") {
        problems.push(`${filename ?? "<unknown>"}: React Compiler skipped ${event.fnName ?? "a function"}`);
      }
    },
  };
  const plugin: Plugin = {
    name: "my:react-compiler-check",
    apply: "build",
    buildEnd(error) {
      if (error) return;
      for (const p of problems) this.warn(p);
      if (compiled === 0) this.error("React Compiler compiled no function; check the babel preset in vite.config.ts");
      this.info(`React Compiler compiled ${compiled} function(s)`);
    },
  };
  return { logger, plugin };
}

const compilerCheck = reactCompilerCheck();

export default defineConfig({
  // Go serves the HTML and builds its <script> / <link> tags from the manifest (ADR-0028).
  base: "./",
  build: {
    manifest: true,
    // Emit every asset as a file: the CSP of ADR-0019 §9 has no data: source.
    assetsInlineLimit: 0,
    rolldownOptions: {
      // The SPA, and the stylesheet of the pages the Go server renders without JavaScript
      // (src/server-pages.css, ADR-0030). Go finds both by these manifest keys.
      input: {
        index: fileURLToPath(new URL("index.html", import.meta.url)),
        "server-pages": fileURLToPath(new URL("src/server-pages.css", import.meta.url)),
      },
    },
  },
  server: {
    // `my serve -dev-vite-origin http://localhost:5173` loads the client from here (CONTRIBUTING.md),
    // so the URLs of assets must point at this server, not at the page's origin.
    port: 5173,
    strictPort: true,
    origin: "http://localhost:5173",
    // Catalogs are imported from locales/, outside the workspace root.
    fs: { allow: [searchForWorkspaceRoot(process.cwd()), localesDir] },
  },
  plugins: [
    // File-based routes from src/routes; the generated src/routeTree.gen.ts is committed so that tsc
    // runs before a build. Each route's components go into their own chunk. TanStack Router's docs
    // require this plugin before the React plugin.
    tanstackRouter({ target: "react", autoCodeSplitting: true }),
    react(),
    babel({ presets: [reactCompilerPreset({ logger: compilerCheck.logger })] }),
    compilerCheck.plugin,
    tailwindcss(),
    // The Service Worker (ADR-0028): sw/sw.ts with the precache manifest injected, built into dist/sw.js
    // (https://vite-pwa-org.netlify.app/guide/inject-manifest.html). Go generates the Web App Manifest and
    // the HTML, so the plugin writes neither and injects no registration script; src/lib/pwa.ts registers
    // the worker and asks before activating an update.
    VitePWA({
      strategies: "injectManifest",
      srcDir: "sw",
      filename: "sw.ts",
      manifest: false,
      injectRegister: false,
      // Absolute, unlike the build's relative base: the worker lives at /sw.js and controls the whole
      // origin, whichever page registers it.
      base: "/",
      scope: "/",
      injectManifest: {
        // A classic script, which every browser registers.
        rollupFormat: "iife",
        globPatterns: ["**/*.{js,css,svg,png,woff2}"],
        // index.html is the stand-in shell of `vite preview` (the worker precaches the server's shell at /
        // instead), and the stylesheet of the server-rendered pages is never loaded by the app.
        globIgnores: ["index.html", "sw.js", "assets/server-pages-*.css"],
      },
    }),
  ],
});
