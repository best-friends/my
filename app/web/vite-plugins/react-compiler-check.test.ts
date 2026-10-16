// SPDX-License-Identifier: AGPL-3.0-only
// Runs babel-plugin-react-compiler on small modules and proves that the build gate fails on a function the
// compiler skips, prints the compiler's reason, and lets a "use no memo" opt-out through.
import { transformSync } from "@babel/core";
import reactCompiler from "babel-plugin-react-compiler";
import { describe, expect, it } from "vitest";
import { reactCompilerCheck } from "./react-compiler-check.ts";

function compile(code: string) {
  const check = reactCompilerCheck();
  transformSync(code, {
    filename: "/src/useX.ts",
    babelrc: false,
    configFile: false,
    plugins: [check.babelPlugin, [reactCompiler, { logger: check.logger }]],
  });
  return check;
}

/** Calls the plugin's buildEnd as Vite would, and returns the message it fails with. */
function buildEnd(check: ReturnType<typeof reactCompilerCheck>): string | undefined {
  const context = {
    error(message: string): never {
      throw new Error(message);
    },
    info() {},
  };
  const hook = check.plugin.buildEnd;
  if (typeof hook !== "function") throw new Error("buildEnd is not a function");
  try {
    void hook.call(context as never, undefined);
  } catch (error) {
    return (error as Error).message;
  }
  return undefined;
}

describe("reactCompilerCheck", () => {
  it("passes a hook the compiler compiles", () => {
    const check = compile(`
      import { useState } from "react";
      export function useCount() {
        const [count, setCount] = useState(0);
        return { count, increment: () => setCount(count + 1) };
      }
    `);
    expect(check.compiled()).toBe(1);
    expect(check.problems()).toEqual([]);
    expect(buildEnd(check)).toBeUndefined();
  });

  it("fails on a hook read from an object, with the compiler's reason", () => {
    // openapi-react-query's hooks are methods of the client (`api.query.useMutation`), which the compiler
    // cannot prove to be the same function on every render.
    const check = compile(`
      import { useApi } from "./api";
      export function useSend() {
        const api = useApi();
        return api.query.useMutation("post", "/statuses");
      }
    `);
    expect(check.compiled()).toBe(0);
    const [problem, ...rest] = check.problems();
    expect(rest).toEqual([]);
    expect(problem).toMatch(/^\/src\/useX\.ts:3:\d+: .*Hooks must be the same function on every render/);
    expect(buildEnd(check)).toMatch(/React Compiler skipped these functions[\s\S]*Hooks must be the same function/);
  });

  it("lets a function that opts out with \"use no memo\" through", () => {
    const check = compile(`
      import { useState } from "react";
      import { useApi } from "./api";
      export function useSend() {
        "use no memo";
        const api = useApi();
        return api.query.useMutation("post", "/statuses");
      }
      export function useCount() {
        const [count] = useState(0);
        return count;
      }
    `);
    expect(check.compiled()).toBe(1);
    expect(check.problems()).toEqual([]);
    expect(buildEnd(check)).toBeUndefined();
  });

  it("fails when the compiler compiled nothing", () => {
    expect(buildEnd(compile("export const x = 1;"))).toMatch(/compiled no function/);
  });
});
