// SPDX-License-Identifier: AGPL-3.0-only
// What the app does so that its libraries stay inside the shell's CSP. jsdom enforces no CSP, so these check
// the setup; e2e/csp.spec.ts checks the built app under the real policy of `my serve`.
import { afterEach, expect, test } from "vitest";
import { z } from "zod";
import { conformToContentSecurityPolicy, reactAriaPressableStyleID } from "./csp";

afterEach(() => {
  document.getElementById(reactAriaPressableStyleID)?.remove();
});

test("Zod, as the app imports it, has its eval-based JIT turned off", () => {
  expect(z.config().jitless).toBe(true);
});

test("adds one inert stand-in for React Aria's pressable <style>, however often it runs", () => {
  conformToContentSecurityPolicy(document);
  conformToContentSecurityPolicy(document);

  const markers = document.querySelectorAll(`#${reactAriaPressableStyleID}`);
  expect(markers).toHaveLength(1);
  expect(markers[0]?.tagName).toBe("TEMPLATE");
  expect(markers[0]?.parentElement).toBe(document.head);
});
