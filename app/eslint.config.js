// SPDX-License-Identifier: AGPL-3.0-only
// ESLint for the client workspace (ADR-0025, ADR-0026, ADR-0027). Each block implements rows of the
// 強制 column in docs/architecture/web-client.md; the row IDs (R1, A5, ...) are noted next to the rules.
// `make web-lint` runs it with --max-warnings 0, so a warning fails like an error.
import eslintComments from "@eslint-community/eslint-plugin-eslint-comments/configs";
import eslintReact from "@eslint-react/eslint-plugin";
import js from "@eslint/js";
import tanstackQuery from "@tanstack/eslint-plugin-query";
import tanstackRouter from "@tanstack/eslint-plugin-router";
import betterTailwind from "eslint-plugin-better-tailwindcss";
import boundaries from "eslint-plugin-boundaries";
import formatjs from "eslint-plugin-formatjs";
import reactHooks from "eslint-plugin-react-hooks";
import testingLibrary from "eslint-plugin-testing-library";
import { defineConfig, globalIgnores } from "eslint/config";
import globals from "globals";
import tseslint from "typescript-eslint";
import * as restrict from "./eslint/restrictions.js";

const src = ["ui/src/**/*.{ts,tsx}", "web/src/**/*.{ts,tsx}"];
const tests = ["**/*.test.{ts,tsx}"];
const stories = ["**/*.stories.{ts,tsx}"];
/** The Storybook preview and the story test setup of @my/ui, which run in the browser like the kit. */
const storybookPreview = ["ui/.storybook/preview.tsx", "ui/.storybook/vitest.setup.ts"];
/** Components: Tailwind rules for D2 apply here (the app shell may use viewport breakpoints). */
const components = ["ui/src/**/*.tsx", "web/src/features/**/*.tsx"];
/** Where the API client and fetch may be used (S1); src/test holds the MSW setup and its own tests. */
const apiAllowed = ["web/src/features/*/api/**", "web/src/lib/**", "web/src/test/**"];

/** Rules @eslint-react duplicates from eslint-plugin-react-hooks; the react-hooks/* IDs are canonical. */
const eslintReactHookDuplicates = Object.fromEntries(
  [
    "error-boundaries",
    "exhaustive-deps",
    "globals",
    "immutability",
    "purity",
    "refs",
    "rules-of-hooks",
    "set-state-in-effect",
    "set-state-in-render",
    "static-components",
    "unsupported-syntax",
    "use-memo",
  ].map((rule) => [`@eslint-react/${rule}`, "off"]),
);

