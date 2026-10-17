// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the status feature (web-client.md §7, §13): any post (GET /statuses/{id}), as
// Bob's post `statusFixture`; the thread around it (GET /statuses/{id}/context): no ancestors and no
// replies; and the actions on it (boost, favourite, bookmark and their removal; delete). An action answers
// the post with the viewer's relation set as asked.
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

export const handlers = [
  http.get(`${apiBase}/statuses/:id`, ({ params }) => HttpResponse.json(statusFixture(String(params["id"])))),
  http.get(`${apiBase}/statuses/:id/context`, () => HttpResponse.json(context)),
  ...relation("boost", "boosted"),
  ...relation("favourite", "favourited"),
  ...relation("bookmark", "bookmarked"),
  http.delete(`${apiBase}/statuses/:id`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
];
