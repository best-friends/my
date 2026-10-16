// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the status feature (web-client.md S1, FR-106 to FR-108, FR-113): one post,
// the thread around it, and the viewer's actions on it. Keys follow openapi-react-query's
// `[method, path, init]`, so the post's key is the one the compose and notifications features use too.
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";

/** A status (post or boost), as the client API returns it. */
export type Status = components["schemas"]["Status"];

/** One attachment of a status. */
export type MediaAttachment = components["schemas"]["MediaAttachment"];

/** The actions that the viewer turns on and off on a post. */
export type Relation = "boost" | "favourite" | "bookmark";

/** A post (GET /statuses/{id}). */
export function statusQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/statuses/{id}", { params: { path: { id } } });
}

/** The thread around a post (GET /statuses/{id}/context). */
export function statusContextQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/statuses/{id}/context", { params: { path: { id } } });
}

/**
 * Turns the viewer's `relation` to post `id` on or off; returns the post after it. The server treats each
 * as a setter (turning on what is on changes nothing), so a retry after a lost answer is safe.
 */
export async function setRelation(api: Api, relation: Relation, id: string, on: boolean): Promise<Status> {
  const init = { params: { path: { id } } };
  const { data, error } = await (() => {
    switch (relation) {
      case "boost":
        return on ? api.fetch.POST("/statuses/{id}/boost", init) : api.fetch.DELETE("/statuses/{id}/boost", init);
      case "favourite":
        return on ? api.fetch.POST("/statuses/{id}/favourite", init) : api.fetch.DELETE("/statuses/{id}/favourite", init);
      case "bookmark":
        return on ? api.fetch.POST("/statuses/{id}/bookmark", init) : api.fetch.DELETE("/statuses/{id}/bookmark", init);
    }
  })();
  // The client's middleware throws an ApiError for every error status (lib/api/client.ts).
  if (data === undefined) throw new ApiError(error);
  return data;
}

/** Deletes the viewer's post `id` (DELETE /statuses/{id}). */
export async function deleteStatus(api: Api, id: string): Promise<void> {
  // The client's middleware throws for every error status; a 204 has no body to read.
  await api.fetch.DELETE("/statuses/{id}", { params: { path: { id } } });
}

/**
 * The web client's address of a post: `/@{acct}/{id}`, where acct is the author's username for a local
 * account and `username@domain` for a remote one. The name only makes the address readable; the ID finds
 * the post (the server answers the same path with the post's link preview, ADR-0028).
 */
export function statusPath(status: Pick<Status, "id" | "account">): string {
  return `/@${status.account.acct}/${status.id}`;
}

/** The address to share for a post: its HTML page, or its ActivityPub ID when it has none. */
export function statusShareUrl(status: Pick<Status, "url" | "uri">): string {
  return status.url ?? status.uri;
}
