// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the status feature (web-client.md S1, FR-106 to FR-109, FR-113): one post,
// the thread around it, its edit history, and the viewer's actions on it (editing their own post among them). Keys follow openapi-react-query's
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

/** The new content of an edited post (PUT /statuses/{id}). */
export type EditStatusRequest = components["schemas"]["EditStatusRequest"];

/** What the author typed, for editing a post (GET /statuses/{id}/source). */
export type StatusSource = components["schemas"]["StatusSource"];

/** One revision of a post (GET /statuses/{id}/history). */
export type StatusEdit = components["schemas"]["StatusEdit"];

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

/** Every revision of a post, oldest first; the last one is the current content (GET /statuses/{id}/history). */
export function statusHistoryQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/statuses/{id}/history", { params: { path: { id } } });
}

/**
 * What the viewer typed for their post `id`, for editing it (GET /statuses/{id}/source). It is read again
 * every time the edit dialog opens (`staleTime: 0`), so that an edit starts from the latest text. A failure
 * shows at once with a button to try again, instead of after seconds of silent retries.
 */
export function statusSourceQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/statuses/{id}/source", { params: { path: { id } } }, { staleTime: 0, gcTime: 0, retry: false });
}

/** The instance, for the most characters a post may have (`limits.max_post_characters`). */
export function instanceQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/instance");
}

/** Replaces the content of the viewer's post `id` (PUT /statuses/{id}); returns the edited post. */
export async function editStatus(api: Api, id: string, body: EditStatusRequest): Promise<Status> {
  const { data, error } = await api.fetch.PUT("/statuses/{id}", { params: { path: { id } }, body });
  // The client's middleware throws an ApiError for every error status (lib/api/client.ts).
  if (data === undefined) throw new ApiError(error);
  return data;
}

/**
 * Puts the server's answer to an edit into the cache (web-client.md S1, ADR-0045): every cached copy of the
 * post, in a list or inside a boost, becomes the edited post, where it is, so that no list on screen moves
 * (Y7). The history has one more revision: it is marked stale, and fetched again if it is on screen.
 */
export async function cacheEditedStatus(api: Api, queryClient: QueryClient, edited: Status): Promise<void> {
  updateCachedStatuses(queryClient, forPost(edited.id, () => edited));
  await queryClient.invalidateQueries({ queryKey: statusHistoryQueryOptions(api, edited.id).queryKey, exact: true });
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
 * and only marked stale: a list on screen keeps its top (Y7), and a list opened next is fetched again before it shows (S7).
 * A bookmark likewise joins or leaves the viewer's list of bookmarks, which is only marked: on the bookmarks page a
 * removed post stays, shown unbookmarked, so that the press can be undone and focus stays on its button; the post is
 * gone when the list is opened again.
 */
export function cacheRelationAnswer(queryClient: QueryClient, id: string, relation: Relation, answer: Status, last: boolean, viewerId: string): void {
  if (last) {
    const from = answer.boost ?? answer;
    updateCachedStatuses(queryClient, forPost(id, (status) => withRelationFrom(status, relation, from)));
  }
  if (relation === "boost") {
    void queryClient.invalidateQueries({
      predicate: (query) => query.queryKey[1] === "/timelines/home" || isAccountStatusesQuery(query.queryKey, viewerId),
      refetchType: "none",
    });
  }
  if (relation === "bookmark") {
    void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[1] === "/bookmarks", refetchType: "none" });
  }
}

/**
 * The posts that account `accountId` pinned to its profile, newest pin first, in a single page
 * (GET /accounts/{id}/statuses?pinned=true, FR-403). For a remote account, those this server knows of.
 */
export function pinnedStatusesQueryOptions(api: Api, accountId: string) {
  return api.query.queryOptions("get", "/accounts/{id}/statuses", { params: { path: { id: accountId }, query: { pinned: true } } });
}

/**
 * Pins the viewer's post `id` to their profile, or unpins it; returns the post after it. The server treats
 * both as setters, so a retry after a lost answer is safe; a pin beyond the limit is a 422 whose detail says so.
 */
export async function setPin(api: Api, id: string, on: boolean): Promise<Status> {
  const init = { params: { path: { id } } };
  const { data, error } = await (on ? api.fetch.POST("/statuses/{id}/pin", init) : api.fetch.DELETE("/statuses/{id}/pin", init));
  // The client's middleware throws an ApiError for every error status (lib/api/client.ts).
  if (data === undefined) throw new ApiError(error);
  return data;
}

/**
 * Puts the server's answer to a pin or unpin of the viewer `viewerId`'s post `id` into the cache (web-client.md
 * S1): every copy of the post takes its `pinned`, and the viewer's pinned posts, a list that changes, are read
 * again. Resolves once the list on screen has been read again, so that the success shows with it.
 */
export async function cachePinAnswer(api: Api, queryClient: QueryClient, id: string, answer: Status, viewerId: string): Promise<void> {
  const pinned = answer.pinned === true;
  updateCachedStatuses(queryClient, forPost(id, (status) => (status.pinned === pinned ? status : { ...status, pinned })));
  await queryClient.invalidateQueries({ queryKey: pinnedStatusesQueryOptions(api, viewerId).queryKey });
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

/** The web client's address of a post's edit history: `/@{acct}/{id}/history`. */
export function statusHistoryPath(status: Pick<Status, "id" | "account">): string {
  return `${statusPath(status)}/history`;
}

/** The address to share for a post: its HTML page, or its ActivityPub ID when it has none. */
export function statusShareUrl(status: Pick<Status, "url" | "uri">): string {
  return status.url ?? status.uri;
}
