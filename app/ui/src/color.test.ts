// SPDX-License-Identifier: AGPL-3.0-only
// The color math shared by the token check and the theme editor. The vectors are the same as the Go domain's
// (internal/domain/instance/theme_test.go), so the client and the server agree on whether a theme passes.
import { describe, expect, it } from "vitest";
import { contrast, parseColor, toHex } from "./color";
import { themeContrast } from "./themeContrast";
import { themeTokens } from "./themeTokens.gen";

describe("contrast", () => {
  it.each([
    ["#000000", "#ffffff", 21],
    ["#ffffff", "#ffffff", 1],
    ["#777777", "#ffffff", 4.48],
    ["#00000080", "#ffffff", 4.0],
  ])("of %s on %s is %f", (fg, bg, want) => {
    expect(contrast(parseColor(fg), parseColor(bg))).toBeCloseTo(want, 2);
  });
});

describe("toHex", () => {
  it("writes opaque and translucent colors", () => {
    expect(toHex(parseColor("oklch(100% 0 0)"))).toBe("#ffffff");
    expect(toHex(parseColor("oklch(0% 0 0 / 0.4)"))).toBe("#00000066");
  });
});

describe("themeContrast", () => {
  it("passes every pair with the defaults", () => {
    const failing = themeContrast(new Map()).filter((c) => c.ratio === null || c.ratio < c.minimum);
    expect(failing).toEqual([]);
  });

  it("checks an override against the defaults of the other tokens in its theme", () => {
    const accent = themeTokens.find((t) => t.name === "color-accent");
    const results = themeContrast(new Map([["color-accent", { light: "#a0c4ff", dark: accent?.dark ?? "" }]]));
    const failing = results.filter((c) => c.ratio !== null && c.ratio < c.minimum);
    expect(failing.map((c) => `${c.fg}/${c.bg}/${c.scheme}`)).toContain("color-accent/color-surface/light");
    expect(failing.every((c) => c.scheme === "light")).toBe(true);
  });

  it("reports a malformed color as no ratio", () => {
    const results = themeContrast(new Map([["color-fg", { light: "red", dark: "#ffffff" }]]));
    expect(results.find((c) => c.fg === "color-fg" && c.scheme === "light")?.ratio).toBeNull();
  });
});
