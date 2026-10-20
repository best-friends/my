// SPDX-License-Identifier: AGPL-3.0-only
// The viewer's actions on a post (FR-106 to FR-109, FR-113, FR-116): boost, favourite, bookmark and the emoji
// reaction (which shares the Like slot with the favourite, useLikeSlot) are turned on and off optimistically in every cached copy of the post and restored when the server refuses
// (web-client.md S8); delete waits for the server, then drops the post from every list, and a pin (FR-403)
// waits for it, then marks the post and reads the viewer's pinned posts again. Each result is
// announced (Y7) and a failure is also shown as a toast, because the restored button alone tells a
// sighted user nothing (E2).
import { useAnnouncer, useToast } from "@my/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type Translate, useT } from "../../../lib/i18n";
import { useReply } from "../../compose";
import { useViewer } from "../../session";
import { hasRelation, relationCount } from "../api/cache";
import { useLikeSlot } from "./useLikeSlot";
import {
  cacheDeletedStatus,
  cachePinAnswer,
  cacheRelationAnswer,
  cacheRelationPending,
  deleteStatus,
  type ReactionEmoji,
  type Relation,
  restoreRelation,
  setPin,
  setRelation,
  type Status,
  statusShareUrl,
} from "../api/status";

/** One toggle of the action bar. */
export type RelationToggle = {
  isSelected: boolean;
  /** The public count, or null for bookmarks. */
  count: number | null;
  /** Whether the server would refuse it (a boost of a followers-only post by another account, or of a direct one). */
  isDisabled: boolean;
  onChange: (on: boolean) => void;
};

export type StatusActions = {
  /** Opens the form as a reply to the post. */
  reply: () => void;
  boost: RelationToggle;
  favourite: RelationToggle;
  bookmark: RelationToggle;
  /**
   * Reacts to the post with `emoji`, in place of the viewer's favourite or other reaction (ADR-0052 §1); reacting
   * with the emoji the viewer reacted with takes it back.
   */
  react: (emoji: ReactionEmoji) => void;
  /** Copies the post's address to the clipboard. */
  copyLink: () => void;
  /**
   * Deletes the post, for the viewer's own posts; null otherwise. Resolves to whether the post is gone; a
   * failure is already shown.
   */
  delete: (() => Promise<boolean>) | null;
  /** Whether the viewer can edit the post: their own posts without a poll only (FR-109, FR-112: its votes belong to the poll as posted). */
  canEdit: boolean;
  /** Whether the post is being deleted. */
  isDeleting: boolean;
  /** Whether the viewer can report the post's author: anyone's posts but the viewer's own (FR-701). */
  canReport: boolean;
  /**
   * Pins the post to the viewer's profile or unpins it (FR-403), for the viewer's own posts that are not
   * direct (the server's rule); null otherwise. The result or the failure (the limit of pins, say) is shown.
   */
  pin: { isPinned: boolean; toggle: () => void } | null;
};

/** What is announced when the viewer turns `relation` on or off. */
function relationMessage(t: Translate, relation: Relation, on: boolean): string {
  switch (relation) {
    case "boost":
      return on ? t("client:status.boost.on") : t("client:status.boost.off");
    case "favourite":
      return on ? t("client:status.favourite.on") : t("client:status.favourite.off");
    case "bookmark":
      return on ? t("client:status.bookmark.on") : t("client:status.bookmark.off");
  }
}

/**
 * Whether the server lets the viewer boost a post of `visibility` (the client API's 422 otherwise):
 * public, local and unlisted posts by anyone, a followers-only post by its author only, a direct post by
 * no one. It only decides whether to offer the button; the server decides the boost.
 */
function canOfferBoost(visibility: Status["visibility"], isAuthor: boolean): boolean {
  return visibility === "private" ? isAuthor : visibility !== "direct";
}

