// SPDX-License-Identifier: AGPL-3.0-only
// Catalog checks run by `make i18n-check` (ADR-0009 §1).
//   error: ICU syntax error, including a plural / select without `other`
//   error: translation argument unknown to the source, or of a different type
//   error: plural / selectordinal category the locale's CLDR rules do not have
//   error: translation key absent from the source (obsolete)
//   error: rich-text tag, number skeleton / style, or date / time argument in a namespace
//          shared with Go (anything but client*)
//   error: malformed locale directory, namespace, or key; two keys mapping to one accessor
//   error: generated files (pseudo-locales, accessors, golden) differ from `make i18n-gen`
//   warn:  untranslated key (falls back to en at runtime)
//   warn:  translation drops a source argument (sometimes legitimate)
//   warn:  key not referenced from code
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import {
  type Catalog,
  formattingLocale,
  isClientNamespace,
  isPseudo,
  loadCatalog,
  localesDir,
  PSEUDO_LOCALES,
  rel,
  repoRoot,
  SOURCE,
} from "./catalog";
import { generate, goName } from "./generate";
import { type Args, extractArgs, outsideSharedSubset, parseMessage, pluralCategories } from "./icu";

let errors = 0;
let warnings = 0;
const error = (m: string) => (errors++, console.error(`error: ${m}`));
const warn = (m: string) => (warnings++, console.warn(`warn:  ${m}`));

const NAMESPACE = /^[a-z][a-z0-9]*$/;
const KEY = /^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$/;

function analyze(locale: string, ns: string, key: string, message: unknown): Args | undefined {
  const where = `locales/${locale}/${ns}.json ${key}`;
  if (typeof message !== "string") return void error(`${where}: defaultMessage must be a string`);
  try {
    const ast = parseMessage(message);
    if (!isClientNamespace(ns)) {
      for (const f of outsideSharedSubset(ast)) {
        error(`${where}: ${f} is outside the Go/TS shared subset (allowed only in client* namespaces)`);
      }
    }
    const rules = {
      cardinal: new Set<string>(new Intl.PluralRules(formattingLocale(locale)).resolvedOptions().pluralCategories),
      ordinal: new Set<string>(
        new Intl.PluralRules(formattingLocale(locale), { type: "ordinal" }).resolvedOptions().pluralCategories,
      ),
    };
    for (const c of pluralCategories(ast)) {
      if (!rules[c.type].has(c.key)) error(`${where}: ${c.type} category "${c.key}" does not exist in ${locale}`);
    }
    return extractArgs(ast);
  } catch (e) {
    error(`${where}: ${(e as Error).message}`);
  }
}

function checkCatalog(catalog: Catalog) {
  if (!catalog[SOURCE]) return void error(`locales/${SOURCE}: the source locale is missing`);
  for (const locale of Object.keys(catalog)) {
    let canonical: string | undefined;
    try {
      canonical = Intl.getCanonicalLocales(locale)[0];
    } catch {}
    if (canonical !== locale) error(`locales/${locale}: not a canonical BCP 47 tag${canonical ? ` (use ${canonical})` : ""}`);
  }

  const sourceArgs = new Map<string, Args | undefined>();
  const accessors = new Map<string, string>();
  for (const [ns, entries] of Object.entries(catalog[SOURCE])) {
    if (!NAMESPACE.test(ns)) error(`locales/${SOURCE}/${ns}.json: namespace must match ${NAMESPACE}`);
    for (const [key, e] of Object.entries(entries)) {
      const id = `${ns}:${key}`;
      if (!KEY.test(key)) error(`locales/${SOURCE}/${ns}.json ${key}: key must match ${KEY}`);
      if (!e.description) warn(`locales/${SOURCE}/${ns}.json ${key}: no description for translators`);
      const name = goName(id);
      if (accessors.has(name)) error(`${id} and ${accessors.get(name)} both map to the accessor ${name}`);
      accessors.set(name, id);
      sourceArgs.set(id, analyze(SOURCE, ns, key, e.defaultMessage));
    }
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
          error(`locales/${locale}/${ns}.json ${key}: not in the source locale (obsolete)`);
          continue;
        }
        const want = sourceArgs.get(id);
        const got = analyze(locale, ns, key, e.defaultMessage);
        if (!want || !got) continue;
        for (const [name, t] of got) {
          const w = want.get(name);
          if (!w) error(`locales/${locale}/${ns}.json ${key}: unknown argument {${name}}`);
          else if (w.kind !== t.kind) error(`locales/${locale}/${ns}.json ${key}: {${name}} is ${t.kind}, the source has ${w.kind}`);
        }
        for (const name of want.keys()) {
          if (!got.has(name) && !isPseudo(locale)) warn(`locales/${locale}/${ns}.json ${key}: drops argument {${name}}`);
        }
      }
    }
  }
  return sourceArgs;
}

/** Warns about keys referenced neither by a Go accessor call nor by a TS key literal. */
function checkUnused(ids: Iterable<string>) {
  const read = (pattern: string, skip: (p: string) => boolean) =>
    [...new Bun.Glob(pattern).scanSync({ cwd: repoRoot })]
      .filter((p) => !p.includes("node_modules/") && !p.startsWith("internal/i18n/") && !skip(p))
      .map((p) => readFileSync(join(repoRoot, p), "utf8"))
      .join("\n");
  const go = read("{cmd,internal,pkg}/**/*.go", (p) => p.endsWith("_gen.go") || p.endsWith("_test.go"));
  const ts = existsSync(join(repoRoot, "app")) ? read("app/**/*.{ts,tsx}", (p) => p.endsWith(".gen.ts")) : "";
  for (const id of ids) {
    const used = (!isClientNamespace(id) && go.includes(`.${goName(id)}(`)) || ts.includes(JSON.stringify(id));
    if (!used) warn(`unused ${id}`);
  }
}

async function checkGenerated(catalog: Catalog) {
  const files = await generate(catalog);
  for (const [path, want] of files) {
    const got = existsSync(path) ? readFileSync(path, "utf8") : undefined;
    if (got !== want) error(`${rel(path)} is ${got === undefined ? "missing" : "stale"}; run make i18n-gen and commit the result`);
  }
  for (const locale of PSEUDO_LOCALES) {
    const dir = join(localesDir, locale);
    if (!existsSync(dir)) continue;
    for (const f of readdirSync(dir)) {
      if (!files.has(join(dir, f))) error(`locales/${locale}/${f} has no source namespace; run make i18n-gen`);
    }
  }
}

const catalog = loadCatalog();
const sourceArgs = checkCatalog(catalog);
if (errors === 0) {
  checkUnused(sourceArgs?.keys() ?? []);
  await checkGenerated(catalog);
}
console.log(`i18n: ${errors} error(s), ${warnings} warning(s)`);
if (errors > 0) process.exit(1);
