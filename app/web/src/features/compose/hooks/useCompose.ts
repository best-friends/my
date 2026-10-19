// SPDX-License-Identifier: AGPL-3.0-only
import { useQueryClient } from "@tanstack/react-query";
import { createContext, use, useState, useSyncExternalStore } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useViewer } from "../../session";
import { cacheReplyTarget, type Status } from "../api/compose";
import { type ComposeState, DraftStore, draftStorageKey } from "./draftStore";
import { replyMentions } from "./policy";

/** Where the form is: always on screen in a side panel (wide screens), or in a sheet opened by a button. */
export type ComposeLayout = "panel" | "sheet";

/** The id of the post's text area, which a reply focuses. Only one form is on the page at a time. */
export const composeTextId = "compose-text";

/** What the compose provider shares; it changes only when the account, the layout or the sheet changes. */
export type ComposeController = {
  accountId: string;
  store: DraftStore;
  layout: ComposeLayout;
  isSheetOpen: boolean;
  setSheetOpen: (isOpen: boolean) => void;
  /** Starts a reply to `status`: the form names it and starts with its mentions, then gets focus. */
  reply: (status: Status) => void;
};

export const ComposeContext = createContext<ComposeController | null>(null);

/** The tab's storage, or none where the browser refuses it (a sandboxed frame). */
function tabStorage(): Storage | null {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

/** The controller for the signed-in viewer, or null while nobody is signed in. */
export function useComposeController(layout: ComposeLayout): ComposeController | null {
  const viewer = useViewer();
  const api = useApi();
  const queryClient = useQueryClient();
  const accountId = viewer.status === "signed-in" ? viewer.account.id : null;
  // One store per account: when the account changes, the store is replaced during rendering, as
  // react.dev describes for state that depends on a changed value ("Adjusting some state when a prop changes").
  const [store, setStore] = useState<{ accountId: string; store: DraftStore } | null>(null);
  if (accountId !== (store?.accountId ?? null)) {
    setStore(accountId === null ? null : { accountId, store: new DraftStore(tabStorage(), draftStorageKey(accountId)) });
  }
  const [isSheetOpen, setIsSheetOpen] = useState(false);
  if (store === null || accountId === null || store.accountId !== accountId) return null;
  const draftStore = store.store;
  return {
    accountId,
    store: draftStore,
    layout,
    isSheetOpen,
    setSheetOpen: setIsSheetOpen,
    reply(status) {
      const { draft } = draftStore.getSnapshot();
      draftStore.edit({
        replyToId: status.id,
        visibility: null,
        text: draft.text.trim() === "" ? replyMentions(status, accountId) : draft.text,
      });
      cacheReplyTarget(api, queryClient, status);
      if (layout === "sheet") setIsSheetOpen(true);
      else document.getElementById(composeTextId)?.focus();
    },
  };
}

/** The compose controller; null while nobody is signed in. */
export function useComposeContext(): ComposeController | null {
  return use(ComposeContext);
}

/** The compose state of the controller's store. */
export function useComposeState(store: DraftStore): ComposeState {
  return useSyncExternalStore(store.subscribe, store.getSnapshot);
}

/**
 * Returns `reply(status)`, which opens the form as a reply to `status`, or null while nobody is signed in
 * (the reply buttons of other features hide then).
 */
export function useReply(): ((status: Status) => void) | null {
  return useComposeContext()?.reply ?? null;
}
