// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the status feature (web-client.md S1, FR-106 to FR-108, FR-113): one post,
// the thread around it, and the viewer's actions on it. Keys follow openapi-react-query's
// `[method, path, init]`, so the post's key is the one the compose and notifications features use too.
import type { QueryClient, QueryKey } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { forPost, updateCachedStatuses, withRelation, withRelationFrom } from "./cache";

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

/**
 * Turns the viewer's `relation` to post `id` to `on` in every cached copy of the post before the server answers
 * (web-client.md S8), after cancelling a refetch of the post that would overwrite it with the old state.
 */
export async function cacheRelationPending(api: Api, queryClient: QueryClient, id: string, relation: Relation, on: boolean): Promise<void> {
  await queryClient.cancelQueries({ queryKey: statusQueryOptions(api, id).queryKey });
  updateCachedStatuses(queryClient, forPost(id, (status) => withRelation(status, relation, on)));
}

/** Puts back the viewer's `relation` to post `id` as it was before a press to `on` that the server refused. */
export function restoreRelation(queryClient: QueryClient, id: string, relation: Relation, on: boolean): void {
  updateCachedStatuses(queryClient, forPost(id, (status) => withRelation(status, relation, !on)));
}

/** Whether `queryKey` is a page of the posts of the account `accountId` (GET /accounts/{id}/statuses), under any filter. */
function isAccountStatusesQuery(queryKey: QueryKey, accountId: string): boolean {
  if (queryKey[1] !== "/accounts/{id}/statuses") return false;
  const init = queryKey[2] as { params?: { path?: { id?: string } } } | undefined;
  return init?.params?.path?.id === accountId;
}

/**
 * Puts the server's answer to a press of the viewer `viewerId`'s `relation` to post `id` into the cache
 * (web-client.md S1, ADR-0045). The server's count includes others' actions, so it is taken only from the
 * `last` press: an answer to an earlier press does not undo a later one still on its way. A boost is also an
 * entry of the viewer's home timeline and of their own posts, which it joins or leaves: those lists are marked,
 * and the ones no page shows are fetched again at once (a list on screen keeps its top, Y7).
 */
export function cacheRelationAnswer(queryClient: QueryClient, id: string, relation: Relation, answer: Status, last: boolean, viewerId: string): void {
  if (last) {
    const from = answer.boost ?? answer;
    updateCachedStatuses(queryClient, forPost(id, (status) => withRelationFrom(status, relation, from)));
  }
  if (relation === "boost") {
    void queryClient.invalidateQueries({
      predicate: (query) => query.queryKey[1] === "/timelines/home" || isAccountStatusesQuery(query.queryKey, viewerId),
      refetchType: "inactive",
    });
  }
}

/** Deletes the viewer's post `id` (DELETE /statuses/{id}). */
export async function deleteStatus(api: Api, id: string): Promise<void> {
  // The client's middleware throws for every error status; a 204 has no body to read.
  await api.fetch.DELETE("/statuses/{id}", { params: { path: { id } } });
}

/**
 * Takes the deleted post `post` out of the cache (web-client.md S1, ADR-0045): its own query and thread go,
 * every list drops it and the boosts of it, and its parent has one reply less.
 */
export function cacheDeletedStatus(api: Api, queryClient: QueryClient, post: Status): void {
  queryClient.removeQueries({ queryKey: statusQueryOptions(api, post.id).queryKey, exact: true });
  queryClient.removeQueries({ queryKey: statusContextQueryOptions(api, post.id).queryKey, exact: true });
  updateCachedStatuses(queryClient, forPost(post.id, () => null));
  const parentId = post.in_reply_to_id;
  if (parentId !== undefined) {
    updateCachedStatuses(queryClient, forPost(parentId, (parent) => ({ ...parent, replies_count: Math.max(0, parent.replies_count - 1) })));
  }
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
