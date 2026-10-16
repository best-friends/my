// SPDX-License-Identifier: AGPL-3.0-only
// Runs before every component test of @my/ui (the `unit` project in ../vitest.config.ts): jest-dom
// matchers and DOM cleanup. The kit makes no requests, so there is no MSW server here.
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// jsdom has no layout and does not implement scrolling; the Feed's virtualizer scrolls the window when
// articles are measured. A no-op keeps jsdom from logging "Not implemented" for every test.
window.scrollTo = () => undefined;

afterEach(() => {
  cleanup();
});
