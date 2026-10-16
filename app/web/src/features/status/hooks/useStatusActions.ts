// SPDX-License-Identifier: AGPL-3.0-only
// The viewer's actions on a post (FR-106 to FR-108, FR-113): boost, favourite and bookmark are turned on
// and off optimistically in every cached copy of the post and restored when the server refuses
// (web-client.md S8); delete waits for the server, then drops the post from every list. Each result is
// announced (Y7) and a failure is also shown as a toast, because the restored button alone tells a
// sighted user nothing (E2).
import { useAnnouncer, useToast } from "@my/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type Translate, useT } from "../../../lib/i18n";
import { useReply } from "../../compose";
import { useViewer } from "../../session";
import { forPost, hasRelation, relationCount, updateCachedStatuses, withRelation, withRelationFrom } from "../api/cache";
import { deleteStatus, type Relation, setRelation, type Status, statusContextQueryOptions, statusQueryOptions, statusShareUrl } from "../api/status";

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
  /** Copies the post's address to the clipboard. */
  copyLink: () => void;
  /**
   * Deletes the post, for the viewer's own posts; null otherwise. Resolves to whether the post is gone; a
   * failure is already shown.
   */
  delete: (() => Promise<boolean>) | null;
  /** Whether the post is being deleted. */
  isDeleting: boolean;
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
function useRelation(post: Status, relation: Relation): RelationToggle {
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
    onMutate: async ({ on }) => {
      // A refetch of the post that is under way would overwrite the optimistic state with the old one.
      await queryClient.cancelQueries({ queryKey: statusQueryOptions(api, post.id).queryKey });
      updateCachedStatuses(queryClient, forPost(post.id, (status) => withRelation(status, relation, on)));
    },
    onSuccess: (answer, { on }) => {
      // The server's count includes others' actions. Take it only from the last press, so that an answer
      // to an earlier press does not undo a later one that is still on its way.
      if (queryClient.isMutating({ mutationKey, exact: true }) <= 1) {
        const from = answer.boost ?? answer;
        updateCachedStatuses(queryClient, forPost(post.id, (status) => withRelationFrom(status, relation, from)));
      }
      announce(relationMessage(t, relation, on));
    },
    onError: (error, { on }) => {
      updateCachedStatuses(queryClient, forPost(post.id, (status) => withRelation(status, relation, !on)));
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
  const boost = useRelation(post, "boost");
  const favourite = useRelation(post, "favourite");
  const bookmark = useRelation(post, "bookmark");
  const remove = useMutation({
    mutationFn: () => deleteStatus(api, post.id),
    onSuccess: () => {
      // Its own query and thread go; every list drops it and the boosts of it; its parent has one reply less.
      queryClient.removeQueries({ queryKey: statusQueryOptions(api, post.id).queryKey, exact: true });
      queryClient.removeQueries({ queryKey: statusContextQueryOptions(api, post.id).queryKey, exact: true });
      updateCachedStatuses(queryClient, forPost(post.id, () => null));
      const parentId = post.in_reply_to_id;
      if (parentId !== undefined) {
        updateCachedStatuses(queryClient, forPost(parentId, (parent) => ({ ...parent, replies_count: Math.max(0, parent.replies_count - 1) })));
      }
      toast.show({ message: t("client:status.delete.done"), tone: "success" });
    },
    onError: (error) => {
      toast.show({ message: t("client:status.delete.failed", { reason: errorMessage(error) }), tone: "danger" });
    },
  });

  if (viewer.status !== "signed-in" || reply === null) return null;
  const isAuthor = post.account.id === viewer.account.id;
  return {
    reply: () => {
      reply(post);
    },
    boost: { ...boost, isDisabled: !canOfferBoost(post.visibility, isAuthor) },
    favourite,
    bookmark,
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
    isDeleting: remove.isPending,
  };
}
