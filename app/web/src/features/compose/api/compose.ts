// SPDX-License-Identifier: AGPL-3.0-only
// The server data of the compose feature (web-client.md S1): the limits and preferences a new post
// follows, the post being replied to, the uploaded media while the server processes them, and the
// writes (uploading a file, posting). Types come from the spec (S9).
import type { QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";

export type Status = components["schemas"]["Status"];
export type MediaAttachment = components["schemas"]["MediaAttachment"];
export type Visibility = components["schemas"]["Visibility"];
export type CreateStatusRequest = components["schemas"]["CreateStatusRequest"];

/** The instance, for `limits.max_post_characters`. */
export function instanceQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/instance");
}

/** The viewer's preferences: the default visibility and language, and whether media need a description. */
export function preferencesQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/preferences");
}

/** A status, for the post being replied to. */
export function statusQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/statuses/{id}", { params: { path: { id } } });
}

/** How often an uploaded file is polled while the server processes it, in ms. */
const mediaPollInterval = 1000;

/**
 * An uploaded media file. While it is `processing`, the query polls it until it is `ready` or `failed`
 * (the uploadMedia operation's contract).
 */
export function mediaQueryOptions(api: Api, id: string) {
  return api.query.queryOptions(
    "get",
    "/media/{id}",
    { params: { path: { id } } },
    { refetchInterval: (query) => (query.state.data?.state === "processing" ? mediaPollInterval : false) },
  );
}

/** Caches an upload's answer, so that its query starts from it (and polls only while it is processing). */
export function cacheUploadedMedia(api: Api, queryClient: QueryClient, media: MediaAttachment): void {
  queryClient.setQueryData(mediaQueryOptions(api, media.id).queryKey, media);
}

/** Uploads one image (POST /media, 202) and returns the file in its current state, usually `processing`. */
export async function uploadMedia(api: Api, file: File): Promise<MediaAttachment> {
  const { data } = await api.fetch.POST("/media", {
    // The body is serialized by hand: openapi-typescript types a binary part as a string.
    body: { file: file.name },
    bodySerializer() {
      const form = new FormData();
      form.append("file", file, file.name);
      return form;
    },
  });
  // The client's middleware throws for every error status, so a response without data cannot happen.
  if (data === undefined) throw new Error("POST /media answered without a body");
  return data;
}

/**
 * Posts a status. `idempotencyKey` stays the same for every retry of one draft, so a retry after a lost
 * response returns the status already posted instead of posting it twice.
 */
export async function createStatus(api: Api, body: CreateStatusRequest, idempotencyKey: string): Promise<Status> {
  const { data } = await api.fetch.POST("/statuses", { params: { header: { "Idempotency-Key": idempotencyKey } }, body });
  if (data === undefined) throw new Error("POST /statuses answered without a body");
  return data;
}

/** The paths whose cached lists a new post can appear in. */
const listsWithNewPosts: ReadonlySet<string> = new Set([
  "/timelines/home",
  "/timelines/local",
  "/timelines/tag/{hashtag}",
  "/accounts/{id}/statuses",
  "/statuses/{id}/context",
]);

/** Puts a new post into the cache and refetches the lists it can appear in. */
export async function cachePostedStatus(api: Api, queryClient: QueryClient, status: Status): Promise<void> {
  queryClient.setQueryData(statusQueryOptions(api, status.id).queryKey, status);
  // openapi-react-query keys are [method, path, init].
  await queryClient.invalidateQueries({ predicate: (query) => typeof query.queryKey[1] === "string" && listsWithNewPosts.has(query.queryKey[1]) });
}
