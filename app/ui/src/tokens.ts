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

/**
 * The kind of value a token of theme.css holds, which decides how a server theme's override of it is
 * checked (ADR-0059): `color` is `#rrggbb` per theme (`#rrggbbaa` where the default is translucent),
 * `length` is `0` or a `rem` / `px` value, `number` is unitless, `duration` is `ms` / `s`, `easing` is a
 * keyword or `cubic-bezier()`, `font-family` is a list of family names, and `shadow` is `none` or up to
 * four `[inset] <x> <y> [<blur> [<spread>]] <color>` layers.
 */
export type TokenKind = "color" | "length" | "number" | "font-family" | "shadow" | "duration" | "easing";

/** How the tokens whose name (without `--`) matches `pattern` are overridden. */
export type TokenRule = {
  pattern: RegExp;
  kind: TokenKind;
  /** The bounds of a `length` (in rem), `number`, or `duration` (in ms), inclusive. */
  min?: number;
  max?: number;
};

/**
 * The overridable tokens of theme.css by name, first match wins. The bounds keep a theme usable: text
 * stays readable, a spacing step stays between half and twice the default, and motion stays short.
 * scripts/theme-tokens.ts fails on a token that matches no rule and is not in `fixedTokens`.
 */
export const tokenRules: readonly TokenRule[] = [
  { pattern: /^color-/, kind: "color" },
  { pattern: /^font-(?:sans|mono)$/, kind: "font-family" },
  { pattern: /^text-[a-z0-9]+--line-height$/, kind: "number", min: 1, max: 2.5 },
  { pattern: /^text-[a-z0-9]+$/, kind: "length", min: 0.625, max: 3 },
  { pattern: /^spacing$/, kind: "length", min: 0.125, max: 0.5 },
  { pattern: /^radius-/, kind: "length", min: 0, max: 2 },
  { pattern: /^elevation-/, kind: "shadow" },
  { pattern: /^(?:transition-duration-[a-z]+|default-transition-duration)$/, kind: "duration", min: 0, max: 1000 },
  { pattern: /^(?:ease-[a-z]+|default-transition-timing-function)$/, kind: "easing" },
];

/**
 * Tokens of theme.css that a server theme cannot override: the primitive palette, an implementation
 * detail of the semantic colors (override those instead), and `--shadow-*`, whose values Tailwind copies
 * into its utilities (override `--elevation-*`, which they refer to).
 */
export const fixedTokens: readonly RegExp[] = [/^palette-/, /^shadow-/];