/** The mutation of one relation of post `post`, serialized per post and relation so that the last press wins. */
function useRelation(post: Status, relation: Relation, viewerId: string): RelationToggle {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const announce = useAnnouncer();
  const toast = useToast();
  const mutationKey = ["status", post.id, relation] as const;
  const { mutate } = useMutation({
    mutationKey,
    // One request at a time per post and relation, in the order pressed (TanStack Query, "Mutation Scopes").
    scope: { id: `status:${post.id}:${relation}` },
    mutationFn: ({ on }: { on: boolean }) => setRelation(api, relation, post.id, on),
    onMutate: ({ on }) => cacheRelationPending(api, queryClient, post.id, relation, on),
    onSuccess: (answer, { on }) => {
      // The last press is the only one still running.
      const last = queryClient.isMutating({ mutationKey, exact: true }) <= 1;
      cacheRelationAnswer(queryClient, post.id, relation, answer, last, viewerId);
      announce(relationMessage(t, relation, on));
    },
    onError: (error, { on }) => {
      restoreRelation(queryClient, post.id, relation, on);
      toast.show({ message: t("client:status.action.failed", { reason: errorMessage(error) }), tone: "danger" });
    },
  });
  return {
    isSelected: hasRelation(post, relation),
    count: relationCount(post, relation),
    isDisabled: false,
    onChange: (on) => {
      mutate({ on });
    },
  };
}

/**
 * The actions on `status`, or null while nobody is signed in. For a boost, the actions are on the boosted
 * post. Read `status` from the query cache on every render; the hook never copies it (S2).
 */
export function useStatusActions(status: Status): StatusActions | null {
  const post = status.boost ?? status;
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const toast = useToast();
  const viewer = useViewer();
  const reply = useReply();
  const viewerId = viewer.status === "signed-in" ? viewer.account.id : "";
  const boost = useRelation(post, "boost", viewerId);
  const likeSlot = useLikeSlot(post);
  const bookmark = useRelation(post, "bookmark", viewerId);
  const remove = useMutation({
    mutationFn: () => deleteStatus(api, post.id),
    onSuccess: () => {
      cacheDeletedStatus(api, queryClient, post);
      toast.show({ message: t("client:status.delete.done"), tone: "success" });
    },
    onError: (error) => {
      toast.show({ message: t("client:status.delete.failed", { reason: errorMessage(error) }), tone: "danger" });
    },
  });

  const pin = useMutation({
    mutationFn: ({ on }: { on: boolean }) => setPin(api, post.id, on),
    // One request at a time per post, in the order chosen.
    scope: { id: `status:${post.id}:pin` },
    onSuccess: async (answer, { on }) => {
      await cachePinAnswer(api, queryClient, post.id, answer, viewerId);
      // The toast is announced too.
      toast.show({ message: on ? t("client:status.pin.done") : t("client:status.unpin.done"), tone: "success" });
    },
    onError: (error, { on }) => {
      const reason = errorMessage(error);
      toast.show({ message: on ? t("client:status.pin.failed", { reason }) : t("client:status.unpin.failed", { reason }), tone: "danger" });
    },
  });

  if (viewer.status !== "signed-in" || reply === null) return null;
  const isAuthor = post.account.id === viewer.account.id;
  const isPinned = post.pinned === true;
  return {
    reply: () => {
      reply(post);
    },
    boost: { ...boost, isDisabled: !canOfferBoost(post.visibility, isAuthor) },
    favourite: {
      isSelected: likeSlot.slot.kind === "favourite",
      count: post.favourites_count,
      isDisabled: false,
      onChange: (on) => {
        likeSlot.set(on ? { kind: "favourite" } : { kind: "none" });
      },
    },
    bookmark,
    react: (emoji) => {
      const { slot } = likeSlot;
      likeSlot.set(slot.kind === "reaction" && slot.emoji.name === emoji.name ? { kind: "none" } : { kind: "reaction", emoji });
    },
    copyLink: () => {
      navigator.clipboard.writeText(statusShareUrl(post)).then(
        () => {
          // The toast is announced too.
          toast.show({ message: t("client:status.link.copied"), tone: "success" });
        },
        () => {
          toast.show({ message: t("client:status.link.failed"), tone: "danger" });
        },
      );
    },
    delete: isAuthor ? () => remove.mutateAsync().then(() => true, () => false) : null,
    canEdit: isAuthor && post.poll === undefined,
    isDeleting: remove.isPending,
    canReport: !isAuthor,
    pin:
      isAuthor && post.visibility !== "direct"
        ? {
            isPinned,
            toggle: () => {
              pin.mutate({ on: !isPinned });
            },
          }
        : null,
  };
}
