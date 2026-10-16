// SPDX-License-Identifier: AGPL-3.0-only
// Proves that each restriction in restrictions.js rejects a violation and accepts the intended
// alternative. Run with `bun test` from app/ (part of `make web-lint`).
import { fileURLToPath } from "node:url";
import { describe, it } from "bun:test";
import betterTailwind from "eslint-plugin-better-tailwindcss";
import { RuleTester } from "eslint";
import tseslint from "typescript-eslint";
import { builtinRules as core } from "eslint/use-at-your-own-risk";
import * as restrict from "./restrictions.js";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parser: tseslint.parser,
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
  settings: {
    "better-tailwindcss": { entryPoint: fileURLToPath(new URL("../web/src/index.css", import.meta.url)) },
  },
});

/** @param {string} code */
const fails = (code, errors = 1) => ({ code, errors });

ruleTester.run("no-restricted-syntax (every file)", core.get("no-restricted-syntax"), {
  valid: [
    { code: "<button onClick={f} />", options: restrict.syntax },
    { code: "<div onScroll={f} />", options: restrict.syntax },
    { code: "const h = '100dvh';", options: restrict.syntax },
    { code: "el.setAttribute('aria-label', x);", options: restrict.syntax },
    { code: "<p style={{ color: 'red' }} />", options: restrict.syntax },
    { code: "function C({ a = 1 }) { return a; }", options: restrict.syntax },
    { code: "function useX() { 'use strict'; return 'use no memo'; }", options: restrict.syntax },
    { code: "useQuery(api.query.queryOptions('get', '/instance'));", options: restrict.syntax },
    { code: "useMutation({ mutationFn: (body) => createStatus(api, body) });", options: restrict.syntax },
  ],
  invalid: [
    // A5
    { ...fails("<div onClick={f} />"), options: restrict.syntax },
    { ...fails("<span onKeyDown={f} />"), options: restrict.syntax },
    { ...fails("<div onPointerDownCapture={f} />"), options: restrict.syntax },
    // D5
    { ...fails("const h = '100vh';"), options: restrict.syntax },
    { ...fails("const h = `calc(100vh - ${x})`;"), options: restrict.syntax },
    // D10
    { ...fails("el.setAttribute('style', 'color: red');"), options: restrict.syntax },
    { ...fails("el.style.cssText = 'color: red';"), options: restrict.syntax },
    // R13
    { ...fails("C.defaultProps = { a: 1 };"), options: restrict.syntax },
    // R5
    { ...fails("React.useMemo(() => 1, []);"), options: restrict.syntax },
    { ...fails("export default React.memo(C);"), options: restrict.syntax },
    // R4
    { ...fails("function useX() { 'use no memo'; return 1; }"), options: restrict.syntax },
    { ...fails("'use no forget';"), options: restrict.syntax },
    { ...fails("api.query.useQuery('get', '/instance');"), options: restrict.syntax },
    { ...fails("const m = services.api.query.useMutation('post', '/statuses');"), options: restrict.syntax },
  ],
});

const testSyntax = [...restrict.syntax, ...restrict.testSyntax];
ruleTester.run("no-restricted-syntax (tests)", core.get("no-restricted-syntax"), {
  valid: [
    { code: "vi.mock('./useClock');", options: testSyntax },
    { code: "vi.spyOn(console, 'error');", options: testSyntax },
    { code: "waitFor(() => expect(a).toBe(1));", options: testSyntax },
  ],
  invalid: [
    // T3
    { ...fails("vi.mock('../api/queries');"), options: testSyntax },
    { ...fails("vi.mock(import('../../lib/api'));"), options: testSyntax },
    { ...fails("vi.doMock('openapi-fetch');"), options: testSyntax },
    { ...fails("vi.spyOn(globalThis, 'fetch');"), options: testSyntax },
    { ...fails("vi.stubGlobal('fetch', vi.fn());"), options: testSyntax },
    { ...fails("globalThis.fetch = vi.fn();"), options: testSyntax },
    // T7
    { ...fails("vi.waitFor(() => expect(a).toBe(1));"), options: testSyntax },
  ],
});

ruleTester.run("no-restricted-imports", core.get("no-restricted-imports"), {
  valid: [
    { code: "import { useState } from 'react';", options: [restrict.apiImports] },
    { code: "import { timeline } from '../api';", options: [restrict.apiImports] },
    // Inside features/<x>/api and lib/
    { code: "import createClient from 'openapi-fetch';", options: [{ paths: restrict.importPaths }] },
    { code: "import { client } from '../../../lib/api';", options: [{ paths: restrict.importPaths }] },
  ],
  invalid: [
    // R5
    { ...fails("import { useMemo, useCallback } from 'react';", 2), options: [restrict.apiImports] },
    { ...fails("import { memo } from 'react';"), options: [{ paths: restrict.importPaths }] },
    // S1
    { ...fails("import createClient from 'openapi-fetch';"), options: [restrict.apiImports] },
    { ...fails("import { client } from '../../lib/api';"), options: [restrict.apiImports] },
    { ...fails("import { client } from '../../lib/api/client';"), options: [restrict.apiImports] },
  ],
});

ruleTester.run("no-restricted-globals / no-restricted-properties (fetch)", core.get("no-restricted-globals"), {
  valid: [{ code: "client.GET('/x');", options: restrict.fetchGlobal }],
  invalid: [{ ...fails("fetch('/x');"), options: restrict.fetchGlobal }],
});
ruleTester.run("no-restricted-properties (fetch)", core.get("no-restricted-properties"), {
  valid: [{ code: "client.fetch('/x');", options: restrict.fetchProperties }],
  invalid: [{ ...fails("window.fetch('/x');"), options: restrict.fetchProperties }],
});

const classes = [{ restrict: restrict.classes }];
const componentClasses = [{ restrict: [...restrict.classes, ...restrict.componentClasses] }];
/** @param {string} className */
const jsx = (className) => `<p className="${className}" />`;
const noRestrictedClasses = betterTailwind.rules["no-restricted-classes"];

ruleTester.run("better-tailwindcss/no-restricted-classes", noRestrictedClasses, {
  valid: [
    // D7
    ...["ms-2 me-2 ps-2 pe-2 start-0 end-0 text-start text-end rounded-s border-e", "rtl:-scale-x-100 ltr:ml-2 md:rtl:mr-2", "mx-2 px-2 border-lime-500 rounded-lg place-items-center"].map(
      (c) => ({ code: jsx(c), options: classes }),
    ),
    // D5, D6
    ...["min-h-svh h-dvh", "motion-safe:animate-spin animate-none"].map((c) => ({ code: jsx(c), options: classes })),
    // D2: container queries in components, breakpoints in the shell
    { code: jsx("@container @md:flex"), options: componentClasses },
    { code: jsx("md:flex"), options: classes },
  ],
  invalid: [
    // D7
    ...["ml-2", "-mr-1", "pl-4", "hover:pr-4", "left-0", "right-2", "text-left", "text-right", "rounded-l-lg", "rounded-tr", "border-l", "border-r-2", "float-left", "scroll-ml-4"].map(
      (c) => ({ ...fails(jsx(c)), options: classes }),
    ),
    // D5
    ...["h-screen", "min-h-screen", "md:max-h-screen", "h-[100vh]"].map((c) => ({ ...fails(jsx(c)), options: classes })),
    // D6
    ...["animate-spin", "hover:animate-bounce"].map((c) => ({ ...fails(jsx(c)), options: classes })),
    // D2
    ...["md:flex", "max-lg:hidden", "min-[40rem]:grid"].map((c) => ({ ...fails(jsx(c)), options: componentClasses })),
  ],
});
