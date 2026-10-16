// SPDX-License-Identifier: AGPL-3.0-only
// Catalog lint, the CI checks proposed for ADR-0009:
//   error  (a) ICU syntax error (incl. missing `other`)
//   error  (b) placeholder mismatch vs. source: extra argument or different type
//   error  (c) plural / selectordinal category that the locale's CLDR rules do not have
//   error  (d) key present in a translation but not in the source (obsolete)
//   warn   (e) key missing in a translation (untranslated; falls back to en)
//   warn   (f) translation drops a source argument (sometimes legitimate, e.g. ja drops gender)
//   warn   (g) source key not referenced from code (unused) — needs a code search, see report
//   error  (h) generated accessors are stale (run ts/gen.ts)
//   error  (i) construct outside the Go/TS shared subset in a non-client namespace
//
//   bun run ts/check.ts
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { formattingLocale, loadCatalog, root } from "./catalog";
import { type Args, extractArgs, outsideSharedSubset, parseMessage, pluralCategories } from "./icu";

const SOURCE = "en";
const PSEUDO = new Set(["en-XA", "ar-XB"]);
const catalog = loadCatalog();
let errors = 0;
let warnings = 0;
const error = (m: string) => (errors++, console.error(`error: ${m}`));
const warn = (m: string) => (warnings++, console.warn(`warn:  ${m}`));

function analyze(locale: string, ns: string, key: string, source: string): Args | undefined {
  try {
    const ast = parseMessage(source);
    const cardinal = new Set(new Intl.PluralRules(formattingLocale(locale)).resolvedOptions().pluralCategories);
    const ordinal = new Set(
      new Intl.PluralRules(formattingLocale(locale), { type: "ordinal" }).resolvedOptions().pluralCategories,
    );
    if (!ns.startsWith("client")) {
      for (const f of outsideSharedSubset(ast)) {
        error(`${locale}/${ns}.json ${key}: ${f} is not in the Go/TS shared subset (use a client* namespace)`);
      }
    }
    for (const c of pluralCategories(ast)) {
      if (!(c.type === "cardinal" ? cardinal : ordinal).has(c.key as Intl.LDMLPluralRule)) {
        error(`${locale}/${ns}.json ${key}: ${c.type} category "${c.key}" does not exist in ${locale}`);
      }
    }
    return extractArgs(ast);
  } catch (e) {
    error(`${locale}/${ns}.json ${key}: ${(e as Error).message}`);
  }
}

const sourceArgs = new Map<string, Args | undefined>();
for (const [ns, entries] of Object.entries(catalog[SOURCE])) {
  for (const [key, e] of Object.entries(entries)) sourceArgs.set(`${ns}:${key}`, analyze(SOURCE, ns, key, e.defaultMessage));
}

for (const [locale, namespaces] of Object.entries(catalog)) {
  if (locale === SOURCE) continue;
  for (const id of sourceArgs.keys()) {
    const [ns, key] = id.split(/:(.*)/s);
    if (!namespaces[ns]?.[key]) warn(`${locale}: untranslated ${id}`);
  }
  for (const [ns, entries] of Object.entries(namespaces)) {
    for (const [key, e] of Object.entries(entries)) {
      const id = `${ns}:${key}`;
      if (!sourceArgs.has(id)) {
        error(`${locale}/${ns}.json ${key}: not in source locale (obsolete)`);
        continue;
      }
      const want = sourceArgs.get(id);
      const got = analyze(locale, ns, key, e.defaultMessage);
      if (!want || !got) continue;
      for (const [name, t] of got) {
        const w = want.get(name);
        if (!w) error(`${locale}/${ns}.json ${key}: unknown argument {${name}}`);
        else if (w.kind !== t.kind) error(`${locale}/${ns}.json ${key}: {${name}} is ${t.kind}, source has ${w.kind}`);
      }
      for (const name of want.keys()) {
        if (!got.has(name) && !PSEUDO.has(locale)) warn(`${locale}/${ns}.json ${key}: drops argument {${name}}`);
      }
    }
  }
}

// (h) generated accessors must be up to date.
const before = ["ts/messages.gen.ts", "messages_gen.go"].map((f) => readFileSync(join(root, f), "utf8"));
await import("./gen");
const after = ["ts/messages.gen.ts", "messages_gen.go"].map((f) => readFileSync(join(root, f), "utf8"));
if (before.some((b, i) => b !== after[i])) error("generated accessors were stale; regenerated — commit the result");

console.log(`${errors} error(s), ${warnings} warning(s)`);
if (errors > 0) process.exit(1);
