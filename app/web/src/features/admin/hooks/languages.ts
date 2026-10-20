// SPDX-License-Identifier: AGPL-3.0-only
// The languages that the server settings offer as the server's default language, each named in the app's language,
// and how the server counts the characters of the texts that administrators write.
import { supportedLocales } from "../../../lib/i18n";

/** Pseudo-locales are for testing; nobody uses the app in them. */
const pseudoLocales: ReadonlySet<string> = new Set(["en-XA", "ar-XB"]);

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
