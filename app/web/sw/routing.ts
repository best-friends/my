// SPDX-License-Identifier: AGPL-3.0-only
// Which requests the Service Worker answers, and how (ADR-0028). A pure function, so that the rules are
// unit-tested apart from the worker (routing.test.ts).

/**
 * Paths whose navigations the worker leaves to the browser: the server's other endpoints, including the
 * server-rendered sign-in and consent pages (/auth, /oauth), which need the network and must never turn
 * into the SPA's shell. They are the server's reserved prefixes (internal/interface/web/handler.go) and
 * the Web App Manifest.
 */
const passThroughPaths = [
  "/api",
  "/oauth",
  "/auth",
  "/.well-known",
  "/nodeinfo",
  "/ap",
  "/media",
  "/uploads",
  "/manifest.webmanifest",
] as const;

/**
 * - `navigation`: network only with a timeout, falling back to the precached shell;
 * - `media`: cache first, bounded;
 * - `network`: not answered by the worker at all (the client API, every non-GET, other origins).
 */
export type Handling = "navigation" | "media" | "network";

/** The parts of a fetch event's Request that decide its handling. */
export type RequestFacts = {
  url: string;
  method: string;
  mode: string;
  destination: string;
  /** Whether the request carries an Authorization header. */
  authorized: boolean;
};

/** Decides how the worker answers a request; `origin` is the worker's own origin. */
export function classify(request: RequestFacts, origin: string): Handling {
  if (request.method !== "GET") return "network";
  const url = new URL(request.url);
  if (url.origin !== origin) return "network";
  if (request.mode === "navigate") return underAny(url.pathname, passThroughPaths) ? "network" : "navigation";
  // Public media (overview.md: blob keys under media/ are public and served at /media/). Only images: a
  // video's range requests get partial responses, which a cache cannot replay. A request with credentials
  // in a header is never cached.
  if (underAny(url.pathname, ["/media"]) && request.destination === "image" && !request.authorized) return "media";
  return "network";
}

function underAny(path: string, prefixes: readonly string[]): boolean {
  return prefixes.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}
