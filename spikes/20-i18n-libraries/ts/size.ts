// SPDX-License-Identifier: AGPL-3.0-only
// Measures the minified + gzipped cost of each TS candidate for formatting one
// plural message, using Bun's bundler (browser target, production define).
//
//   bun run ts/size.ts
import { gzipSync } from "bun";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { root } from "./catalog";

const MSG = "{count, plural, =0 {No unread notifications} one {# unread notification} other {# unread notifications}}";

const entries: Record<string, { code: string; noParser?: boolean }> = {
  "formatjs intl-messageformat (runtime parse)": {
    code: `import { IntlMessageFormat } from "intl-messageformat";
console.log(new IntlMessageFormat(${JSON.stringify(MSG)}, "en").format({ count: 2 }));`,
  },
  "formatjs intl-messageformat (precompiled AST, no-parser)": {
    noParser: true,
    code: `import { IntlMessageFormat } from "intl-messageformat";
import ast from "./ast.json";
console.log(new IntlMessageFormat(ast, "en").format({ count: 2 }));`,
  },
  "lingui @lingui/core (precompiled)": {
    code: `import { i18n } from "@lingui/core";
i18n.load("en", { m: [["count", "plural", { 0: "No unread notifications", one: ["#", " unread notification"], other: ["#", " unread notifications"] }]] });
i18n.activate("en");
console.log(i18n._("m", { count: 2 }));`,
  },
  "lingui @lingui/core + runtime compileMessage": {
    code: `import { i18n } from "@lingui/core";
import { compileMessage } from "@lingui/message-utils/compileMessage";
i18n.setMessagesCompiler(compileMessage);
i18n.load("en", { m: ${JSON.stringify(MSG)} });
i18n.activate("en");
console.log(i18n._("m", { count: 2 }));`,
  },
  "i18next + i18next-icu": {
    code: `import i18next from "i18next";
import ICU from "i18next-icu";
await i18next.use(ICU).init({ lng: "en", resources: { en: { translation: { m: ${JSON.stringify(MSG)} } } } });
console.log(i18next.t("m", { count: 2 }));`,
  },
  "@messageformat/core (runtime compile)": {
    code: `import MessageFormat from "@messageformat/core";
console.log(new MessageFormat("en").compile(${JSON.stringify(MSG)})({ count: 2 }));`,
  },
};

const tmp = join(root, "ts", "size");
rmSync(tmp, { recursive: true, force: true });
mkdirSync(tmp, { recursive: true });
const { parse } = await import("@formatjs/icu-messageformat-parser");
writeFileSync(join(tmp, "ast.json"), JSON.stringify(parse(MSG)));

const rows: string[] = [];
for (const [name, { code, noParser }] of Object.entries(entries)) {
  const file = join(tmp, `${rows.length}.ts`);
  writeFileSync(file, code);
  const result = await Bun.build({
    entrypoints: [file],
    target: "browser",
    minify: true,
    define: { "process.env.NODE_ENV": '"production"' },
    plugins: noParser
      ? [
          {
            name: "no-parser",
            setup(b) {
              b.onResolve({ filter: /^@formatjs\/icu-messageformat-parser$/ }, () => ({
                path: require.resolve("@formatjs/icu-messageformat-parser/no-parser.js"),
              }));
            },
          },
        ]
      : [],
  });
  if (!result.success) throw new AggregateError(result.logs, name);
  const bytes = new Uint8Array(await result.outputs[0].arrayBuffer());
  rows.push(`| ${name} | ${(bytes.length / 1024).toFixed(1)} | ${(gzipSync(bytes).length / 1024).toFixed(1)} |`);
}
rmSync(tmp, { recursive: true, force: true });
console.log("| candidate | min (KiB) | min+gzip (KiB) |\n| --- | ---: | ---: |");
console.log(rows.join("\n"));
