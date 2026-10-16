// SPDX-License-Identifier: AGPL-3.0-only
// Playwright E2E for the web client (ADR-0015, ADR-0027). Every test runs in one project per
// viewport width × locale. E2E_MATRIX=full runs all of them (nightly); otherwise only the per-PR subset
// runs: every locale at the narrowest and widest width. The client's tests run against `vite preview` of
// the built app (`make web-e2e` builds it first), or against E2E_BASE_URL when it is set. The tests of the
// server-rendered pages (e2e/auth-pages.spec.ts) and of the main flow (e2e/main-flow.spec.ts) run against a real `my serve` on SQLite that
// e2e/server.ts starts from bin/my with the pseudo-locales enabled, or against E2E_SERVER_URL when it is
// set (a server started with `my serve -pseudo-locales`). Both servers listen on free ports picked per
// run (E2E_PREVIEW_PORT and E2E_SERVER_PORT override them; e2e/server-origin.ts), and a run never reuses a
// server it did not start, so runs in several worktrees on one machine stay apart.
import { defineConfig, devices, type Project } from "@playwright/test";
import { previewOrigin, previewPort, serverPort } from "./e2e/server-origin.ts";

/** Viewport widths in CSS px (ADR-0027). */
const widths = [320, 390, 768, 1024, 1440] as const;
/** The source locale, Japanese and the two pseudo-locales (ADR-0009, ADR-0015). */
const locales = ["en", "ja", "en-XA", "ar-XB"] as const;
/** Widths every PR runs: 320 px is where horizontal scrolling appears first (web-client.md D3). */
const prWidths: ReadonlySet<number> = new Set([320, 1440]);

const fullMatrix = process.env["E2E_MATRIX"] === "full";
const ci = process.env["CI"] !== undefined;
const port = previewPort;
const externalBaseURL = process.env["E2E_BASE_URL"];
const externalServer = process.env["E2E_SERVER_URL"] !== undefined;

const projects: Project[] = widths
  .filter((width) => fullMatrix || prWidths.has(width))
  .flatMap((width) =>
    locales.map((locale) => {
      // Widths below the tablet breakpoint emulate a phone: touch, a coarse pointer and the mobile viewport.
      const phone = width < 768;
      return {
        name: `${locale}-${width}`,
        use: {
          ...devices["Desktop Chrome"],
          viewport: { width, height: phone ? 800 : 900 },
          isMobile: phone,
          hasTouch: phone,
          locale,
        },
      };
    }),
  );

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: ci,
  retries: ci ? 1 : 0,
  ...(ci ? { workers: 2 } : {}),
  reporter: ci ? [["github"], ["html", { open: "never" }]] : [["list"]],
  use: {
    baseURL: externalBaseURL ?? previewOrigin,
    trace: "retain-on-failure",
    // A Service Worker answers navigations before page.route() sees them, which would bypass the locale
    // fixture (e2e/fixtures.ts). Only e2e/pwa.spec.ts, which tests the worker, allows it.
    serviceWorkers: "block",
  },
  projects,
  webServer: [
    ...(externalBaseURL === undefined
      ? [
          {
            command: `bun run preview --host 127.0.0.1 --port ${String(port)} --strictPort`,
            url: `http://127.0.0.1:${String(port)}`,
            // A server already on the port belongs to someone else (another worktree's build, say), so the
            // run fails rather than test it; E2E_BASE_URL is the way to test a server this run does not start.
            reuseExistingServer: false,
          },
        ]
      : []),
    ...(externalServer
      ? []
      : [
          {
            // A fresh database every run: the specs depend on its seed and on rate limits not yet used up.
            command: "node e2e/server.ts",
            url: `http://127.0.0.1:${String(serverPort)}/readyz`,
            reuseExistingServer: false,
            gracefulShutdown: { signal: "SIGTERM", timeout: 5000 },
          } as const,
        ]),
  ],
});
