// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the timeline feature (web-client.md §7): one page each; the home timeline
// needs a token.
import { http, HttpResponse } from "msw";
import { statusPage, timelineStatusFixture } from "../../../test/fixtures";
import { apiBase, withViewer } from "../../../test/msw/http";

export const handlers = [
  http.get(`${apiBase}/timelines/home`, ({ request }) => withViewer(request, () => HttpResponse.json(statusPage([timelineStatusFixture])))),
  http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([timelineStatusFixture]))),
  http.get(`${apiBase}/timelines/tag/:hashtag`, () => HttpResponse.json(statusPage([timelineStatusFixture]))),
];
