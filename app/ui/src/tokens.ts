// SPDX-License-Identifier: AGPL-3.0-only
// The documented foreground / background pairs of the semantic color tokens in theme.css.
// scripts/check-tokens.ts verifies each pair against WCAG 2.2 AA in the light and the dark theme,
// and fails when a semantic color is neither in a pair nor listed as decorative. The tokens
// story renders the same list.

/** What a pair is used for, which sets its minimum contrast ratio. */
export type ContrastUse =
  /** Text and text-like icons: 4.5:1 (WCAG 1.4.3). */
  | "text"
  /** Control boundaries, focus indicators and meaningful graphics: 3:1 (WCAG 1.4.11). */
  | "ui";

/** The minimum contrast ratio for each use (AA). */
export const minimumContrast: Record<ContrastUse, number> = { text: 4.5, ui: 3 };

/** A semantic color drawn on a semantic background, both named without the `--color-` prefix. */
export type ColorPair = { fg: string; bg: string; use: ContrastUse };

/** The backgrounds that text and controls sit on. */
export const surfaces = ["surface", "surface-raised", "surface-sunken"] as const;

const onEverySurface = (fg: string, use: ContrastUse): ColorPair[] => surfaces.map((bg) => ({ fg, bg, use }));

export const colorPairs: readonly ColorPair[] = [
  ...onEverySurface("fg", "text"),
  ...onEverySurface("fg-muted", "text"),
  ...onEverySurface("accent", "text"),
  ...onEverySurface("accent-hover", "text"),
  ...onEverySurface("danger", "text"),
  ...onEverySurface("success", "text"),
  ...onEverySurface("warning", "text"),
  ...onEverySurface("border-strong", "ui"),
  ...onEverySurface("focus", "ui"),
  { fg: "accent-fg", bg: "accent", use: "text" },
  { fg: "accent-fg", bg: "accent-hover", use: "text" },
  { fg: "danger-fg", bg: "danger", use: "text" },
  { fg: "success-fg", bg: "success", use: "text" },
  { fg: "warning-fg", bg: "warning", use: "text" },
];

/**
 * Semantic colors that carry no information on their own, so no contrast minimum applies:
 * border separates regions that are already distinguishable, and overlay is the modal scrim.
 */
export const decorativeColors: readonly string[] = ["border", "overlay"];
