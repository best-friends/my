// SPDX-License-Identifier: AGPL-3.0-only
// Uploading the images of a profile (the avatar and the header): the server accepts a file, processes it in
// the background, and is polled until the file is ready (api/openapi.yaml, uploadMedia and getMedia).
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";

export type MediaAttachment = components["schemas"]["MediaAttachment"];

/** The image types the server accepts (uploadMedia). */
export const acceptedImageTypes = ["image/jpeg", "image/png", "image/gif", "image/webp"] as const;

/** How long to wait between two polls of a file still processing, and how many polls to make at most. */
const pollInterval = 1000;
const maxPolls = 60;

/** The server could not process the image, or did not finish in time. */
export class MediaProcessingError extends Error {
  override name = "MediaProcessingError";
}

/** Uploads `file` and resolves with it once the server has processed it. */
export async function uploadImage(api: Api, file: File, signal?: AbortSignal): Promise<MediaAttachment> {
  const { data: uploaded } = await api.fetch.POST("/media", {
    // The generated type of a binary property is string; the serializer sends the File itself.
    body: { file: "" },
    bodySerializer: () => {
      const form = new FormData();
      form.append("file", file);
      return form;
    },
    ...(signal === undefined ? {} : { signal }),
  });
  let media = uploaded;
  for (let poll = 0; media?.state === "processing"; poll++) {
    if (poll >= maxPolls) throw new MediaProcessingError(`media ${media.id} is still processing`);
    await delay(pollInterval, signal);
    const { data } = await api.fetch.GET("/media/{id}", { params: { path: { id: media.id } }, ...(signal === undefined ? {} : { signal }) });
    media = data;
  }
  if (media?.state !== "ready") throw new MediaProcessingError(`media ${media?.id ?? "?"} failed`);
  return media;
}

function delay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener(
      "abort",
      () => {
        clearTimeout(timer);
        reject(signal.reason as Error);
      },
      { once: true },
    );
  });
}
