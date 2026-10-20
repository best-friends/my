// SPDX-License-Identifier: AGPL-3.0-only
// Reads the design tokens of src/theme.css for the token check (check-tokens.ts) and the generator of the
// overridable token list (theme-tokens.ts).
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { type LinearRgba, parseColor } from "../src/color";

export const themePath = resolve(import.meta.dir, "../src/theme.css");

export type Scheme = "light" | "dark";
export const schemes: readonly Scheme[] = ["light", "dark"];

/** A custom property declared in an `@theme` block, named without `--`. */
export type ThemeDeclaration = { name: string; value: string };

function withoutComments(css: string): string {
  return css.replaceAll(/\/\*[\s\S]*?\*\//g, "");
}

/** Splits a function's arguments at top-level commas. */
export function splitArguments(body: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < body.length; i++) {
    const ch = body[i];
    if (ch === "(") depth++;
    else if (ch === ")") depth--;
    else if (ch === "," && depth === 0) {
      parts.push(body.slice(start, i).trim());
      start = i + 1;
    }
  }
  parts.push(body.slice(start).trim());
  return parts;
}

/** The declarations of `--<prefix>-*` anywhere in css, by the name after the prefix. */
export function readDeclarations(css: string, prefix: string): Map<string, string> {
  const declarations = new Map<string, string>();
  for (const match of withoutComments(css).matchAll(new RegExp(`--${prefix}-([a-z0-9-]+)\\s*:\\s*([^;]+);`, "g"))) {
    const [, name, value] = match;
    if (name !== undefined && value !== undefined) declarations.set(name, value.trim());
  }
  return declarations;
}

/**
 * The declarations of every `@theme` and `@theme static` block in order, without the resets (`initial`).
 * Throws on `@theme inline`, whose values Tailwind copies into utilities, so a server theme could not
 * override them (ADR-0059).
 */
export function readThemeDeclarations(css: string): ThemeDeclaration[] {
  const source = withoutComments(css);
  if (/@theme\b[^{]*\binline\b/.test(source)) throw new Error("theme.css: do not use @theme inline (ADR-0059)");
  const out: ThemeDeclaration[] = [];
  for (const block of source.matchAll(/@theme\b[^{]*\{([^}]*)\}/g)) {
    for (const match of (block[1] ?? "").matchAll(/--([a-z0-9-]+)\s*:\s*([^;]+);/g)) {
      const [, name = "", raw = ""] = match;
      const value = raw.replaceAll(/\s+/g, " ").trim();
      if (value !== "initial") out.push({ name, value });
    }
  }
  return out;
}

/** The source of theme.css. */
export function readTheme(): string {
  return readFileSync(themePath, "utf8");
}

/** Resolves a semantic color of theme.css for a theme, following light-dark() and var() references. */
export function colorResolver(css: string): (value: string, scheme: Scheme) => LinearRgba {
  const palette = readDeclarations(css, "palette");
  const semantic = readDeclarations(css, "color");
  const resolveColor = (value: string, scheme: Scheme, seen: string[] = []): LinearRgba => {
    const lightDark = /^light-dark\(([\s\S]*)\)$/.exec(value);
    if (lightDark?.[1] !== undefined) {
      const args = splitArguments(lightDark[1]);
      if (args.length !== 2) throw new Error(`light-dark() needs two colors: ${value}`);
      return resolveColor(args[scheme === "light" ? 0 : 1] ?? "", scheme, seen);
    }
    const reference = /^var\(--(palette|color)-([a-z0-9-]+)\)$/.exec(value);
    if (reference !== null) {
      const [, layer = "", name = ""] = reference;
      const key = `${layer}-${name}`;
      if (seen.includes(key)) throw new Error(`circular reference: ${[...seen, key].join(" -> ")}`);
      const target = (layer === "palette" ? palette : semantic).get(name);
      if (target === undefined) throw new Error(`undefined --${key}`);
      return resolveColor(target, scheme, [...seen, key]);
    }
    return parseColor(value);
  };
  return (value, scheme) => resolveColor(value, scheme);
}
