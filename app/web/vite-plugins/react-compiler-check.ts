// SPDX-License-Identifier: AGPL-3.0-only
// The React Compiler gate of `vite build` (web-client.md R4): a logger for babel-plugin-react-compiler, a
// Babel plugin that finds the functions that opt out of it, and a Vite plugin that fails the build on what
// they saw.
import type { PluginObj } from "@babel/core";
import type { RolldownBabelPreset } from "@rolldown/plugin-babel";
import type { Plugin } from "vite";

interface SourceLocation {
  start: { line: number; column: number };
}

/** The logger events this check reads (babel-plugin-react-compiler's LoggerEvent). */
export type CompilerEvent =
  | { kind: "CompileSuccess" }
  | { kind: "CompileError"; fnLoc: SourceLocation | null; detail: { toString(): string } }
  | { kind: "PipelineError"; fnLoc: SourceLocation | null; data: string }
  | { kind: string };

/** The directives babel-plugin-react-compiler treats as an opt-out. */
const optOut = /^use no (memo|forget)$/;

const key = (filename: string | null | undefined, loc: SourceLocation) => `${filename ?? "<unknown>"}:${loc.start.line}:${loc.start.column + 1}`;

/**
 * Fails `vite build` when React Compiler compiled no function, or when it gave up on one: the error lists
 * every skipped function with the compiler's reason.
 *
 * A function that opts out with `"use no memo"` is not a failure, whatever the compiler reports about it:
 * `no-restricted-syntax` rejects the directive unless a disable comment gives the reason, so the documented
 * opt-outs are the allow-list. The compiler reports a skipped function as CompileError / PipelineError with
 * the function's location; validations that only log (no location) leave the function compiled and are the
 * lint rules' concern.
 */
export function reactCompilerCheck() {
  let compiled = 0;
  const skips = new Map<string, string[]>();
  const optOuts = new Set<string>();

  const logger = {
    logEvent(filename: string | null, event: CompilerEvent) {
      if (event.kind === "CompileSuccess") {
        compiled++;
        return;
      }
      let reason: string;
      if (event.kind === "CompileError" && "detail" in event && event.fnLoc != null) reason = event.detail.toString();
      else if (event.kind === "PipelineError" && "data" in event && event.fnLoc != null) reason = event.data;
      else return;
      const at = key(filename, event.fnLoc);
      skips.set(at, [...(skips.get(at) ?? []), reason]);
    },
  };

  /** Records the functions whose body opens with an opt-out directive, at the location the compiler reports. */
  const babelPlugin = (): PluginObj => ({
    name: "my:react-compiler-opt-outs",
    visitor: {
      Function(path, state) {
        const { body, loc } = path.node;
        if (loc != null && body.type === "BlockStatement" && body.directives.some((d) => optOut.test(d.value.value))) {
          optOuts.add(key(state.filename, loc));
        }
      },
    },
  });

  /** The compiler preset with the opt-out finder next to the compiler, on the same files. */
  const preset = (compiler: RolldownBabelPreset): RolldownBabelPreset => ({
    ...compiler,
    preset: () => ({ plugins: [babelPlugin], presets: [compiler.preset] }),
  });

  /** Every skipped function that did not opt out, as `file:line:column: reason`. */
  const problems = () =>
    [...skips].filter(([at]) => !optOuts.has(at)).flatMap(([at, reasons]) => reasons.map((reason) => `${at}: ${reason}`));

  const plugin: Plugin = {
    name: "my:react-compiler-check",
    apply: "build",
    buildEnd(error) {
      if (error) return;
      const found = problems();
      if (found.length > 0) {
        this.error(`React Compiler skipped these functions; rewrite them so that it compiles them (web-client.md R4):\n${found.join("\n")}`);
      }
      if (compiled === 0) this.error("React Compiler compiled no function; check the babel preset in vite.config.ts");
      this.info(`React Compiler compiled ${compiled} function(s)`);
    },
  };

  return { logger, babelPlugin, preset, plugin, problems, compiled: () => compiled };
}
