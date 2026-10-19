// SPDX-License-Identifier: AGPL-3.0-only
// Proves that each project-specific restriction of app/.oxlintrc.json rejects a violation and accepts the
// intended alternative, with the options the workspace is linted with for a file of each kind. The rules of
// JS plugins (restrict/*, better-tailwindcss/no-restricted-classes) run in Oxlint's RuleTester; Oxlint's
// native rules (no-restricted-imports / -globals / -properties) run in the oxlint binary, which is the only
// way to run them. Run with `node --test` from app/ (part of `make web-lint`): the RuleTester needs Node.js.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";
import betterTailwind from "eslint-plugin-better-tailwindcss";
import { RuleTester } from "oxlint/plugins-dev";
import { appDir, ruleOptions, ruleValue } from "./config.js";
import restrict from "./restrictions.js";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({ languageOptions: { parserOptions: { lang: "tsx" } } });

/** A file of each kind, whose options the cases use. */
const files = {
  route: "web/src/routes/index.tsx",
  featureHook: "web/src/features/timeline/hooks/useTimeline.ts",
  featureComponent: "web/src/features/timeline/components/Timeline.tsx",
  featureApi: "web/src/features/timeline/api/timelines.ts",
  featureTest: "web/src/features/timeline/timeline.test.tsx",
  featureComponentTest: "web/src/features/timeline/components/Timeline.test.tsx",
  lib: "web/src/lib/services.ts",
  icons: "ui/src/Icons.tsx",
  component: "ui/src/Button.tsx",
  svgControl: "ui/src/Checkbox.tsx",
  story: "ui/src/EmptyState.stories.tsx",
};

/** @param {string} code */
const fails = (code, errors = 1) => ({ code, errors });

describe("where the restrictions apply", () => {
  it("draws SVG only in the controls that list it, stories of them and tests (G10)", () => {
    assert.ok(ruleOptions("restrict/svg", files.component).length > 0);
    assert.ok(ruleOptions("restrict/svg", files.featureComponent).length > 0);
    for (const file of [files.svgControl, files.story, files.featureTest]) assert.equal(ruleValue("restrict/svg", file), "off");
  });
  it("keeps cache writes out of a feature's hooks and components, but not out of their tests (S1)", () => {
    assert.ok(ruleOptions("restrict/feature", files.featureComponent).length > 0);
    for (const file of [files.featureApi, files.featureComponentTest, files.route]) assert.equal(ruleValue("restrict/feature", file), undefined);
  });
  it("allows the API client and fetch in features/<x>/api and lib/ (S1)", () => {
    for (const file of [files.featureApi, files.lib]) {
      assert.equal(ruleValue("no-restricted-globals", file), "off");
      assert.equal(ruleValue("no-restricted-properties", file), "off");
    }
  });
});

const syntax = ruleOptions("restrict/syntax", files.route);
ruleTester.run("restrict/syntax (every file)", restrict.rules.syntax, {
  valid: [
    { code: "<button onClick={f} />", options: syntax },
    { code: "<div onScroll={f} />", options: syntax },
    { code: "const h = '100dvh';", options: syntax },
    { code: "el.setAttribute('aria-label', x);", options: syntax },
    { code: "<p style={{ color: 'red' }} />", options: syntax },
    { code: "function C({ a = 1 }) { return a; }", options: syntax },
    { code: "function useX() { 'use strict'; return 'use no memo'; }", options: syntax },
    { code: "useQuery(api.query.queryOptions('get', '/instance'));", options: syntax },
    { code: "useMutation({ mutationFn: (body) => createStatus(api, body) });", options: syntax },
    { code: "queryClient.query({ ...options, staleTime: Infinity });", options: syntax },
  ],
  invalid: [
    // A5
    { ...fails("<div onClick={f} />"), options: syntax },
    { ...fails("<span onKeyDown={f} />"), options: syntax },
    { ...fails("<div onPointerDownCapture={f} />"), options: syntax },
    // D5
    { ...fails("const h = '100vh';"), options: syntax },
    { ...fails("const h = `calc(100vh - ${x})`;"), options: syntax },
    // D10
    { ...fails("el.setAttribute('style', 'color: red');"), options: syntax },
    { ...fails("el.style.cssText = 'color: red';"), options: syntax },
    // R13
    { ...fails("C.defaultProps = { a: 1 };"), options: syntax },
    // R5
    { ...fails("React.useMemo(() => 1, []);"), options: syntax },
    { ...fails("export default React.memo(C);"), options: syntax },
    // R4
    { ...fails("function useX() { 'use no memo'; return 1; }"), options: syntax },
    { ...fails("'use no forget';"), options: syntax },
    { ...fails("api.query.useQuery('get', '/instance');"), options: syntax },
    { ...fails("const m = services.api.query.useMutation('post', '/statuses');"), options: syntax },
    // S7
    { ...fails("queryClient.query({ ...options, staleTime: 'static' });"), options: syntax },
    { ...fails("queryClient.ensureQueryData(options);"), options: syntax },
    { ...fails("context.queryClient.ensureInfiniteQueryData(options);"), options: syntax },
  ],
});

