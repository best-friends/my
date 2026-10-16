// SPDX-License-Identifier: AGPL-3.0-only
// Builds testdata/cldr.json: for several locales with non-trivial plural rules,
// an ICU message listing exactly the locale's CLDR categories, and how
// FormatJS (i.e. the webview's Intl) formats it. The Go test replays the same
// messages to detect CLDR / number-formatting drift between the runtimes.
//
//   bun run ts/cldr.ts
import { IntlMessageFormat } from "intl-messageformat";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { root } from "./catalog";

const locales = ["en", "ja", "fr", "pt-PT", "ru", "pl", "cy", "ar", "he", "zh-Hant", "de", "hi"];
const numbers = [0, 1, 2, 3, 5, 11, 21, 22, 101, 1000000, 1.5];

function categoriesMessage(arg: string, type: "plural" | "selectordinal", cats: string[]): string {
  return `{${arg}, ${type}, ${cats.map((c) => `${c} {${c}:#}`).join(" ")}}`;
}

const out = locales.map((locale) => {
  const cardinal = new Intl.PluralRules(locale).resolvedOptions().pluralCategories;
  const ordinal = new Intl.PluralRules(locale, { type: "ordinal" }).resolvedOptions().pluralCategories;
  const message = `${categoriesMessage("n", "plural", cardinal)} | ${categoriesMessage("n", "selectordinal", ordinal)} | {n, number}`;
  const mf = new IntlMessageFormat(message, locale);
  return {
    locale,
    message,
    outputs: numbers.map((n) => ({ n, want: String(mf.format({ n })) })),
  };
});

writeFileSync(join(root, "testdata", "cldr.json"), `${JSON.stringify(out, null, 2)}\n`);
console.log("wrote testdata/cldr.json");
