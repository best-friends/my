// SPDX-License-Identifier: AGPL-3.0-only
// Project-specific restrictions from docs/architecture/web-client.md, as options for ESLint's
// no-restricted-* core rules and better-tailwindcss/no-restricted-classes. eslint.config.js composes
// them per path; restrictions.test.js proves each one rejects a violation.

/** @typedef {{ selector: string, message: string }} SyntaxRestriction */
/** @typedef {{ pattern: string, message: string }} ClassRestriction */

/** Any chain of Tailwind variants (`hover:`, `md:`, `[&>*]:`) in front of a utility. */
const variants = String.raw`(?:[^:\s]+:)*`;

/** @type {SyntaxRestriction[]} Every client source file. */
export const syntax = [
  {
    // A5: interactive elements come from @my/ui (React Aria Components).
    selector: String.raw`JSXOpeningElement[name.name=/^(div|span)$/] > JSXAttribute[name.name=/^on(Click|DoubleClick|KeyDown|KeyUp|KeyPress|MouseDown|MouseUp|PointerDown|PointerUp)(Capture)?$/]`,
    message: "Do not make a <div> or <span> interactive; use a @my/ui component built on React Aria Components (web-client.md A5).",
  },
  {
    // D5
    selector: String.raw`Literal[value=/\b100vh\b/], TemplateElement[value.raw=/\b100vh\b/]`,
    message: "Do not use 100vh; use svh / dvh (web-client.md D5).",
  },
  {
    // D10
    selector: String.raw`CallExpression[callee.property.name='setAttribute'][arguments.0.value=/^style$/i]`,
    message: "Do not set the style attribute; it breaks the CSP. Use React's style={} (web-client.md D10).",
  },
  {
    // D10
    selector: "MemberExpression[property.name='cssText']",
    message: "Do not write cssText; it breaks the CSP. Use React's style={} (web-client.md D10).",
  },
  {
    // R13
    selector: "AssignmentExpression > MemberExpression.left[property.name='defaultProps']",
    message: "Do not use defaultProps; use default parameter values (web-client.md R13).",
  },
  {
    // R5: the named imports are covered by no-restricted-imports.
    selector: "MemberExpression[object.name='React'][property.name=/^(useMemo|useCallback|memo)$/]",
    message: "React Compiler memoizes; do not write useMemo / useCallback / memo. If one is required, disable this line with a reason (web-client.md R5).",
  },
  {
    // R4: React Compiler applies to every function; an opt-out needs a reason.
    selector: String.raw`ExpressionStatement[directive=/^use no (memo|forget)$/]`,
    message: "Do not opt out of React Compiler. If a library cannot be compiled, wrap it in one hook and disable this line with a reason (web-client.md R4).",
  },
  {
    // R4, §10: a hook read from an object (openapi-react-query's api.query.useQuery) makes React Compiler
    // skip the calling hook (react.dev "Don't dynamically use Hooks").
    selector: "CallExpression > MemberExpression.callee[object.property.name='query'][property.name=/^use[A-Z]/]",
    message:
      "Do not call the hooks of the API client (api.query.useQuery / useMutation); React Compiler skips the calling hook. Pass api.query.queryOptions(...) to useQuery, and a function of features/<x>/api to useMutation (web-client.md R4, §10).",
  },
  {
    // S7: "static" ignores invalidation, so a loader would show what a write has marked stale (ADR-0045).
    selector: "Property[key.name='staleTime'][value.value='static']",
    message: 'Do not use staleTime: "static"; it ignores invalidation. A loader reads with staleTime: Infinity (web-client.md S7, ADR-0045).',
  },
  {
    // S7: ensureQueryData is deprecated in favor of queryClient.query.
    selector: "CallExpression > MemberExpression.callee[property.name=/^ensure(Infinite)?QueryData$/]",
    message: "Do not use ensureQueryData; a loader reads with queryClient.query / infiniteQuery and staleTime: Infinity (web-client.md S7).",
  },
];

/** Modules that may be mocked only through MSW (T3): the API client and feature API modules. */
const apiModule = String.raw`/(^|\/)api(\/|$)|^openapi-(fetch|react-query)$/`;

