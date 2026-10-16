// SPDX-License-Identifier: AGPL-3.0-only
// Catalog loading and the repository paths shared by the i18n scripts.
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";

export const repoRoot = join(import.meta.dir, "..", "..", "..");
export const localesDir = join(repoRoot, "locales");
/** Where the Go package and its parity fixtures live. */
export const goPackageDir = join(repoRoot, "internal", "i18n");
export const goTestdataDir = join(goPackageDir, "testdata");

export const rel = (path: string): string => relative(repoRoot, path);

/** The locale every catalog is written in, and the last fallback. */
export const SOURCE = "en";
/** Pseudo-locales generated from SOURCE (pseudo.ts); never edited or translated by hand. */
export const PSEUDO_LOCALES = ["en-XA", "ar-XB"] as const;
export const isPseudo = (locale: string): boolean => (PSEUDO_LOCALES as readonly string[]).includes(locale);

/** One catalog entry, in the FormatJS JSON shape that Weblate reads natively. */
export type Entry = { defaultMessage: string; description?: string };
export type Namespace = Record<string, Entry>;
/** locale -> namespace -> key -> entry */
export type Catalog = Record<string, Record<string, Namespace>>;

/** Reads every locales/<locale>/<namespace>.json file. */
export function loadCatalog(): Catalog {
  const catalog: Catalog = {};
  for (const locale of readdirSync(localesDir, { withFileTypes: true })) {
    if (!locale.isDirectory()) continue;
    catalog[locale.name] = {};
    for (const file of readdirSync(join(localesDir, locale.name)).sort()) {
      if (!file.endsWith(".json")) continue;
      catalog[locale.name][file.slice(0, -".json".length)] = JSON.parse(
        readFileSync(join(localesDir, locale.name, file), "utf8"),
      );
    }
  }
  return catalog;
}

/**
 * Namespaces whose name starts with "client" are used only by the TypeScript clients and
 * may use FormatJS-only syntax. Every other namespace is formatted by Go as well and must
 * stay inside the shared subset (icu.ts outsideSharedSubset).
 */
export const isClientNamespace = (ns: string): boolean => ns.startsWith("client");

/**
 * Pseudo-locales keep the source locale's plural categories, so both runtimes format them
 * with its CLDR rules (Go: formattingLocale in internal/i18n).
 */
export const formattingLocale = (locale: string): string => (isPseudo(locale) ? SOURCE : locale);

/** Serializes JSON the way every generated and catalog file is written. */
export const toJSON = (value: unknown): string => `${JSON.stringify(value, null, 2)}\n`;
