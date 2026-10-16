// SPDX-License-Identifier: AGPL-3.0-only
// Default MSW handlers: one successful response per client API operation in api/openapi.yaml, at the
// spec's path under its server URL, and the OAuth endpoints the web client calls. The fixtures take their
// types from the TypeScript types generated from the spec (web-client.md S9); API types are never written
// by hand here. A test that needs another answer overrides a handler with `server.use()`.
import { http, HttpResponse } from "msw";
import type { components } from "../../lib/api/schema.gen";

type Schemas = components["schemas"];

/** The client API base path, `servers[0].url` in api/openapi.yaml. */
export const apiBase = "/api/v1";

/** A valid `Instance` for GET /instance (operationId getInstance). */
export const instanceFixture: Schemas["Instance"] = {
  domain: "my.example",
  name: "my example",
  version: "0.0.0-test",
  source_url: "https://github.com/best-friends/my",
  registrations: { mode: "approval" },
  limits: { max_post_characters: 500 },
};

/** The signed-in user's `Account` for GET /accounts/verify_credentials (operationId verifyAccountCredentials). */
export const viewerFixture: Schemas["Account"] = {
  id: "0192f0c4-7a8e-7000-8000-000000000001",
  username: "alice",
  acct: "alice",
  display_name: "Alice",
  note: "",
  fields: [],
  url: "https://my.example/@alice",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

/** An RFC 9457 problem response, as the client API writes it. */
export function problem(status: number, type: string, title: string, init: { headers?: Record<string, string> } = {}) {
  const body: Schemas["Problem"] = { type, title, status };
  return HttpResponse.json(body, { status, headers: { "Content-Type": "application/problem+json", "Cache-Control": "no-store", ...init.headers } });
}

/** The 401 of a request without a usable access token. */
function unauthenticated(request: Request) {
  return request.headers.get("Authorization") === null
    ? problem(401, "/api/v1/problems/unauthenticated", "Unauthenticated", { headers: { "WWW-Authenticate": "Bearer" } })
    : problem(401, "/api/v1/problems/invalid-token", "Invalid token", { headers: { "WWW-Authenticate": 'Bearer error="invalid_token"' } });
}

/** The OAuth endpoints (internal/interface/oauth). */
export const oauthPaths = {
  authorize: "/oauth/authorize",
  token: "/oauth/token",
  refresh: "/oauth/token/refresh",
  revoke: "/oauth/revoke",
} as const;

/** The access token that the default handlers accept. */
const validAccessToken = "test-access-token";

/** A token response of the OAuth endpoints (RFC 6749 §5.1), without a refresh token as for the web client. */
export function tokenResponse(accessToken: string) {
  return HttpResponse.json({ access_token: accessToken, token_type: "Bearer", expires_in: 3600, scope: "read write follow push" }, { headers: { "Cache-Control": "no-store" } });
}

/** An OAuth error response (RFC 6749 §5.2). */
export function oauthError(error: string, status = 400) {
  return HttpResponse.json({ error }, { status, headers: { "Cache-Control": "no-store" } });
}

export const handlers = [
  http.get(`${apiBase}/instance`, () => HttpResponse.json(instanceFixture)),
  http.get(`${apiBase}/accounts/verify_credentials`, ({ request }) =>
    request.headers.get("Authorization") === `Bearer ${validAccessToken}` ? HttpResponse.json(viewerFixture) : unauthenticated(request),
  ),
  // No refresh token cookie: the browser has no session to resume.
  http.post(oauthPaths.refresh, () => oauthError("invalid_grant")),
  http.post(oauthPaths.token, () => oauthError("invalid_grant")),
  http.post(oauthPaths.revoke, () => new HttpResponse(null, { status: 200 })),
  ...notificationHandlers(),
];

// --- notifications (#132) -------------------------------------------------------------------------

/**
 * A refresh token cookie that resumes a session with the access token the default handlers accept: pass it
 * to `server.use()` before `renderApp` to start signed in.
 */
export function resumableSession() {
  return http.post(oauthPaths.refresh, () => tokenResponse(validAccessToken));
}

/** Another local account, the one that acts in the notification fixtures. */
export const otherAccountFixture: Schemas["Account"] = {
  id: "0192f0c4-7a8e-7000-8000-000000000002",
  username: "bob",
  acct: "bob",
  display_name: "Bob",
  note: "",
  fields: [],
  url: "https://my.example/@bob",
  locked: false,
  bot: false,
  created_at: "2026-10-01T00:00:00Z",
};

/** A post by Bob that mentions Alice, for GET /statuses/{id} (operationId getStatus) with any ID. */
export function statusFixture(id: string): Schemas["Status"] {
  return {
    id,
    uri: `https://my.example/users/bob/statuses/${id}`,
    url: `https://my.example/@bob/${id}`,
    account: otherAccountFixture,
    content: '<p><a href="https://my.example/@alice">@alice</a> hello</p>',
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: "2026-10-16T09:00:00Z",
    media_attachments: [],
    mentions: [{ id: viewerFixture.id, username: "alice", acct: "alice", url: viewerFixture.url }],
    tags: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
  };
}

/** Alice's notifications, newest first, for GET /notifications (operationId listNotifications). */
export const notificationsFixture: Schemas["Notification"][] = [
  {
    id: "0192f0c4-7a8e-7000-8000-0000000000a2",
    type: "mention",
    created_at: "2026-10-16T09:00:00Z",
    account: otherAccountFixture,
    status_id: "0192f0c4-7a8e-7000-8000-0000000000b1",
  },
  { id: "0192f0c4-7a8e-7000-8000-0000000000a1", type: "follow", created_at: "2026-10-15T09:00:00Z", account: otherAccountFixture },
];

/** The default notification handlers: everything up to the follow is read, so the mention is unread. */
function notificationHandlers() {
  const authorized = (request: Request) => request.headers.get("Authorization") === `Bearer ${validAccessToken}`;
  return [
    http.get(`${apiBase}/notifications`, ({ request }) => {
      if (!authorized(request)) return unauthenticated(request);
      const types = new URL(request.url).searchParams.getAll("types");
      const body: Schemas["NotificationPage"] = { items: notificationsFixture.filter((n) => types.length === 0 || types.includes(n.type)) };
      return HttpResponse.json(body);
    }),
    http.get(`${apiBase}/notifications/marker`, ({ request }) => {
      if (!authorized(request)) return unauthenticated(request);
      const body: Schemas["NotificationMarker"] = { last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a1", unread_count: 1 };
      return HttpResponse.json(body);
    }),
    http.put(`${apiBase}/notifications/marker`, async ({ request }) => {
      if (!authorized(request)) return unauthenticated(request);
      const update = (await request.json()) as Schemas["NotificationMarkerUpdate"];
      const body: Schemas["NotificationMarker"] = { last_read_id: update.last_read_id, unread_count: 0 };
      return HttpResponse.json(body);
    }),
    http.post(`${apiBase}/notifications/:id/dismiss`, ({ request }) => (authorized(request) ? new HttpResponse(null, { status: 204 }) : unauthenticated(request))),
    http.get(`${apiBase}/statuses/:id`, ({ params }) => HttpResponse.json(statusFixture(String(params["id"])))),
  ];
}