const svg = ruleOptions("restrict/svg", files.component);
ruleTester.run("restrict/svg (G10)", restrict.rules.svg, {
  valid: [
    { code: "<ReplyIcon />", options: svg },
    { code: "<Button><StarIcon filled /></Button>", options: svg },
    { code: "<img src='/logo.svg' alt='' />", options: svg },
    { code: "<a href='#x'><span>x</span></a>", options: svg },
    { code: "const svg = document.querySelector('svg');", options: svg },
  ],
  invalid: [
    { ...fails("<svg viewBox='0 0 24 24' />"), options: svg },
    { ...fails("<svg aria-hidden='true'><path d='M0 0h24' /></svg>", 2), options: svg },
    { ...fails("<><circle cx={12} cy={12} r={3} /><rect width={4} height={4} /></>", 2), options: svg },
    { ...fails("<g><use href='#icon' /></g>", 2), options: svg },
    { ...fails("<linearGradient id='a'><stop offset='0' /></linearGradient>", 2), options: svg },
  ],
});

const feature = ruleOptions("restrict/feature", files.featureHook);
ruleTester.run("restrict/feature (a feature's hooks and components)", restrict.rules.feature, {
  valid: [
    { code: "useMutation({ ...createStatusMutation(api), onSuccess: (status) => cachePostedStatus(api, queryClient, status) });", options: feature },
    { code: "const job = queryClient.getQueryData(key);", options: feature },
    { code: "const last = queryClient.isMutating({ mutationKey }) <= 1;", options: feature },
  ],
  invalid: [
    // S1
    { ...fails("queryClient.setQueryData(key, value);"), options: feature },
    { ...fails("void queryClient.invalidateQueries({ queryKey });"), options: feature },
    { ...fails("client.setQueriesData({ predicate }, update);"), options: feature },
    { ...fails("queryClient.removeQueries({ queryKey, exact: true });"), options: feature },
  ],
});

const test = ruleOptions("restrict/test", files.featureTest);
ruleTester.run("restrict/test (tests)", restrict.rules.test, {
  valid: [
    { code: "vi.mock('./useClock');", options: test },
    { code: "vi.spyOn(console, 'error');", options: test },
    { code: "waitFor(() => expect(a).toBe(1));", options: test },
  ],
  invalid: [
    // T3
    { ...fails("vi.mock('../api/queries');"), options: test },
    { ...fails("vi.mock(import('../../lib/api'));"), options: test },
    { ...fails("vi.doMock('openapi-fetch');"), options: test },
    { ...fails("vi.spyOn(globalThis, 'fetch');"), options: test },
    { ...fails("vi.stubGlobal('fetch', vi.fn());"), options: test },
    { ...fails("globalThis.fetch = vi.fn();"), options: test },
    // T7
    { ...fails("vi.waitFor(() => expect(a).toBe(1));"), options: test },
  ],
});

const tailwindSettings = { "better-tailwindcss": { entryPoint: path.join(appDir, "web/src/index.css") } };
/** @param {string} className */
const jsx = (className) => `<p className="${className}" />`;
const classes = ruleOptions("better-tailwindcss/no-restricted-classes", files.route);
const componentClasses = ruleOptions("better-tailwindcss/no-restricted-classes", files.featureComponent);

