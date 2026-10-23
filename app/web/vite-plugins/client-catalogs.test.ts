// SPDX-License-Identifier: AGPL-3.0-only
// Loads the modules of the client-catalogs plugin from a locales directory of area files and proves that a
// locale's module merges every area file of its client catalog, and that the index lists only the locales
// with a client catalog.
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, beforeEach, expect, it } from "vitest";
import { clientCatalogs } from "./client-catalogs.ts";

let localesDir = "";

beforeEach(() => {
  localesDir = mkdtempSync(join(tmpdir(), "client-catalogs-"));
  const files: Record<string, object> = {
    "en/client/settings.json": { "settings.title": { defaultMessage: "Settings" } },
    "en/client/admin/report.json": { "admin.report.title": { defaultMessage: "Report" } },
    "ja/client/settings.json": { "settings.title": { defaultMessage: "設定" } },
    "fr/server/errors.json": { "errors.not_found": { defaultMessage: "Introuvable" } },
  };
  for (const [file, catalog] of Object.entries(files)) {
    mkdirSync(dirname(join(localesDir, file)), { recursive: true });
    writeFileSync(join(localesDir, file), JSON.stringify(catalog));
  }
});

afterEach(() => {
  rmSync(localesDir, { recursive: true, force: true });
});

/** Resolves and loads `id` through the plugin; returns the module's code and the files it watches. */
function load(id: string): { code: string; watched: string[] } {
  const plugin = clientCatalogs(localesDir);
  const { resolveId, load } = plugin;
  if (typeof resolveId !== "function" || typeof load !== "function") throw new Error("unexpected plugin shape");
  const resolved = resolveId.call({} as never, id, undefined, {} as never);
  if (typeof resolved !== "string") throw new Error(`${id} did not resolve`);
  const watched: string[] = [];
  const code = load.call({ addWatchFile: (file: string) => watched.push(file) } as never, resolved);
  if (typeof code !== "string") throw new Error(`${id} did not load`);
  return { code, watched };
}

it("merges the area files of a locale's client catalog", () => {
  const { code, watched } = load("virtual:locales/en/client");
  const json = /^export default JSON\.parse\((.*)\);$/m.exec(code)?.[1] ?? "";
  expect(JSON.parse(JSON.parse(json) as string)).toEqual({
    "admin.report.title": { defaultMessage: "Report" },
    "settings.title": { defaultMessage: "Settings" },
  });
  expect(watched).toHaveLength(2);
});

it("lists the locales with a client catalog", () => {
  const { code } = load("virtual:locales");
  expect(code).toContain('"en": () => import("virtual:locales/en/client")');
  expect(code).toContain('"ja": () => import("virtual:locales/ja/client")');
  expect(code).not.toContain("fr");
});
