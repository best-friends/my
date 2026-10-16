// SPDX-License-Identifier: AGPL-3.0-only
// Where the E2E reach the real server (`my serve`, started by e2e/server.ts), shared by the Playwright
// config and the specs of the server-rendered pages.

/** The port of `vite preview`, which serves the built client unless E2E_BASE_URL points elsewhere. */
export const previewPort = 4173;

/** The origin of `vite preview`. */
export const previewOrigin = `http://127.0.0.1:${String(previewPort)}`;

/** The port e2e/server.ts listens on; E2E_SERVER_PORT moves it, for runs side by side on one machine. */
export const serverPort = Number(process.env["E2E_SERVER_PORT"] ?? "4180");

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
