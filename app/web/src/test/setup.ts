// SPDX-License-Identifier: AGPL-3.0-only
// Runs before every Vitest file (vitest.config.ts): jest-dom matchers, DOM cleanup and the MSW server.
// The network is mocked only through MSW (web-client.md T3); a request that no handler matches fails the test.
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, beforeEach, vi } from "vitest";
import { server } from "./msw/server";

beforeAll(() => {
  server.listen({ onUnhandledFrame: "error" });
});

beforeEach(() => {
  // jsdom has no layout, so it does not implement scrolling; the router restores the scroll position
  // on every navigation. restoreMocks (vitest.config.ts) removes the stub after each test.
  vi.spyOn(window, "scrollTo").mockImplementation(() => undefined);
});

afterEach(() => {
  cleanup();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});
