// SPDX-License-Identifier: AGPL-3.0-only
import { act, renderHook } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { applyStoredTheme, documentTheme, setTheme, themeKey, useTheme } from "./theme";

const html = document.documentElement;

describe("theme", () => {
  test("a choice sets <html data-theme> and the storage; the OS setting removes both", () => {
    setTheme("dark");
    expect(html.dataset["theme"]).toBe("dark");
    expect(localStorage.getItem(themeKey)).toBe("dark");
    expect(documentTheme(document)).toBe("dark");

    setTheme("system");
    expect(html).not.toHaveAttribute("data-theme");
    expect(localStorage.getItem(themeKey)).toBeNull();
    expect(documentTheme(document)).toBe("system");
  });

  test("the stored theme applies at start; an unknown value is the OS setting", () => {
    localStorage.setItem(themeKey, "light");
    applyStoredTheme(document);
    expect(html.dataset["theme"]).toBe("light");

    localStorage.setItem(themeKey, "sepia");
    applyStoredTheme(document);
    expect(html).not.toHaveAttribute("data-theme");
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
      localStorage.setItem(themeKey, "light");
      window.dispatchEvent(new StorageEvent("storage", { key: themeKey, newValue: "light" }));
    });
    expect(result.current).toBe("light");
    expect(html.dataset["theme"]).toBe("light");
  });
});
