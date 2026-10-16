// SPDX-License-Identifier: AGPL-3.0-only
// Runs before every component test of @my/ui (the `unit` project in ../vitest.config.ts): jest-dom
// matchers and DOM cleanup. The kit makes no requests, so there is no MSW server here.
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
});
