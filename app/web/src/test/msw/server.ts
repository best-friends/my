// SPDX-License-Identifier: AGPL-3.0-only
import { setupServer } from "msw/node";
import * as modules from "./handlers";

/** The default handlers of every module listed in handlers.ts. */
export const defaultHandlers = Object.values(modules).flatMap((source) => source.handlers);

/**
 * The MSW server for Vitest, started by src/test/setup.ts with the default handlers.
 * A test overrides a response with `server.use(...)`; setup.ts resets the overrides after each test.
 */
export const server = setupServer(...defaultHandlers);
