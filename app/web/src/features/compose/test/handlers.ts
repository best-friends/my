// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the compose feature (web-client.md §7, §12): the preferences, posting and
// the media uploads.
import { http, HttpResponse } from "msw";
import { mediaFixture, preferencesFixture, type Schemas, viewerStatusFixture } from "../../../test/fixtures";
import { apiBase, authorized, unauthenticated, withViewer } from "../../../test/msw/http";

/** The ID the default POST /media handler gives an upload. */
export const uploadedMediaId = "0192f0c4-7a8e-7000-8000-0000000000b1";

export const handlers = [
  http.get(`${apiBase}/preferences`, ({ request }) => withViewer(request, () => HttpResponse.json(preferencesFixture))),
  // Posts the status as the viewer, as the server would render it.
  http.post(`${apiBase}/statuses`, async ({ request }) => {
    if (!authorized(request)) return unauthenticated(request);
    const body = (await request.json()) as Schemas["CreateStatusRequest"];
    return HttpResponse.json(
      viewerStatusFixture({
        content: `<p>${body.text}</p>`,
        spoiler_text: body.spoiler_text ?? "",
        sensitive: body.sensitive ?? false,
        visibility: body.visibility,
        language: body.language ?? preferencesFixture.default_post_language,
        ...(body.in_reply_to_id === undefined ? {} : { in_reply_to_id: body.in_reply_to_id }),
        ...(body.quoted_status_id === undefined ? {} : { quote: { state: "pending" } }),
        quote_policy: body.quote_policy ?? preferencesFixture.default_quote_policy,
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
