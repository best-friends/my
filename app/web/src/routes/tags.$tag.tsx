// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { useId } from "react";
import { z } from "zod";
import { TimelineFeed, TimelineSkeleton, timelineQueryOptions } from "../features/timeline";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** The hashtag, without the `#` (api/openapi.yaml, getHashtagTimeline); the hashtag links of posts point here. */
const paramsSchema = z.object({ tag: z.string().min(1).max(256) });

/** The timeline of one hashtag (FR-203). */
export const Route = createFileRoute("/tags/$tag")({
  params: { parse: (params) => paramsSchema.parse(params) },
  loader: async ({ context: { queryClient, api }, params: { tag } }) => {
    // The cached pages are shown as they are; the page refetches them in the background when stale.
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "tag", tag }), staleTime: "static" });
  },
  component: TagPage,
  pendingComponent: TagPending,
});

function TagHeading({ tag, id }: { tag: string; id?: string }) {
  const t = useT();
  return (
    <PageHeading {...(id !== undefined && { id })}>
      {/* The hashtag is the user's text, in any script (web-client.md Y10). */}
      <span dir="auto">{t("client:tag.title", { tag })}</span>
    </PageHeading>
  );
}

function TagPage() {
  const { tag } = Route.useParams();
  const headingId = useId();
  return (
    <>
      <TagHeading tag={tag} id={headingId} />
      {/* A new hashtag is a new timeline: nothing of the previous one carries over (web-client.md R10). */}
      <TimelineFeed key={tag} timeline={{ kind: "tag", tag }} aria-labelledby={headingId} />
    </>
  );
}

function TagPending() {
  const { tag } = Route.useParams();
  return (
    <>
      <TagHeading tag={tag} />
      <TimelineSkeleton />
    </>
  );
}
