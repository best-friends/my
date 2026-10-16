// SPDX-License-Identifier: AGPL-3.0-only
// Starts the real server for the E2E of the server-rendered pages (ADR-0030): a fresh SQLite database
// in a temporary directory with schema/sqlite.sql applied, the admin and the third-party OAuth client of
// e2e/server-origin.ts, then `my serve` until Playwright stops it. Playwright runs it as a webServer
// with Node.js (`node e2e/server.ts`), from the binary `make build` writes (override with E2E_MY_BIN).
import { execFileSync, spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { mkdirSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { admin, serverPort, thirdPartyClient } from "./server-origin.ts";

const root = resolve(import.meta.dirname, "../../..");
const bin = process.env["E2E_MY_BIN"] ?? join(root, "bin/my");
const dir = mkdtempSync(join(tmpdir(), "my-e2e-"));
const database = join(dir, "my.db");
const host = `localhost:${serverPort}`;

const env: NodeJS.ProcessEnv = {
  ...process.env,
  MY_HOST: host,
  MY_LISTEN: `127.0.0.1:${serverPort}`,
  MY_WEB_ORIGIN: `http://${host}`,
  MY_DB: "sqlite",
  MY_DATABASE_URL: database,
  MY_BLOB_FS_DIR: join(dir, "blobs"),
  MY_KEK: randomBytes(32).toString("base64"),
  MY_MAIL: "none",
};
mkdirSync(env["MY_BLOB_FS_DIR"] ?? "", { recursive: true });

// The schema file is plain SQL; Atlas applies the same file in production (ADR-0023).
const db = new DatabaseSync(database);
db.exec(readFileSync(join(root, "schema/sqlite.sql"), "utf8"));
db.close();

execFileSync(bin, ["admin", "create", "-username", "alice", "-email", admin.email], {
  env,
  input: `${admin.password}\n`,
  stdio: ["pipe", "inherit", "inherit"],
});

// Third-party clients have no registration flow yet, so the client is written to its tables directly
// (data-model.md §4.1): a public client limited to the read scope with one redirect URI.
const seed = new DatabaseSync(database);
const id = randomUUID();
seed
  .prepare(
    "INSERT INTO oauth_clients (id, client_id, client_secret_hash, name, website, scope, is_first_party, owner_user_id, created_at) VALUES (?, ?, NULL, ?, NULL, 'read', 0, NULL, ?)",
  )
  .run(id, thirdPartyClient.clientID, thirdPartyClient.name, Date.now());
seed
  .prepare("INSERT INTO oauth_client_redirect_uris (client_id, redirect_uri) VALUES (?, ?)")
  .run(id, thirdPartyClient.redirectURI);
seed.close();

const server = spawn(bin, ["serve"], { env, stdio: "inherit" });
const stop = () => {
  server.kill("SIGTERM");
};
process.on("SIGINT", stop);
process.on("SIGTERM", stop);
server.on("exit", (code, signal) => {
  rmSync(dir, { recursive: true, force: true });
  process.exit(signal === null ? (code ?? 1) : 0);
});