ruleTester.run("better-tailwindcss/no-restricted-classes", betterTailwind.rules["no-restricted-classes"], {
  valid: [
    // D7
    ...["ms-2 me-2 ps-2 pe-2 start-0 end-0 text-start text-end rounded-s border-e", "rtl:-scale-x-100 ltr:ml-2 md:rtl:mr-2 data-open:ltr:mask-l-from-50%", "mx-2 px-2 border-lime-500 rounded-lg place-items-center mask-x-from-90% mask-linear-50 mask-radial-from-50%"].map(
      (c) => ({ code: jsx(c), options: classes, settings: tailwindSettings }),
    ),
    // D5, D6
    ...["min-h-svh h-dvh", "motion-safe:animate-spin animate-none"].map((c) => ({ code: jsx(c), options: classes, settings: tailwindSettings })),
    // D2: container queries in components, breakpoints in the shell
    { code: jsx("@container @md:flex"), options: componentClasses, settings: tailwindSettings },
    { code: jsx("md:flex"), options: classes, settings: tailwindSettings },
  ],
  invalid: [
    // D7
    ...["ml-2", "-mr-1", "pl-4", "hover:pr-4", "left-0", "right-2", "text-left", "text-right", "rounded-l-lg", "rounded-tr", "border-l", "border-r-2", "float-left", "scroll-ml-4", "mask-l-from-50%", "data-open:mask-r-to-8"].map(
      (c) => ({ ...fails(jsx(c)), options: classes, settings: tailwindSettings }),
    ),
    // D5
    ...["h-screen", "min-h-screen", "md:max-h-screen", "h-[100vh]"].map((c) => ({ ...fails(jsx(c)), options: classes, settings: tailwindSettings })),
    // D6
    ...["animate-spin", "hover:animate-bounce"].map((c) => ({ ...fails(jsx(c)), options: classes, settings: tailwindSettings })),
    // D2
    ...["md:flex", "max-lg:hidden", "min-[40rem]:grid"].map((c) => ({ ...fails(jsx(c)), options: componentClasses, settings: tailwindSettings })),
  ],
});

// --- Oxlint's native rules, in the oxlint binary ------------------------------------------------

const oxlint = path.join(appDir, "node_modules/.bin/oxlint");
const scratch = mkdtempSync(path.join(tmpdir(), "oxlint-restrictions-"));
after(() => rmSync(scratch, { recursive: true, force: true }));
let runs = 0;

/**
 * Lints each case alone with only `rule`, configured as the workspace configures it for `file`.
 * @param {string} rule
 * @param {string} file
 * @param {{ valid: string[], invalid: { code: string, errors: number }[] }} cases
 */
function runNative(rule, file, cases) {
  const dir = path.join(scratch, String(runs++));
  const all = [...cases.valid.map((code) => ({ code, errors: 0 })), ...cases.invalid];
  mkdirSync(dir);
  writeFileSync(path.join(dir, ".oxlintrc.json"), JSON.stringify({ plugins: [], categories: { correctness: "off" }, rules: { [rule]: ruleValue(rule, file) } }));
  all.forEach(({ code }, i) => writeFileSync(path.join(dir, `case${String(i)}.tsx`), code + "\n"));
  const result = spawnSync(oxlint, ["-c", path.join(dir, ".oxlintrc.json"), "-f", "json", dir], { encoding: "utf8" });
  /** @type {{ diagnostics: { code: string, filename: string }[] }} */
  const report = JSON.parse(result.stdout);
  describe(`${rule} (${file})`, () => {
    all.forEach(({ code, errors }, i) => {
      it(code, () => {
        const name = `case${String(i)}.tsx`;
        const found = report.diagnostics.filter((d) => path.basename(d.filename) === name);
        assert.equal(found.length, errors, found.map((d) => d.code).join(", "));
      });
    });
  });
}

/** @param {string} code */
const native = (code, errors = 1) => ({ code, errors });

runNative("no-restricted-imports", files.route, {
  valid: ["import { useState } from 'react';", "import { timeline } from '../api';"],
  invalid: [
    // R5
    native("import { useMemo, useCallback } from 'react';", 2),
    // S1
    native("import createClient from 'openapi-fetch';"),
    native("import { client } from '../../lib/api';"),
    native("import { client } from '../../lib/api/client';"),
    // G10: react-icons only in Icons.tsx
    native("import { PiStar } from 'react-icons/pi';"),
    native("import type { IconType } from 'react-icons';"),
  ],
});
// Inside features/<x>/api and lib/
runNative("no-restricted-imports", files.featureApi, {
  valid: ["import createClient from 'openapi-fetch';", "import { client } from '../../../lib/api';"],
  invalid: [native("import { memo } from 'react';"), native("import { PiStar } from 'react-icons/pi';")],
});
// Inside app/ui/src/Icons.tsx: Phosphor only
runNative("no-restricted-imports", files.icons, {
  valid: ["import { PiStar } from 'react-icons/pi';", "import { Button } from './Button';"],
  invalid: [
    native("import { client } from '../../lib/api';"),
    native("import { FaStar } from 'react-icons/fa';"),
    native("import type { IconType } from 'react-icons';"),
    native("import { IconBase } from 'react-icons/lib';"),
    native("import { PiStar } from 'react-icons/pip';"),
  ],
});
// S1
runNative("no-restricted-globals", files.route, { valid: ["client.GET('/x');"], invalid: [native("fetch('/x');")] });
runNative("no-restricted-properties", files.route, { valid: ["client.fetch('/x');"], invalid: [native("window.fetch('/x');")] });
