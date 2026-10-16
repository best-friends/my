// SPDX-License-Identifier: AGPL-3.0-only
// Shared helpers for the spike's TypeScript scripts.
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

export const root = join(import.meta.dir, "..");
export const localesDir = join(root, "locales");

/** One catalog entry, in the Format.JS JSON shape that Weblate understands. */
export type Entry = { defaultMessage: string; description?: string };
export type Namespace = Record<string, Entry>;
/** locale -> namespace -> key -> entry */
export type Catalog = Record<string, Record<string, Namespace>>;

export function loadCatalog(): Catalog {
  const catalog: Catalog = {};
  for (const locale of readdirSync(localesDir).sort()) {
    catalog[locale] = {};
    for (const file of readdirSync(join(localesDir, locale)).sort()) {
      if (!file.endsWith(".json")) continue;
      const ns = file.slice(0, -".json".length);
      catalog[locale][ns] = JSON.parse(readFileSync(join(localesDir, locale, file), "utf8"));
    }
  }
  return catalog;
}

export type Case = { key: string; args: Record<string, string | number> };

export function loadCases(): Case[] {
  return JSON.parse(readFileSync(join(root, "testdata", "cases.json"), "utf8"));
}

/** Locales that the golden file covers (source, translation, pseudo). */
export const goldenLocales = ["en", "ja", "en-XA", "ar-XB"];

/**
 * Pseudo-locales are generated from `en`, so they keep en plural categories and
 * number formatting; only the strings (and `dir`) change. Both runtimes must
 * format them with the source locale's CLDR rules.
 */
export const formattingLocale = (locale: string): string =>
  ({ "en-XA": "en", "ar-XB": "en" })[locale] ?? locale;
