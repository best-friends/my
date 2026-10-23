// SPDX-License-Identifier: AGPL-3.0-only
// The license notices the build copies into dist/licenses/.
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { bundledLicenses } from "./licenses.ts";

let dir: string | undefined;

afterEach(() => {
  if (dir !== undefined) rmSync(dir, { recursive: true, force: true });
  dir = undefined;
});

describe("bundledLicenses", () => {
  it("emits each notice under licenses/ with its own name", async () => {
    dir = mkdtempSync(join(tmpdir(), "licenses-"));
    const file = join(dir, "LICENSE-Example.txt");
    writeFileSync(file, "EXAMPLE LICENSE\n");
    const emitted: { fileName?: string; source?: unknown }[] = [];
    const generateBundle = bundledLicenses([file]).generateBundle;
    if (typeof generateBundle !== "function") throw new Error("unexpected plugin shape");
    const context = { emitFile: (asset: { fileName?: string; source?: unknown }) => emitted.push(asset) };
    await generateBundle.call(context as never, {} as never, {}, false);
    expect(emitted.map((a) => a.fileName)).toEqual(["licenses/LICENSE-Example.txt"]);
    expect(String(emitted[0]?.source)).toBe("EXAMPLE LICENSE\n");
  });
});
