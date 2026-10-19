// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the moderation feature (web-client.md §7): the actions on the user's own account
// (none by default) and appealing one, with the fixture of an action that only its tests use.
import { http, HttpResponse } from "msw";
import type { Schemas } from "../../../test/fixtures";
import { apiBase, withViewer } from "../../../test/msw/http";

/** A moderation action on the viewer's account: a suspension in effect, taken on 16 October 2026, appealable. */
export function ownActionFixture(overrides: Partial<Schemas["ModerationAction"]> = {}): Schemas["ModerationAction"] {
  return {
    id: "0192f0c4-7a8e-7000-8000-0000000007a1",
    kind: "suspend",
    kind_label: "Suspended",
    reason_category: "violation",
    reason_category_label: "Rule violation",
    public_reason: "Repeated spam in replies.",
    status_ids: [],
    state: "active",
    issued_at: "2026-10-16T09:30:00Z",
    expires_at: "2026-11-15T09:30:00Z",
    appealable: true,
    appeal_deadline: "2027-04-14T09:30:00Z",
    ...overrides,
  };
}

export const handlers = [
  // operationIds listOwnModerationActions, appealModerationAction.
  http.get(`${apiBase}/moderation/actions`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/moderation/actions/:id/appeal`, async ({ request, params }) => {
    const { text } = (await request.clone().json()) as Schemas["AppealModerationActionRequest"];
    return withViewer(request, () =>
      HttpResponse.json(
        ownActionFixture({
          id: String(params["id"]),
          appealable: false,
          appeal: { id: "0192f0c4-7a8e-7000-8000-0000000007b1", text, state: "pending", submitted_at: "2026-10-17T00:00:00Z" },
        }),
      ),
    );
  }),
];
