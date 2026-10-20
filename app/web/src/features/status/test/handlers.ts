// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the status feature (web-client.md §7, §13): any post (GET /statuses/{id}), as
// Bob's post `statusFixture`; the thread around it (GET /statuses/{id}/context): no ancestors and no
// replies; its source (GET /statuses/{id}/source): the plain text of that post; its history
// (GET /statuses/{id}/history): the one revision of a post never edited; and the actions on it (boost,
// favourite, bookmark and their removal; edit; delete). An action answers the post with the viewer's
// relation set as asked; an edit answers it with the new text as its content and `edited_at` set.
import { http, HttpResponse } from "msw";
import { type Schemas, statusFixture } from "../../../test/fixtures";
import { apiBase, withViewer } from "../../../test/msw/http";

/** The viewer's relations to a post, as the Status fields that the actions set. */
type RelationFlag = "boosted" | "favourited" | "bookmarked";

const relation = (path: string, flag: RelationFlag) =>
  [true, false].map((on) =>
    (on ? http.post : http.delete)(`${apiBase}/statuses/:id/${path}`, ({ params, request }) =>
      withViewer(request, () => HttpResponse.json({ ...statusFixture(String(params["id"])), [flag]: on })),
    ),
  );

const context: Schemas["StatusContext"] = { ancestors: [], descendants: [] };

/** The one revision of a post never edited. */
function historyOf(status: Schemas["Status"]): Schemas["StatusHistory"] {
  const { content, spoiler_text, sensitive, created_at, media_attachments } = status;
  return { items: [{ content, spoiler_text, sensitive, created_at, media_attachments }] };
}

export const handlers = [
  http.get(`${apiBase}/statuses/:id`, ({ params }) => HttpResponse.json(statusFixture(String(params["id"])))),
  http.get(`${apiBase}/statuses/:id/context`, () => HttpResponse.json(context)),
  http.get(`${apiBase}/statuses/:id/history`, ({ params }) => HttpResponse.json(historyOf(statusFixture(String(params["id"]))))),
  http.get(`${apiBase}/statuses/:id/source`, ({ params, request }) =>
    withViewer(request, () => HttpResponse.json({ id: String(params["id"]), text: "@alice hello", spoiler_text: "", language: "en" } satisfies Schemas["StatusSource"])),
  ),
  http.put(`${apiBase}/statuses/:id`, async ({ params, request }) => {
    const body = (await request.clone().json()) as Schemas["EditStatusRequest"];
    return withViewer(request, () =>
      HttpResponse.json({
        ...statusFixture(String(params["id"])),
        content: `<p>${body.text}</p>`,
        spoiler_text: body.spoiler_text ?? "",
        edited_at: "2026-10-16T12:00:00Z",
      }),
    );
  }),
  ...relation("boost", "boosted"),
  ...relation("favourite", "favourited"),
  ...relation("bookmark", "bookmarked"),
  http.delete(`${apiBase}/statuses/:id`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
];
