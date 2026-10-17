// SPDX-License-Identifier: AGPL-3.0-only
import { Button, PostSkeleton, VisuallyHidden } from "@my/ui";
import { useId, useRef } from "react";
import { useT } from "../../../lib/i18n";
import { useRevealOnce } from "../hooks/useRevealOnce";
import { type ThreadPost, type ThreadState, useStatusThread } from "../hooks/useStatusThread";
import { StatusCard } from "./StatusCard";

export type StatusThreadProps = {
  /** The ID of the post the page is about; the route loader has fetched it. */
  id: string;
  /** Called after the viewer deleted that post, so the page can leave it. */
  onDeleted: () => void;
};

/**
 * A post and the thread around it (FR-113): the posts it replies to, oldest first, under their own
 * heading; the post itself, highlighted, with its full date; and the replies, depth first, under theirs.
 * A line joins each post to the one it replies to when that is right above it. The thread loads after
 * the post, so the post is on screen at once; when posts appear above it, the page scrolls it back to the
 * top of the screen.
 */
export function StatusThread({ id, onDeleted }: StatusThreadProps) {
  const t = useT();
  const { status, thread } = useStatusThread(id);
  const ancestorsId = useId();
  const repliesId = useId();
  const focusedRef = useRef<HTMLElement>(null);
  const hasAncestors = thread.status === "loaded" && thread.ancestors.length > 0;
  useRevealOnce(focusedRef, hasAncestors);

  return (
    <div className="flex flex-col">
      {thread.status === "loaded" && thread.ancestors.length > 0 && (
        <section aria-labelledby={ancestorsId}>
          <h2 id={ancestorsId} className="px-4 pt-2 text-sm font-semibold text-fg-muted">
            {t("client:status.thread.ancestors")}
          </h2>
          <StatusList posts={thread.ancestors} />
        </section>
      )}
      <StatusCard
        ref={focusedRef}
        status={status}
        variant="focused"
        thread={{ lineAbove: thread.status === "loaded" && thread.repliesToAncestor, lineBelow: false }}
        onDeleted={onDeleted}
      />
      <section aria-labelledby={repliesId} aria-busy={thread.status === "loading"}>
        <h2 id={repliesId} className="px-4 pt-4 pb-2 text-lg font-semibold">
          {t("client:status.thread.replies")}
        </h2>
        <ThreadReplies thread={thread} />
      </section>
    </div>
  );
}

/** Placeholder replies, for as long as the thread loads. */
const replyPlaceholders = ["first", "second"] as const;

function ThreadReplies({ thread }: { thread: ThreadState }) {
  const t = useT();
  switch (thread.status) {
    case "loading":
      // The section is busy meanwhile; the hidden text names what it waits for, once (web-client.md G15).
      return (
        <>
          <VisuallyHidden>{t("client:status.thread.loading")}</VisuallyHidden>
          {replyPlaceholders.map((key) => (
            <PostSkeleton key={key} />
          ))}
        </>
      );
    case "error":
      return (
        <div className="flex flex-col items-center gap-3 p-4 text-center">
          <p>{t("client:status.thread.failed", { reason: thread.message })}</p>
          <Button onPress={thread.retry}>{t("client:status.thread.retry")}</Button>
        </div>
      );
    case "loaded":
      return thread.descendants.length === 0 ? (
        <p className="px-4 pb-4 text-fg-muted">{t("client:status.thread.no_replies")}</p>
      ) : (
        <StatusList posts={thread.descendants} />
      );
  }
}

function StatusList({ posts }: { posts: readonly ThreadPost[] }) {
  return (
    <ol>
      {posts.map(({ status, lineAbove, lineBelow }) => (
        <li key={status.id}>
          <StatusCard status={status} thread={{ lineAbove, lineBelow }} />
        </li>
      ))}
    </ol>
  );
}
