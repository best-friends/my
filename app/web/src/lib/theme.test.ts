// SPDX-License-Identifier: AGPL-3.0-only
import { act, renderHook } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
// eslint-disable-next-line boundaries/no-unknown-dependencies -- public/theme.js is the shell's classic script, outside the app's elements; the test reads its text.
import themeScript from "../../public/theme.js?raw";
import { themeKey } from "./storage";
import { applyStoredTheme, documentTheme, setTheme, useTheme } from "./theme";

const html = document.documentElement;

/** What lib/storage keeps for `value`. */
const stored = (value: string) => JSON.stringify({ v: themeKey.version, value });

describe("theme", () => {
  test("a choice sets <html data-theme> and the storage; the OS setting removes both", () => {
    setTheme("dark");
    expect(html.dataset["theme"]).toBe("dark");
    expect(localStorage.getItem(themeKey.name)).toBe(stored("dark"));
    expect(documentTheme(document)).toBe("dark");

    setTheme("system");
    expect(html).not.toHaveAttribute("data-theme");
    expect(localStorage.getItem(themeKey.name)).toBeNull();
    expect(documentTheme(document)).toBe("system");
  });

  test("the stored theme applies at start; an unknown value is the OS setting", () => {
    localStorage.setItem(themeKey.name, stored("light"));
    applyStoredTheme(document);
    expect(html.dataset["theme"]).toBe("light");

    localStorage.setItem(themeKey.name, stored("sepia"));
    applyStoredTheme(document);
    expect(html).not.toHaveAttribute("data-theme");
  });

  test("public/theme.js reads the key and the format of lib/storage", () => {
    expect(themeScript).toContain(`localStorage.getItem(${JSON.stringify(themeKey.name)})`);
    expect(themeScript).toContain(`stored.v === ${String(themeKey.version)}`);
  });

  test("blocked storage still switches the page", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    setTheme("light");
    expect(html.dataset["theme"]).toBe("light");
  });

  test("useTheme renders again on a choice here and on one made in another tab", () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current).toBe("system");

    act(() => {
      setTheme("dark");
    });
    expect(result.current).toBe("dark");

    // Another tab of the origin chose light: its storage event reaches this document only.
    act(() => {
      localStorage.setItem(themeKey.name, stored("light"));
      window.dispatchEvent(new StorageEvent("storage", { key: themeKey.name, newValue: stored("light") }));
    });
    expect(result.current).toBe("light");
    expect(html.dataset["theme"]).toBe("light");
  });
});
