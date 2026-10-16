// SPDX-License-Identifier: AGPL-3.0-only
// Regenerates pseudo-locales, typed accessors (Go and TS), and the parity golden file
// from the source catalogs. Run after editing locales/: `make i18n-gen`.
// Generated files are committed, so building the Go server never needs Bun.
import { mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { loadCatalog, localesDir, PSEUDO_LOCALES, rel } from "./catalog";
import { generate } from "./generate";

const files = await generate(loadCatalog());
// Pseudo-locale directories are fully generated; drop namespaces that no longer exist.
for (const locale of PSEUDO_LOCALES) {
  const dir = join(localesDir, locale);
  mkdirSync(dir, { recursive: true });
  for (const f of readdirSync(dir)) if (!files.has(join(dir, f))) rmSync(join(dir, f));
}
for (const [path, content] of files) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, content);
  console.log(`wrote ${rel(path)}`);
}
