// SPDX-License-Identifier: AGPL-3.0-only
// Reads app/.oxlintrc.json for the tests of this directory, so that they check the restrictions the
// workspace is linted with and not a copy of them.
import { readFileSync } from "node:fs";
import path from "node:path";

/** app/, where .oxlintrc.json is: its globs and relative paths are rooted there. */
export const appDir = path.dirname(import.meta.dirname);

/**
 * Removes the comments of a JSONC text, leaving strings as they are.
 * @param {string} text
 */
function stripComments(text) {
  let out = "";
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch === '"') {
      const start = i;
      for (i++; i < text.length && text[i] !== '"'; i++) if (text[i] === "\\") i++;
      out += text.slice(start, i + 1);
    } else if (ch === "/" && text[i + 1] === "/") {
      while (i < text.length && text[i] !== "\n") i++;
      out += "\n";
    } else if (ch === "/" && text[i + 1] === "*") {
      i = text.indexOf("*/", i + 2) + 1;
    } else {
      out += ch;
    }
  }
  return out;
}

/**
 * @typedef {{ files: string[], excludeFiles?: string[], rules?: Record<string, unknown> }} Override
 * @typedef {{ rules: Record<string, unknown>, overrides: Override[] }} Config
 */

/** @type {Config} */
const config = JSON.parse(stripComments(readFileSync(path.join(appDir, ".oxlintrc.json"), "utf8")));

/** @param {string} file @param {string[]} globs */
const matches = (file, globs) => globs.some((glob) => path.matchesGlob(file, glob));

/**
 * The configuration of `rule` for `file` (relative to app/): the root's, replaced by each override that
 * matches the file, in order, as Oxlint applies them.
 * @param {string} rule
 * @param {string} file
 * @returns {unknown}
 */
export function ruleValue(rule, file) {
  let value = config.rules[rule];
  for (const override of config.overrides) {
    if (!matches(file, override.files) || matches(file, override.excludeFiles ?? [])) continue;
    if (override.rules && rule in override.rules) value = override.rules[rule];
  }
  return value;
}

/**
 * The options of `rule` for `file`, which must enable it at error.
 * @param {string} rule
 * @param {string} file
 * @returns {unknown[]}
 */
export function ruleOptions(rule, file) {
  const value = ruleValue(rule, file);
  if (!Array.isArray(value) || value[0] !== "error") throw new Error(`${rule} is not an error with options in ${file}`);
  return value.slice(1);
}
