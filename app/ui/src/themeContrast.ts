// SPDX-License-Identifier: AGPL-3.0-only
// The contrast of the color tokens under a server theme's overrides (ADR-0059): the theme editor shows it while
// the administrator types, with the same pairs and formulas as the server, which decides whether a theme saves.
import { contrast, parseColor } from "./color";
import { themeTokens } from "./themeTokens.gen";
import { colorPairs, type ContrastUse, minimumContrast } from "./tokens";

/** A color token's value in each theme, as `#rrggbb[aa]`. */
export type ThemeColorValue = { light: string; dark: string };

/** The contrast of one pair in one theme; tokens are named as a theme names them (`color-fg`). */
export type ThemeContrast = {
  fg: string;
  bg: string;
  use: ContrastUse;
  scheme: "light" | "dark";
  /** The ratio, rounded down to two decimals as the server reports it; null when a color is malformed. */
  ratio: number | null;
  minimum: number;
};

const hexColor = /^#[0-9a-f]{6}(?:[0-9a-f]{2})?$/i;

/** The ratio of fg on bg, or null when either is not `#rrggbb[aa]`. */
function ratioOf(fg: string, bg: string): number | null {
  if (!hexColor.test(fg) || !hexColor.test(bg)) return null;
  return Math.floor(contrast(parseColor(fg), parseColor(bg)) * 100) / 100;
}

/**
 * The contrast of every pair in both themes, with `overrides` (by token name) in place of the defaults.
 * A pair passes when its ratio is at least its minimum.
 */
export function themeContrast(overrides: ReadonlyMap<string, ThemeColorValue>): ThemeContrast[] {
  const color = (name: string, scheme: "light" | "dark") => {
    const value = overrides.get(name) ?? themeTokens.find((t) => t.name === name);
    return value?.[scheme] ?? "";
  };
  return (["light", "dark"] as const).flatMap((scheme) =>
    colorPairs.map(({ fg, bg, use }) => ({
      fg: `color-${fg}`,
      bg: `color-${bg}`,
      use,
      scheme,
      ratio: ratioOf(color(`color-${fg}`, scheme), color(`color-${bg}`, scheme)),
      minimum: minimumContrast[use],
    })),
  );
}
