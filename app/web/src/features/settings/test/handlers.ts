// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the settings feature (web-client.md §7): the user's own profile and
// preferences, and the lists of blocks, mutes and follow requests. Unblocking and unmuting are the account
// actions of the profile feature's handlers.
import { http, HttpResponse } from "msw";
import { preferencesFixture, relationshipFixture, type Schemas, viewerFixture } from "../../../test/fixtures";
import { apiBase, problem, withViewer } from "../../../test/msw/http";

/** The VAPID public key of an instance that sends Web Push (Instance.push_server_key): the example key of RFC 8291. */
export const pushServerKey = "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8";

export const handlers = [
  // operationIds updateAccountCredentials, updatePreferences; getPreferences is with the compose handlers.
  http.patch(`${apiBase}/accounts/update_credentials`, ({ request }) => withViewer(request, () => HttpResponse.json(viewerFixture))),
  http.patch(`${apiBase}/preferences`, async ({ request }) => {
    const change = (await request.clone().json()) as Schemas["UpdatePreferencesRequest"];
    return withViewer(request, () => HttpResponse.json({ ...preferencesFixture, ...change }));
  }),
  // operationIds getPushSubscription, putPushSubscription, deletePushSubscription: the grant has no subscription by default.
  http.get(`${apiBase}/push/subscription`, ({ request }) => withViewer(request, () => problem(404, "about:blank", "Not Found"))),
  http.put(`${apiBase}/push/subscription`, async ({ request }) => {
    const body = (await request.clone().json()) as Schemas["PushSubscriptionRequest"];
    return withViewer(request, () =>
      HttpResponse.json({ id: "01900000-0000-7000-8000-0000000000f1", endpoint: body.endpoint, alerts: body.alerts, server_key: pushServerKey }),
    );
  }),
  http.delete(`${apiBase}/push/subscription`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
  // operationIds listBlocks, listMutes, listFollowRequests, acceptFollowRequest, rejectFollowRequest: empty lists by default.
  http.get(`${apiBase}/blocks`, ({ request }) => withViewer(request, () => HttpResponse.json({ accounts: [] }))),
  http.get(`${apiBase}/mutes`, ({ request }) => withViewer(request, () => HttpResponse.json({ mutes: [] }))),
  http.get(`${apiBase}/follow_requests`, ({ request }) => withViewer(request, () => HttpResponse.json({ accounts: [] }))),
  http.post(`${apiBase}/follow_requests/:id/authorize`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json({ ...relationshipFixture(String(params["id"])), followed_by: true })),
  ),
  http.post(`${apiBase}/follow_requests/:id/reject`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(relationshipFixture(String(params["id"])))),
  ),
];
