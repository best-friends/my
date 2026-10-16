// SPDX-License-Identifier: AGPL-3.0-only
// Where the E2E reach the real server (`my serve`, started by e2e/server.ts), shared by the Playwright
// config and the specs of the server-rendered pages.
import { execFileSync } from "node:child_process";

/**
 * Picks `count` distinct free TCP ports on 127.0.0.1. Node.js can only listen asynchronously, so a child
 * Node.js process binds port 0 on every socket at once and prints the ports the kernel chose.
 */
function freePorts(count: number): number[] {
  const script = `
    const net = require("node:net");
    const servers = Array.from({ length: ${String(count)} }, () => net.createServer());
    Promise.all(servers.map((s) => new Promise((ok) => s.listen(0, "127.0.0.1", ok)))).then(() => {
      process.stdout.write(servers.map((s) => s.address().port).join(" "));
      servers.forEach((s) => s.close());
    });`;
  return execFileSync(process.execPath, ["-e", script], { encoding: "utf8" }).trim().split(" ").map(Number);
}

/**
 * The ports of `vite preview` and of e2e/server.ts: E2E_PREVIEW_PORT and E2E_SERVER_PORT when set,
 * otherwise free ports picked when the Playwright config first loads. The picked ports are written back to
 * the environment, which the webServer commands and the worker processes inherit, so every process of one
 * run uses the same ports and runs in other worktrees on the same machine use others.
 */
function ports(): { preview: number; server: number } {
  const missing = ["E2E_PREVIEW_PORT", "E2E_SERVER_PORT"].filter((name) => process.env[name] === undefined);
  const picked = freePorts(missing.length);
  for (const [i, name] of missing.entries()) {
    process.env[name] = String(picked[i]);
  }
  return { preview: Number(process.env["E2E_PREVIEW_PORT"]), server: Number(process.env["E2E_SERVER_PORT"]) };
}

const { preview, server } = ports();

/** The port of `vite preview`, which serves the built client unless E2E_BASE_URL points elsewhere. */
export const previewPort = preview;

/** The origin of `vite preview`. */
export const previewOrigin = `http://127.0.0.1:${String(previewPort)}`;

/** The port e2e/server.ts listens on. */
export const serverPort = server;

/**
 * The origin of `my serve`: E2E_SERVER_URL when set, otherwise the server e2e/server.ts starts. The host
 * is localhost because the server's host must be a domain name, and browsers accept the pages' Secure
 * (__Host-) cookies over plain HTTP only on localhost.
 */
export const serverURL = process.env["E2E_SERVER_URL"] ?? `http://localhost:${serverPort}`;

/** The seeded admin that the specs sign in as. */
export const admin = { email: "alice@example.com", password: "e2e-password-alice" } as const;

/** The seeded third-party OAuth client, which gets the consent page. */
export const thirdPartyClient = {
  clientID: "e2e-third-party",
  name: "E2E Example App",
  redirectURI: `${serverURL}/e2e/callback`,
} as const;
