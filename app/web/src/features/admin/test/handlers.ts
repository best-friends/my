// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the server administration feature (web-client.md §7): the dead jobs, the
// server's texts and the custom emoji, none by default. The default viewer has no permissions, so the screens' tests start with
// `server.use(operatorHandler(...))` (test/msw/http.ts).
import { http, HttpResponse } from "msw";
import type { Schemas } from "../../../test/fixtures";
import { apiBase, withViewer } from "../../../test/msw/http";

/** An instance text as the server answers it, saved at 2026-10-17T00:00:00Z. */
export function instanceTextFixture(key: Schemas["InstanceTextKey"], locale: string, body: string): Schemas["AdminInstanceText"] {
  return { key, locale, body, revised_at: "2026-10-17T00:00:00Z" };
}

/** A local custom emoji as the admin API answers it, registered at 2026-10-20T00:00:00Z, with `overrides`. */
export function adminCustomEmojiFixture(id: string, shortcode: string, overrides: Partial<Schemas["AdminCustomEmoji"]> = {}): Schemas["AdminCustomEmoji"] {
  return {
    id,
    shortcode,
    local: true,
    uri: `https://my.example/ap/emojis/${id}`,
    url: `https://my.example/media/${id}/r/medium.webp`,
    static_url: `https://my.example/media/${id}/r/medium.webp`,
    enabled: true,
    visible_in_picker: true,
    aliases: [],
    created_at: "2026-10-20T00:00:00Z",
    updated_at: "2026-10-20T00:00:00Z",
    ...overrides,
  };
}

/** The ID the default POST /admin/custom_emojis handler gives a new emoji. */
const createdCustomEmojiId = "0199f0e0-0000-7000-8000-00000000e001";

export const handlers = [
  // operationIds listDeadJobs, retryDeadJob.
  http.get(`${apiBase}/admin/dead_jobs`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/dead_jobs/:id/retry`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
  // operationIds listInstanceTexts, setInstanceText: no texts; a new text is recorded as sent, at 2026-10-17T00:00:00Z.
  http.get(`${apiBase}/admin/instance_texts`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.put(`${apiBase}/admin/instance_texts/:key/:locale`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["SetInstanceTextRequest"];
    return withViewer(request, () => HttpResponse.json(instanceTextFixture(String(params["key"]) as Schemas["InstanceTextKey"], String(params["locale"]), body.body)));
  }),
  // operationIds getAdminTheme, setAdminTheme: no theme; a saved theme is answered as sent, at a new revision.
  http.get(`${apiBase}/admin/theme`, ({ request }) => withViewer(request, () => HttpResponse.json({ tokens: [], custom_css: "" } satisfies Schemas["AdminTheme"]))),
  http.put(`${apiBase}/admin/theme`, async ({ request }) => {
    const body = (await request.clone().json()) as Schemas["SetThemeRequest"];
    return withViewer(request, () => HttpResponse.json({ ...body, revision: "0199f0e0-0000-7000-8000-00000000a001", revised_at: "2026-10-20T00:00:00Z" } satisfies Schemas["AdminTheme"]));
  }),
  // operationIds listAdminCustomEmojis, createCustomEmoji, getAdminCustomEmoji, updateCustomEmoji, deleteCustomEmoji,
  // replaceCustomEmojiImage: no emoji; a new or changed emoji is answered as sent.
  http.get(`${apiBase}/admin/custom_emojis`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] } satisfies Schemas["AdminCustomEmojiPage"]))),
  http.post(`${apiBase}/admin/custom_emojis`, async ({ request }) => {
    const { media_id: _, ...body } = (await request.clone().json()) as Schemas["CreateCustomEmojiRequest"];
    return withViewer(request, () => HttpResponse.json(adminCustomEmojiFixture(createdCustomEmojiId, body.shortcode, body)));
  }),
  http.get(`${apiBase}/admin/custom_emojis/:id`, ({ request, params }) => withViewer(request, () => HttpResponse.json(adminCustomEmojiFixture(String(params["id"]), "blobcat")))),
  http.patch(`${apiBase}/admin/custom_emojis/:id`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["UpdateCustomEmojiRequest"];
    return withViewer(request, () => HttpResponse.json(adminCustomEmojiFixture(String(params["id"]), "blobcat", body)));
  }),
  http.delete(`${apiBase}/admin/custom_emojis/:id`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
  http.put(`${apiBase}/admin/custom_emojis/:id/image`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(adminCustomEmojiFixture(String(params["id"]), "blobcat", { updated_at: "2026-10-21T00:00:00Z" }))),
  ),
];