/** @type {SyntaxRestriction[]} Test files, on top of `syntax`. */
export const testSyntax = [
  {
    selector: `CallExpression[callee.object.name='vi'][callee.property.name=/^(mock|doMock)$/][arguments.0.value=${apiModule}]`,
    message: "Do not mock API modules; mock the network with MSW (web-client.md T3).",
  },
  {
    selector: `CallExpression[callee.object.name='vi'][callee.property.name=/^(mock|doMock)$/][arguments.0.source.value=${apiModule}]`,
    message: "Do not mock API modules; mock the network with MSW (web-client.md T3).",
  },
  {
    selector: "CallExpression[callee.object.name='vi'][callee.property.name=/^(spyOn|stubGlobal)$/] > Literal.arguments[value='fetch']",
    message: "Do not mock fetch; mock the network with MSW (web-client.md T3).",
  },
  {
    selector: "AssignmentExpression > MemberExpression.left[property.name='fetch']",
    message: "Do not replace fetch; mock the network with MSW (web-client.md T3).",
  },
  {
    // T7: vi.waitFor has a fixed 1 s limit and does not wrap act(); Testing Library's waitFor uses the timeout of the test setup.
    selector: "CallExpression[callee.object.name='vi'][callee.property.name='waitFor']",
    message: "Wait with findBy or Testing Library's waitFor, which use the timeout of the test setup (web-client.md T7).",
  },
];

/** no-restricted-imports `paths` for every client source file. */
const importPaths = [
  {
    name: "react",
    importNames: ["useMemo", "useCallback", "memo"],
    message: "React Compiler memoizes; do not write useMemo / useCallback / memo. If one is required, disable this line with a reason (web-client.md R5).",
  },
];

const apiMessage = "Server data goes through TanStack Query: only features/<x>/api and lib/ use the API client (web-client.md S1).";

const iconMessage =
  "Icons come from the @my/ui icon set: only app/ui/src/Icons.tsx imports react-icons, and only Phosphor (react-icons/pi) (ADR-0043, web-client.md G10).";

/** no-restricted-imports patterns for react-icons in every file but app/ui/src/Icons.tsx (G10). */
const iconPatterns = [{ regex: String.raw`^react-icons(/|$)`, message: iconMessage }];

/** no-restricted-imports patterns in app/ui/src/Icons.tsx: Phosphor only, no other set of react-icons (G10). */
const iconSetPatterns = [{ regex: String.raw`^react-icons(?!/pi$)(/|$)`, message: iconMessage }];

const apiPatterns = [{ regex: String.raw`(^|/)lib/api(/|$)`, message: apiMessage }];
const apiPaths = [...importPaths, { name: "openapi-fetch", message: apiMessage }, { name: "openapi-react-query", message: apiMessage }];

/** no-restricted-imports options outside features/<x>/api and lib/ (S1, G10). */
export const apiImports = { paths: apiPaths, patterns: [...apiPatterns, ...iconPatterns] };

/** no-restricted-imports options in features/<x>/api and lib/, where the API client is allowed (S1, G10). */
export const apiAllowedImports = { paths: importPaths, patterns: iconPatterns };

/** no-restricted-imports options in app/ui/src/Icons.tsx, the one file that imports react-icons (S1, G10). */
export const iconSetImports = { paths: apiPaths, patterns: [...apiPatterns, ...iconSetPatterns] };

/** no-restricted-globals / no-restricted-properties for `fetch` outside features/<x>/api and lib/ (S1). */
export const fetchGlobal = [{ name: "fetch", message: apiMessage }];
export const fetchProperties = ["window", "globalThis", "self"].map((object) => ({ object, property: "fetch", message: apiMessage }));

/** @type {ClassRestriction[]} Tailwind classes forbidden in every client source file. */
export const classes = [
  {
    // D7: physical directions, except under rtl: / ltr: (direction-specific icon flips, mask-l / mask-r edge fades).
    pattern: String.raw`^(?!${variants}(?:rtl|ltr):)${variants}!?-?(?:m[lr]|p[lr]|scroll-[mp][lr]|left|right|rounded-[lr]|rounded-[tb][lr]|border-[lr]|mask-[lr]|text-(?:left|right)|float-(?:left|right)|clear-(?:left|right))(?:-|$)`,
    message: "Use logical utilities (ms- / me- / ps- / pe- / start- / end- / text-start / rounded-s / border-s); physical directions are allowed only under rtl: / ltr: (web-client.md D7).",
  },
  {
    // D5
    pattern: String.raw`^${variants}!?(?:min-|max-)?h-screen!?$|100vh`,
    message: "Do not use 100vh (h-screen); use h-svh / h-dvh (web-client.md D5).",
  },
  {
    // D6
    pattern: String.raw`^(?!${variants}motion-(?:safe|reduce):)${variants}!?animate-(?!none)`,
    message: "Put animations under motion-safe: (web-client.md D6).",
  },
];

/** @type {ClassRestriction[]} Components (@my/ui and features), on top of `classes`. */
export const componentClasses = [
  {
    // D2: container queries (@md:) are fine; viewport breakpoints belong to the app shell.
    pattern: String.raw`^${variants}(?:max-)?(?:sm|md|lg|xl|2xl|min-\[[^\]]*\]|max-\[[^\]]*\]):`,
    message: "Components use container queries (@container, @md:); viewport breakpoints belong to the app shell (web-client.md D2).",
  },
];
