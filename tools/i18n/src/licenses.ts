// SPDX-License-Identifier: AGPL-3.0-only
// Fails when an installed npm package has a license incompatible with AGPL-3.0-only (ADR-0011).
//
//   bun run src/licenses.ts MIT,Apache-2.0,...   # allowed SPDX identifiers (the Makefile passes its list)
//
// SPDX expressions are evaluated: "A OR B" needs one allowed side, "A AND B" needs both.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const allowed = new Set((process.argv[2] ?? "").split(",").filter(Boolean));
if (allowed.size === 0) {
  console.error("usage: bun run src/licenses.ts <comma-separated SPDX identifiers>");
  process.exit(2);
}

/**
 * Reviewed packages whose license field is not an SPDX identifier, pinned by version so an
 * upgrade is reviewed again.
 */
const reviewed: Record<string, string> = {
  // Douglas Crockford's JSON parser, dedicated to the public domain; pulled in by
  // @formatjs/cli-lib through json-stable-stringify.
  "jsonify@0.0.1": "Public Domain",
};

type Manifest ={ name?: string; version?: string; license?: unknown; licenses?: unknown };

function licenseOf(m: Manifest): string | undefined {
  const one = (l: unknown) => (typeof l === "string" ? l : (l as { type?: string } | null)?.type);
  if (m.license !== undefined) return one(m.license);
  if (Array.isArray(m.licenses)) return m.licenses.map(one).join(" OR ");
}

/** Evaluates an SPDX license expression (without WITH exceptions) against the allowlist. */
function permitted(expr: string): boolean {
  const e = expr.trim().replace(/^\((.*)\)$/s, "$1");
  if (/\sOR\s/.test(e)) return e.split(/\s+OR\s+/).some(permitted);
  if (/\sAND\s/.test(e)) return e.split(/\s+AND\s+/).every(permitted);
  return allowed.has(e);
}

const failures: string[] = [];
let count = 0;
function walk(dir: string) {
  if (!existsSync(dir)) return;
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (!entry.isDirectory() || entry.name.startsWith(".")) continue;
    const pkg = join(dir, entry.name);
    if (entry.name.startsWith("@")) {
      walk(pkg);
      continue;
    }
    const manifest = join(pkg, "package.json");
    if (existsSync(manifest)) {
      const m: Manifest = JSON.parse(readFileSync(manifest, "utf8"));
      const license = licenseOf(m);
      count++;
      const ok = license !== undefined && (permitted(license) || reviewed[`${m.name}@${m.version}`] === license);
      if (!ok) failures.push(`${m.name}@${m.version}: ${license ?? "no license"}`);
    }
    walk(join(pkg, "node_modules"));
  }
}

walk(join(import.meta.dir, "..", "node_modules"));
for (const f of failures) console.error(`error: incompatible npm license: ${f}`);
console.log(`licenses: ${count} npm package(s) checked, ${failures.length} incompatible`);
if (failures.length > 0) process.exit(1);
