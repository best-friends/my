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

/** The signed-in user's `CredentialAccount` for GET /accounts/verify_credentials (operationId verifyAccountCredentials). */
export const viewerFixture: Schemas["CredentialAccount"] = {
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
  discoverable: true,
  indexable: true,
  source: { note: "", fields: [] },
};

/** An `Account` of another user, for the lists of accounts; `n` makes its ID unique. */
export function accountFixture(n: number, name: string): Schemas["Account"] {
  const username = name.toLowerCase();
  return {
    id: `0192f0c4-7a8e-7000-8000-${String(n).padStart(12, "0")}`,
    username,
    acct: `${username}@remote.example`,
    display_name: name,
    note: "",
    fields: [],
    url: `https://remote.example/@${username}`,
    locked: false,
    bot: false,
    created_at: "2026-10-01T00:00:00Z",
  };
}

// --- statuses, media and preferences (the compose feature, #129) ---

/** The signed-in user's `Preferences` for GET /preferences (operationId getPreferences). */
export const preferencesFixture: Schemas["Preferences"] = {
  locale: "en",
  time_zone: "UTC",
  default_visibility: "public",
  default_post_language: "en",
  require_media_description: false,
  timeline_languages: [],
};

/** The `Relationship` with account `id` when the viewer has none, or with `overrides`. */
export function relationshipFixture(id: string, overrides: Partial<Schemas["Relationship"]> = {}): Schemas["Relationship"] {
  return {
    id,
    following: false,
    requested: false,
    followed_by: false,
    requested_by: false,
    show_boosts: true,
    notify: false,
    blocking: false,
    muting: false,
    muting_notifications: false,
    ...overrides,
  };
}

/** A `Status` by the viewer, with `overrides`. */
export function viewerStatusFixture(overrides: Partial<Schemas["Status"]> = {}): Schemas["Status"] {
  return {
    id: "0192f0c4-7a8e-7000-8000-0000000000a1",
    uri: "https://my.example/users/alice/statuses/0192f0c4-7a8e-7000-8000-0000000000a1",
    account: viewerFixture,
    content: "<p>Hello</p>",
    spoiler_text: "",
    sensitive: false,
    visibility: "public",
    language: "en",
    created_at: "2026-10-16T00:00:00Z",
    media_attachments: [],
    mentions: [],
    tags: [],
    replies_count: 0,
    boosts_count: 0,
    favourites_count: 0,
    ...overrides,
  };
}

/**
 * A `MediaAttachment` with `id` in `state`; a ready one has its renditions and blurhash, as GET /media/{id}
 * answers once the server has processed the file.
 */
export function mediaFixture(id: string, state: Schemas["MediaAttachment"]["state"] = "ready", description?: string): Schemas["MediaAttachment"] {
  const variant = (size: string) => ({ url: `https://my.example/media/${id}/${size}.jpg`, content_type: "image/jpeg", width: 640, height: 360, size: 1000 });
  return {
    id,
    type: "image",
    state,
    meta: { width: 640, height: 360, focus: { x: 0, y: 0 } },
    ...(state === "ready" ? { blurhash: "LEHV6nWB2yk8pyo0adR*.7kCMdnj", variants: { full: variant("full"), thumbnail: variant("thumbnail") } } : {}),
    ...(description === undefined ? {} : { description }),
  };
}

/** The ID the default POST /media handler gives an upload. */
export const uploadedMediaId = "0192f0c4-7a8e-7000-8000-0000000000b1";

