// SPDX-License-Identifier: AGPL-3.0-only
// The parts of lib/pwa that run without a Service Worker: clearing caches at sign-out and reloading after
// a failed chunk load. jsdom has neither Cache Storage nor a location that reloads, so both are stand-ins.
import { describe, expect, test, vi } from "vitest";
import { clearCaches, reloadOnPreloadError } from "./pwa";
import { mediaCacheName } from "./sw-protocol";

describe("clearCaches", () => {
  test("deletes every cache but the precache", async () => {
    const names = new Set(["my-precache-v2-http://localhost/", mediaCacheName, "other"]);
    vi.stubGlobal("caches", {
      keys: () => Promise.resolve([...names]),
      delete: (name: string) => Promise.resolve(names.delete(name)),
    });

    await clearCaches();

    expect([...names]).toEqual(["my-precache-v2-http://localhost/"]);
  });
});

describe("reloadOnPreloadError", () => {
  function fakeWindow() {
    const target = new EventTarget();
    const reload = vi.fn();
    const window = Object.assign(target, { sessionStorage: globalThis.sessionStorage, location: { reload } });
    return { window: window as unknown as Window, reload };
  }

  function preloadError(): Event {
    return new Event("vite:preloadError", { cancelable: true });
  }

  test("reloads once, and lets a second failure soon after through", () => {
    sessionStorage.clear();
    const { window, reload } = fakeWindow();
    const stop = reloadOnPreloadError(window);

    const first = preloadError();
    window.dispatchEvent(first);
    expect(reload).toHaveBeenCalledOnce();
    expect(first.defaultPrevented).toBe(true);

    const second = preloadError();
    window.dispatchEvent(second);
    expect(reload).toHaveBeenCalledOnce();
    expect(second.defaultPrevented).toBe(false);

    stop();
  });

  test("stops listening when cleaned up", () => {
    sessionStorage.clear();
    const { window, reload } = fakeWindow();
    reloadOnPreloadError(window)();

    window.dispatchEvent(preloadError());
    expect(reload).not.toHaveBeenCalled();
  });
});
