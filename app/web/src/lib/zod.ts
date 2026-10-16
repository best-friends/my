// SPDX-License-Identifier: AGPL-3.0-only
// What `import … from "zod"` resolves to in the app (the alias in vite.config.ts): Zod with its JIT turned
// off. Zod compiles each object schema with `new Function` when the page allows eval, and decides that when
// the schema is created, at module evaluation. Its probe for eval is itself a Trusted Types violation under
// the shell's CSP (`require-trusted-types-for 'script'`, ADR-0019 §9). Every importer of "zod" evaluates
// after this module, so no schema is created before `jitless` is set ("Disable JIT schema compilation.
// Useful in environments that disallow `eval`.", the $ZodConfig type of zod/v4/core).
import { config } from "zod/v4/core";

config({ jitless: true });

export * from "zod/v4";
export { default } from "zod/v4";
