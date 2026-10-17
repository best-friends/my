// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the settings feature (web-client.md §7): the user's own profile and
// preferences, and the lists of blocks, mutes and follow requests. Unblocking and unmuting are the account
// actions of the profile feature's handlers.
import { http, HttpResponse } from "msw";
import { preferencesFixture, relationshipFixture, type Schemas, viewerFixture } from "../../../test/fixtures";
import { apiBase, withViewer } from "../../../test/msw/http";

export const handlers = [
  // operationIds updateAccountCredentials, updatePreferences; getPreferences is with the compose handlers.
  http.patch(`${apiBase}/accounts/update_credentials`, ({ request }) => withViewer(request, () => HttpResponse.json(viewerFixture))),
  http.patch(`${apiBase}/preferences`, async ({ request }) => {
    const change = (await request.clone().json()) as Schemas["UpdatePreferencesRequest"];
    return withViewer(request, () => HttpResponse.json({ ...preferencesFixture, ...change }));
  }),
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
