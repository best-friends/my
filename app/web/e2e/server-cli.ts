// SPDX-License-Identifier: AGPL-3.0-only
// The configuration of the `my serve` that e2e/server.ts starts, and `bin/my` commands against its
// database: e2e/server.ts seeds the database with them, and the specs of the main flow
// (e2e/main-flow.spec.ts) create users and approve sign-ups with them while the server runs, as an
// operator does (docs/operations/registration.md).
import { execFileSync } from "node:child_process";
import { join, resolve } from "node:path";
import { serverDir, serverKEK, serverPort } from "./server-origin.ts";

const root = resolve(import.meta.dirname, "../../..");

/** The binary `make build` writes; E2E_MY_BIN overrides it. */
export const myBin = process.env["E2E_MY_BIN"] ?? join(root, "bin/my");

/** The SQLite database of the server. */
export const serverDatabase = join(serverDir, "my.db");

/** The environment of `my serve` and of every `bin/my` command against its database. */
export function serverEnv(): NodeJS.ProcessEnv {
  const host = `localhost:${String(serverPort)}`;
  return {
    ...process.env,
    MY_HOST: host,
    MY_LISTEN: `127.0.0.1:${String(serverPort)}`,
    MY_WEB_ORIGIN: `http://${host}`,
    MY_DB: "sqlite",
    MY_DATABASE_URL: serverDatabase,
    MY_BLOB_FS_DIR: join(serverDir, "blobs"),
    MY_KEK: serverKEK,
    MY_MAIL: "none",
  };
}

/**
 * Runs `bin/my` with `args` against the server's database, with `input` on its standard input, and
 * returns its standard output. A failing command throws with its standard error.
 */
export function runMy(args: readonly string[], input = ""): string {
  return execFileSync(myBin, args, { env: serverEnv(), input, encoding: "utf8", stdio: ["pipe", "pipe", "pipe"] });
}

/** Whether the specs reach the server e2e/server.ts starts, whose database `runMy` can change. */
export const ownsServer = process.env["E2E_SERVER_URL"] === undefined;
