// SPDX-License-Identifier: AGPL-3.0-only
// Copies the license notices of the third-party data the client bundles but that are not npm dependencies (and so are
// outside `web:licenses`), such as the Unicode emoji data (features/emoji/unicode/LICENSE-Unicode.txt, ADR-0091 §1),
// into dist/licenses/ under their own names: the build carries each notice with the data it covers, Go serves them at
// /licenses/, and the about page links to each (features/about).
import { readFileSync } from "node:fs";
import { basename } from "node:path";
import type { Plugin } from "vite";

export function bundledLicenses(files: readonly string[]): Plugin {
  return {
    name: "my:licenses",
    apply: "build",
    generateBundle() {
      for (const file of files) {
        this.emitFile({ type: "asset", fileName: `licenses/${basename(file)}`, source: readFileSync(file) });
      }
    },
  };
}
