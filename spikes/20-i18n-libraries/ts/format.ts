// SPDX-License-Identifier: AGPL-3.0-only
// Formats every case in testdata/cases.json for every golden locale with
// FormatJS intl-messageformat (the TS-side reference implementation).
//
//   bun run ts/format.ts          # verify against testdata/golden.json
//   bun run ts/format.ts --write  # regenerate testdata/golden.json
import { IntlMessageFormat } from "intl-messageformat";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { formattingLocale, goldenLocales, loadCases, loadCatalog, root } from "./catalog";

type Golden = Record<string, string[]>; // locale -> outputs in case order

const catalog = loadCatalog();
const cases = loadCases();
const actual: Golden = {};
for (const locale of goldenLocales) {
  actual[locale] = cases.map(({ key, args }) => {
    const entry = catalog[locale]?.demo?.[key];
    if (!entry) throw new Error(`${locale}: missing key ${key}`);
    return String(new IntlMessageFormat(entry.defaultMessage, formattingLocale(locale)).format(args));
  });
}

const goldenPath = join(root, "testdata", "golden.json");
if (process.argv.includes("--write")) {
  writeFileSync(goldenPath, `${JSON.stringify(actual, null, 2)}\n`);
  console.log("wrote testdata/golden.json");
} else {
  const golden: Golden = JSON.parse(readFileSync(goldenPath, "utf8"));
  let failures = 0;
  for (const locale of goldenLocales) {
    actual[locale].forEach((got, i) => {
      const want = golden[locale][i];
      if (got !== want) {
        failures++;
        console.error(`${locale} ${cases[i].key} ${JSON.stringify(cases[i].args)}\n  got  ${JSON.stringify(got)}\n  want ${JSON.stringify(want)}`);
      }
    });
  }
  if (failures > 0) process.exit(1);
  console.log(`ok: ${goldenLocales.length * cases.length} outputs match testdata/golden.json`);
}