const statusHandlers = [
  http.get(`${apiBase}/preferences`, ({ request }) =>
    request.headers.get("Authorization") === `Bearer ${validAccessToken}` ? HttpResponse.json(preferencesFixture) : unauthenticated(request),
  ),
  // Posts the status as the viewer, as the server would render it.
  http.post(`${apiBase}/statuses`, async ({ request }) => {
    if (request.headers.get("Authorization") !== `Bearer ${validAccessToken}`) return unauthenticated(request);
    const body = (await request.json()) as Schemas["CreateStatusRequest"];
    return HttpResponse.json(
      viewerStatusFixture({
        content: `<p>${body.text}</p>`,
        spoiler_text: body.spoiler_text ?? "",
        sensitive: body.sensitive ?? false,
        visibility: body.visibility,
        language: body.language ?? preferencesFixture.default_post_language,
        ...(body.in_reply_to_id === undefined ? {} : { in_reply_to_id: body.in_reply_to_id }),
        media_attachments: (body.media ?? []).map((m) => mediaFixture(m.id, "ready", m.description)),
      }),
    );
  }),
  // Accepts the upload; it is processed by the time the client asks again.
  http.post(`${apiBase}/media`, () => HttpResponse.json(mediaFixture(uploadedMediaId, "processing"), { status: 202 })),
  http.get(`${apiBase}/media/:id`, ({ params }) => HttpResponse.json(mediaFixture(String(params["id"])))),
  http.patch(`${apiBase}/media/:id`, async ({ params, request }) =>
    HttpResponse.json(mediaFixture(String(params["id"]), "ready", ((await request.json()) as Schemas["MediaUpdate"]).description)),
  ),
];

// --- end of statuses, media and preferences ---

/** A local post of another account, for the timelines. */
export const timelineStatusFixture: Schemas["Status"] = {
  id: "0192f0c4-7a8e-7000-8000-0000000000a1",
  uri: "https://my.example/ap/statuses/0192f0c4-7a8e-7000-8000-0000000000a1",
  url: "https://my.example/@bob/0192f0c4-7a8e-7000-8000-0000000000a1",
  account: { ...viewerFixture, id: "0192f0c4-7a8e-7000-8000-000000000002", username: "bob", acct: "bob", display_name: "Bob", url: "https://my.example/@bob" },
  content: "<p>Hello from the local timeline!</p>",
  spoiler_text: "",
  sensitive: false,
  visibility: "public",
  language: "en",
  created_at: "2026-10-16T09:00:00Z",
  media_attachments: [],
  mentions: [],
  tags: [],
  replies_count: 0,
  boosts_count: 0,
  favourites_count: 0,
};

/** A page of a timeline with the given statuses and, when there are older ones, the cursor to them. */
export function statusPage(items: Schemas["Status"][], nextCursor?: string): Schemas["StatusPage"] {
  return nextCursor === undefined ? { items } : { items, next_cursor: nextCursor };
}

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

/** Answers with `respond()` when the request carries the access token the default handlers accept. */
function withViewer(request: Request, respond: () => Response): Response {
  return request.headers.get("Authorization") === `Bearer ${validAccessToken}` ? respond() : unauthenticated(request);
}

/** Makes the session resume: the refresh endpoint answers with the access token the default handlers accept. */
export const signedInHandler = http.post(oauthPaths.refresh, () => tokenResponse(validAccessToken));

export const handlers = [
  http.get(`${apiBase}/instance`, () => HttpResponse.json(instanceFixture)),
  http.get(`${apiBase}/accounts/verify_credentials`, ({ request }) =>
    request.headers.get("Authorization") === `Bearer ${validAccessToken}` ? HttpResponse.json(viewerFixture) : unauthenticated(request),
  ),
  // The user's own profile and preferences (operationIds updateAccountCredentials, updatePreferences; getPreferences is with the compose handlers).
  http.patch(`${apiBase}/accounts/update_credentials`, ({ request }) => withViewer(request, () => HttpResponse.json(viewerFixture))),
  http.patch(`${apiBase}/preferences`, async ({ request }) => {
    const change = (await request.clone().json()) as Schemas["UpdatePreferencesRequest"];
    return withViewer(request, () => HttpResponse.json({ ...preferencesFixture, ...change }));
  }),
  // Blocks, mutes and follow requests (operationIds listBlocks, listMutes, listFollowRequests, unblockAccount,
  // unmuteAccount, acceptFollowRequest, rejectFollowRequest): empty lists by default.
  http.get(`${apiBase}/blocks`, ({ request }) => withViewer(request, () => HttpResponse.json({ accounts: [] }))),
  http.get(`${apiBase}/mutes`, ({ request }) => withViewer(request, () => HttpResponse.json({ mutes: [] }))),
  http.get(`${apiBase}/follow_requests`, ({ request }) => withViewer(request, () => HttpResponse.json({ accounts: [] }))),
  http.post(`${apiBase}/accounts/:id/unblock`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(relationshipFixture(String(params["id"])))),
  ),
  http.post(`${apiBase}/accounts/:id/unmute`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(relationshipFixture(String(params["id"])))),
  ),
  http.post(`${apiBase}/follow_requests/:id/authorize`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json({ ...relationshipFixture(String(params["id"])), followed_by: true })),
  ),
  http.post(`${apiBase}/follow_requests/:id/reject`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(relationshipFixture(String(params["id"])))),
  ),
  // Timelines (#128): one page each; the home timeline needs a token.
  http.get(`${apiBase}/timelines/home`, ({ request }) =>
    request.headers.get("Authorization") === `Bearer ${validAccessToken}` ? HttpResponse.json(statusPage([timelineStatusFixture])) : unauthenticated(request),
  ),
  http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([timelineStatusFixture]))),
  http.get(`${apiBase}/timelines/tag/:hashtag`, () => HttpResponse.json(statusPage([timelineStatusFixture]))),
  // No refresh token cookie: the browser has no session to resume.
  http.post(oauthPaths.refresh, () => oauthError("invalid_grant")),
  http.post(oauthPaths.token, () => oauthError("invalid_grant")),
  http.post(oauthPaths.revoke, () => new HttpResponse(null, { status: 200 })),
  ...statusHandlers,
  ...notificationHandlers(),
  ...accountHandlers(),
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

