// SPDX-License-Identifier: AGPL-3.0-only
// Runs before every Vitest file (vitest.config.ts): jest-dom matchers, DOM cleanup and the MSW server.
// The network is mocked only through MSW (web-client.md T3); a request that no handler matches fails the test.
import "@testing-library/jest-dom/vitest";
import { cleanup, configure } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, beforeEach, vi } from "vitest";
import { server } from "./msw/server";

// findBy and waitFor give up after this long (web-client.md T7). The wait is wall-clock time, which a busy
// machine stretches: Testing Library's 1 s default timed out in `make check` while Go and PostgreSQL ran
// alongside. A test that passes never waits this long; the limit only bounds how long a broken one hangs.
// testTimeout in vitest.config.ts leaves room for several such waits.
configure({ asyncUtilTimeout: 10_000 });

beforeAll(() => {
  server.listen({ onUnhandledFrame: "error" });
  // jsdom has no Element.scrollIntoView; a page that scrolls an element into view (a post's page) calls it.
  Object.defineProperty(Element.prototype, "scrollIntoView", { configurable: true, writable: true, value: () => undefined });
});

beforeEach(() => {
  // jsdom has no layout, so it does not implement scrolling; the router restores the scroll position
  // on every navigation. restoreMocks (vitest.config.ts) removes the stub after each test.
  vi.spyOn(window, "scrollTo").mockImplementation(() => undefined);
});

afterEach(() => {
  cleanup();
  server.resetHandlers();
  // What the app keeps for the next start (the instance's timelines) must not leak into the next test.
  localStorage.clear();
  // The tests share one document; a theme chosen in one (lib/theme) must not color the next.
  delete document.documentElement.dataset["theme"];
});

afterAll(() => {
  server.close();
});