export default defineConfig(
  globalIgnores(["**/node_modules/", "**/dist/", "**/storybook-static/"]),

  // --- Every file -----------------------------------------------------------------------------
  js.configs.recommended,
  tseslint.configs.strictTypeChecked,
  eslintComments.recommended,
  {
    linterOptions: { reportUnusedDisableDirectives: "error", reportUnusedInlineConfigs: "error" },
    languageOptions: {
      parserOptions: {
        project: ["./ui/tsconfig.json", "./ui/tsconfig.node.json", "./ui/scripts/tsconfig.json", "./web/tsconfig.json", "./web/tsconfig.node.json"],
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      // Numbers format unambiguously in messages; only objects, nullish values and the like are rejected.
      "@typescript-eslint/restrict-template-expressions": ["error", { allowNumber: true }],
      // A disable comment must say why (R5, Y2: the documented escape hatches).
      "@eslint-community/eslint-comments/require-description": ["error", { ignore: [] }],
      "@eslint-community/eslint-comments/no-unlimited-disable": "error",
      // R9: the React rules are never switched off.
      "@eslint-community/eslint-comments/no-restricted-disable": ["error", "react-hooks/*"],
    },
  },
  {
    files: ["**/*.js"],
    extends: [tseslint.configs.disableTypeChecked],
    languageOptions: { globals: globals.node },
  },
  {
    files: ["web/vite.config.ts", "ui/*.config.ts", "ui/.storybook/main.ts", "ui/scripts/**/*.ts"],
    languageOptions: { globals: globals.node },
  },

  // --- Client source (@my/ui and app/web) -----------------------------------------------------
  {
    files: [...src, ...storybookPreview],
    extends: [
      // R1–R6, R9, R11: React Compiler rules, all at error.
      reactHooks.configs.flat["recommended-latest"],
      // R11–R14
      eslintReact.configs["strict-type-checked"],
    ],
    languageOptions: { globals: globals.browser },
    settings: {
      // The workspace root has no react dependency to detect the version from.
      "react-x": { version: "19.3.0" },
      "better-tailwindcss": { entryPoint: "web/src/index.css" },
    },
    plugins: { "better-tailwindcss": betterTailwind, formatjs },
    rules: {
      ...eslintReactHookDuplicates,
      "react-hooks/exhaustive-deps": "error",
      "react-hooks/incompatible-library": "error",
      "react-hooks/unsupported-syntax": "error",
      "react-hooks/no-deriving-state-in-effects": "error",
      // A8: only PostContent may render sanitized HTML (re-enabled below).
      "@eslint-react/dom-no-dangerously-set-innerhtml": "error",
      // A7
      "@typescript-eslint/no-restricted-types": [
        "error",
        {
          types: Object.fromEntries(
            ["FC", "React.FC", "FunctionComponent", "React.FunctionComponent", "VFC", "React.VFC"].map((name) => [
              name,
              { message: "Type props explicitly instead of using React.FC (web-client.md A7)." },
            ]),
          ),
        },
      ],
      // A9
      "formatjs/no-literal-string-in-jsx": ["error", { props: { include: [["*", "{label,placeholder,title,alt}"]] } }],
      // D9 (and D8 through no-unknown-classes once the palette is reset to semantic tokens)
      ...betterTailwind.configs["correctness-error"].rules,
      "better-tailwindcss/no-duplicate-classes": "error",
      "better-tailwindcss/no-deprecated-classes": "error",
      "better-tailwindcss/enforce-canonical-classes": "error",
      "better-tailwindcss/no-unnecessary-whitespace": "error",
      // D5, D6, D7
      "better-tailwindcss/no-restricted-classes": ["error", { restrict: restrict.classes }],
      // A5, D5, D10, R5, R13
      "no-restricted-syntax": ["error", ...restrict.syntax],
      // R5, S1
      "no-restricted-imports": ["error", restrict.apiImports],
      "no-restricted-globals": ["error", ...restrict.fetchGlobal],
      "no-restricted-properties": ["error", ...restrict.fetchProperties],
    },
  },
  {
    // S1: the API client and fetch are allowed here.
    files: apiAllowed,
    rules: {
      "no-restricted-imports": ["error", { paths: restrict.importPaths }],
      "no-restricted-globals": "off",
      "no-restricted-properties": "off",
    },
  },
  {
    // D2
    files: components,
    rules: {
      "better-tailwindcss/no-restricted-classes": ["error", { restrict: [...restrict.classes, ...restrict.componentClasses] }],
    },
  },
  {
    // A8: the single audited component that renders server-sanitized post HTML.
    files: ["**/PostContent.tsx"],
    rules: { "@eslint-react/dom-no-dangerously-set-innerhtml": "off" },
  },

  // --- Server state and routing (S1, S7) ------------------------------------------------------
  {
    files: ["web/src/**/*.{ts,tsx}"],
    extends: [tanstackQuery.configs["flat/recommended-strict"], tanstackRouter.configs["flat/recommended"]],
  },

  // --- Dependency direction (A1–A3) -----------------------------------------------------------
  {
    files: src,
    plugins: { boundaries },
    settings: {
      "import/resolver": { typescript: { alwaysTryTypes: true, noWarnOnMultipleProjects: true, project: ["ui/tsconfig.json", "web/tsconfig.json"] } },
      "boundaries/root-path": import.meta.dirname,
      "boundaries/flag-as-external": { outsideRootPath: true },
      "boundaries/elements": [
        { type: "ui", pattern: "ui/src", partialMatch: false },
        { type: "route", pattern: "web/src/routes", partialMatch: false },
        { type: "feature", pattern: "web/src/features/*", capture: ["feature"], partialMatch: false },
        { type: "lib", pattern: "web/src/lib", partialMatch: false },
        // main.tsx, App.tsx and the router wiring: the app shell assembles everything.
        { type: "shell", pattern: "web/src", partialMatch: false },
      ],
    },
    rules: {
      "boundaries/no-unknown-files": "error",
      "boundaries/no-unknown-dependencies": "error",
      "boundaries/dependencies": [
        "error",
        {
          default: "disallow",
          // Check package imports too, for the @my/ui allowlist (A2). Imports inside one element are not checked.
          checkAllOrigins: true,
          message: "{{from.element.types}} must not import {{to.element.types}}{{to.module.source}} (web-client.md A1–A3).",
          policies: [
            { allow: { to: { module: { origin: ["external", "core"] } } } },
            { from: { element: { type: "shell" } }, allow: { to: { element: { type: ["shell", "route", "lib", "ui"] } } } },
            {
              from: { element: { type: ["shell", "route", "feature"] } },
              allow: { to: { element: { type: "feature", fileInternalPath: "index.ts" } } },
            },
            { from: { element: { type: "route" } }, allow: { to: { element: { type: ["lib", "ui"] } } } },
            { from: { element: { type: "feature" } }, allow: { to: { element: { type: ["lib", "ui"] } } } },
            // A2: @my/ui knows only React, React Aria and the Tailwind helpers.
            {
              from: { element: { type: "ui" } },
              disallow: { to: { module: { origin: "external" } } },
              message: "@my/ui depends only on React, React Aria Components and the Tailwind helpers, not {{to.module.source}} (web-client.md A2).",
            },
            {
              from: { element: { type: "ui" } },
              allow: {
                to: {
                  module: {
                    origin: "external",
                    source: ["react", "react/*", "react-dom", "react-aria-components", "react-aria", "react-stately", "tailwind-variants", "tailwind-merge"],
                  },
                },
              },
            },
            // T5: stories also import the Storybook types; components never do.
            {
              from: { element: { type: "ui", fileInternalPath: "**/*.stories.tsx" } },
              allow: { to: { module: { origin: "external", source: ["@storybook/react-vite"] } } },
            },
          ],
        },
      ],
    },
  },

  // --- Tests and stories (Y2, T2, T3) ---------------------------------------------------------
  {
    files: tests,
    extends: [testingLibrary.configs["flat/react"]],
    rules: {
      "testing-library/prefer-user-event": "error",
      "testing-library/no-test-id-queries": "error",
      "no-restricted-syntax": ["error", ...restrict.syntax, ...restrict.testSyntax],
    },
  },
  {
    files: [...tests, ...stories],
    rules: { "formatjs/no-literal-string-in-jsx": "off" },
  },
);