/** A post by Bob that mentions Alice. */
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
    status: statusFixture("0192f0c4-7a8e-7000-8000-0000000000b1"),
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
  ];
}

// --- accounts and profiles (#131) ----------------------------------------------------------------

/** The relationship that each change of the default handlers returns. */
const relationshipAfter: Readonly<Record<string, Partial<Schemas["Relationship"]>>> = {
  follow: { following: true },
  unfollow: {},
  block: { blocking: true },
  unblock: {},
  mute: { muting: true, muting_notifications: true },
  unmute: {},
};

/**
 * The default account handlers: GET /accounts/lookup finds Alice (the viewer) and Bob with their counts and
 * nobody else; the viewer has no relationship with anyone; each change returns the relationship after it;
 * an account's posts are the timeline fixture, and each follow list holds the other account.
 */
function accountHandlers() {
  const authorized = (request: Request) => request.headers.get("Authorization") === `Bearer ${validAccessToken}`;
  // A function: the `handlers` array at the top calls accountHandlers() before the fixtures of the later
  // sections (otherAccountFixture) are initialized.
  const profiles = (): Readonly<Record<string, Schemas["Account"]>> => ({
    alice: { ...viewerFixture, followers_count: 1, following_count: 1 },
    bob: { ...otherAccountFixture, followers_count: 1, following_count: 1 },
  });
  const followList = (id: string): Schemas["AccountPage"] => ({ accounts: [id === viewerFixture.id ? otherAccountFixture : viewerFixture] });
  return [
    http.get(`${apiBase}/accounts/lookup`, ({ request }) => {
      const acct = new URL(request.url).searchParams.get("acct") ?? "";
      const account = profiles()[acct.replace(/^@/, "")];
      return account === undefined ? problem(404, "about:blank", "Not Found") : HttpResponse.json(account);
    }),
    http.get(`${apiBase}/accounts/relationships`, ({ request }) => {
      if (!authorized(request)) return unauthenticated(request);
      return HttpResponse.json(new URL(request.url).searchParams.getAll("id").map((id) => relationshipFixture(id)));
    }),
    http.post(`${apiBase}/accounts/:id/:action`, ({ request, params }) => {
      if (!authorized(request)) return unauthenticated(request);
      const after = relationshipAfter[String(params["action"])];
      return after === undefined ? problem(404, "about:blank", "Not Found") : HttpResponse.json(relationshipFixture(String(params["id"]), after));
    }),
    http.get(`${apiBase}/accounts/:id/statuses`, () => HttpResponse.json(statusPage([timelineStatusFixture]))),
    http.get(`${apiBase}/accounts/:id/followers`, ({ params }) => HttpResponse.json(followList(String(params["id"])))),
    http.get(`${apiBase}/accounts/:id/following`, ({ params }) => HttpResponse.json(followList(String(params["id"])))),
  ];
}
