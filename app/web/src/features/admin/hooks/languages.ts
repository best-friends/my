// SPDX-License-Identifier: AGPL-3.0-only
// The languages that the server's texts are edited in (its name, its descriptions, its rules): those of the app's
// catalogs, and any other language a text was saved in (from the CLI), each named in the app's language.
import { supportedLocales } from "../../../lib/i18n";

/** Pseudo-locales are for testing; nobody reads the server's texts in them. */
const pseudoLocales: ReadonlySet<string> = new Set(["en-XA", "ar-XB"]);

/** The languages to edit a text in: the app's, and those of `saved`, sorted by tag. */
export function editedLocales(saved: Iterable<string>): string[] {
  return [...new Set([...supportedLocales.filter((locale) => !pseudoLocales.has(locale)), ...saved])].toSorted();
}

/** The app's languages that people may pick as the server's default language. */
export function selectableLocales(): string[] {
  return supportedLocales.filter((locale) => !pseudoLocales.has(locale));
}

/** `tag`'s name in `inLocale`, or the tag itself when the browser does not know it. */
export function languageName(tag: string, inLocale: string): string {
  try {
    return new Intl.DisplayNames([inLocale], { type: "language", fallback: "code" }).of(tag) ?? tag;
  } catch {
    // Not a valid tag for Intl; the tag is its own name.
    return tag;
  }
}

/** The number of characters of `text` as the server counts them: Unicode code points. */
export function characterCount(text: string): number {
  return Array.from(text).length;
}
