// SPDX-License-Identifier: AGPL-3.0-only
// Generates pseudo-locales from the source locale (en):
//   en-XA: accented + ~40% longer + [brackets]   (formatjs "en-XA")
//   ar-XB: RTL pseudo (bidi override around text) (formatjs "en-XB", written as ar-XB)
// Placeholders, plural/select keywords and `#` are preserved because the
// transformation runs on the parsed ICU AST, not on the raw string.
import { compile } from "@formatjs/cli-lib";
import type { MessageFormatElement } from "@formatjs/icu-messageformat-parser";
import { printAST } from "@formatjs/icu-messageformat-parser/printer.js";
import { mkdirSync, readdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { localesDir, type Namespace } from "./catalog";

const targets = [
  { locale: "en-XA", pseudo: "en-XA" },
  { locale: "ar-XB", pseudo: "en-XB" },
] as const;

const sourceDir = join(localesDir, "en");
for (const file of readdirSync(sourceDir).filter((f) => f.endsWith(".json"))) {
  const source: Namespace = await Bun.file(join(sourceDir, file)).json();
  for (const { locale, pseudo } of targets) {
    // formatjs only pseudo-localizes ASTs; print them back to ICU source.
    const compiled: Record<string, MessageFormatElement[]> = JSON.parse(
      await compile([join(sourceDir, file)], { pseudoLocale: pseudo, ast: true }),
    );
    const out: Namespace = {};
    for (const key of Object.keys(source)) {
      out[key] = { defaultMessage: printAST(compiled[key]), description: "Generated pseudo-locale; do not translate." };
    }
    mkdirSync(join(localesDir, locale), { recursive: true });
    writeFileSync(join(localesDir, locale, file), `${JSON.stringify(out, null, 2)}\n`);
    console.log(`wrote locales/${locale}/${file}`);
  }
}
