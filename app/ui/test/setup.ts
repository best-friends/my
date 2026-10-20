// SPDX-License-Identifier: AGPL-3.0-only
// Runs before every component test of @my/ui (the `unit` project in ../vitest.config.ts): jest-dom
// matchers and DOM cleanup. The kit makes no requests, so there is no MSW server here.
import "@testing-library/jest-dom/vitest";
import { cleanup, configure } from "@testing-library/react";
import { destroyAnnouncer } from "react-aria/private/live-announcer/LiveAnnouncer";
import { afterEach } from "vitest";

// findBy and waitFor give up after this long, as in app/web/src/test/setup.ts (web-client.md T7): the wait
// is wall-clock time, which a busy machine stretches well past Testing Library's 1 s default.
configure({ asyncUtilTimeout: 10_000 });

// jsdom has no layout and does not implement scrolling; the Feed's virtualizer scrolls the window when
// articles are measured. A no-op keeps jsdom from logging "Not implemented" for every test.
window.scrollTo = () => undefined;
// Nor does jsdom draw: without the canvas package, getContext logs "Not implemented". A canvas without a context
// is what a browser without a 2D canvas gives, and Blurhash then draws nothing.
Object.defineProperty(HTMLCanvasElement.prototype, "getContext", { configurable: true, writable: true, value: () => null });

afterEach(() => {
  cleanup();
  // React Aria's live announcer delays an announcement made outside act by a 100 ms timer, which can fire once
  // jsdom is gone; destroying the announcer makes the timer do nothing (as in app/web/src/test/setup.ts).
  destroyAnnouncer();
});
