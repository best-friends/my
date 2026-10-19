// SPDX-License-Identifier: AGPL-3.0-only
// The moderation actions on domains (FR-702): the list under a filter, taking a new action (a suspension only
// after its confirmation, NFR-MOD-7), and lifting one. The server keeps domains in IDNA A-labels; the UI shows
// them with the U-labels they stand for (lib/idn).
import { useToast } from "@my/ui";
import { type InfiniteData, useMutation, useMutationState, useQueryClient, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type ClientMessageKey, type Translate, useT } from "../../../lib/i18n";
import { domainToUnicode } from "../../../lib/idn";
import {
  createDomainActionMutation,
  type DomainAction,
  type DomainActionKind,
  domainActionsQueryOptions,
  type DomainActionScope,
  isActiveOnlyQuery,
  isDomainActionsQuery,
  liftDomainActionMutation,
  type NewDomainAction,
  refreshAuditLog,
  refreshDomainActionLists,
} from "../api/admin";
import { type PagedList, toPagedList } from "./pagedList";

export const domainActionKindLabels: Readonly<Record<DomainActionKind, ClientMessageKey>> = {
  limit: "client:admin.domain_actions.kind.limit",
  suspend: "client:admin.domain_actions.kind.suspend",
  reject_media: "client:admin.domain_actions.kind.reject_media",
  reject_reports: "client:admin.domain_actions.kind.reject_reports",
  force_sensitive: "client:admin.domain_actions.kind.force_sensitive",
};

/** A domain for a sentence: its U-labels with its A-labels after them when they differ, isolated from the text around it. */
export function domainInText(t: Translate, domain: string): string {
  const unicode = domainToUnicode(domain);
  const text = unicode === domain ? domain : t("client:admin.domain.both", { unicode, ascii: domain });
  // A domain reads left to right in every locale (U+2066 LEFT-TO-RIGHT ISOLATE … U+2069 POP DIRECTIONAL ISOLATE).
  return `⁦${text}⁩`;
}

/** The domain actions of `scope`, newest first. */
export function useDomainActions(scope: DomainActionScope): PagedList<DomainAction> {
  const api = useApi();
  return toPagedList(useSuspenseInfiniteQuery(domainActionsQueryOptions(api, scope)));
}

export type LiftDomainAction = {
  /** Lifts the action; `onLifted` says so, and a failure is shown in a toast. */
  lift: (action: DomainAction) => void;
  /** The IDs of the actions being lifted. */
  pendingIds: readonly string[];
};

/**
 * Lifting domain actions. `onLifted(action)` is called with the lifted action, once the audit log is fetched
 * again, just before the lists change: the lists of actions in effect drop it (to keep focus,
 * lib/useFocusAfterRemoval), the others show it lifted.
 * Several actions may be lifted at once, so everything after a success is in the mutation's own callbacks.
 */
export function useLiftDomainAction(onLifted: (action: DomainAction) => void): LiftDomainAction {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const toast = useToast();
  const options = liftDomainActionMutation(api);
  const mutation = useMutation({
    ...options,
    onSuccess: async (lifted) => {
      // The audit log records the lift: the action is lifted once it is fetched again, so that going back to it
      // shows the lift at once (refreshAuditLog). The lists change from the answer, which shows them right.
      await refreshAuditLog(queryClient);
      onLifted(lifted);
      for (const query of queryClient.getQueryCache().findAll({ predicate: (query) => isDomainActionsQuery(query.queryKey) })) {
        const keep = isActiveOnlyQuery(query.queryKey) ? [] : [lifted];
        queryClient.setQueryData<InfiniteData<{ items: DomainAction[] }>>(query.queryKey, (data) =>
          data && { ...data, pages: data.pages.map((page) => ({ ...page, items: page.items.flatMap((item) => (item.id === lifted.id ? keep : [item])) })) },
        );
      }
    },
    // The action stays in effect, and its row and focus stay (web-client.md E2).
    onError: (error) => {
      toast.show({ message: t("client:admin.domain_actions.lift_failed", { reason: errorMessage(error) }), tone: "danger" });
    },
  });
  const pendingIds = useMutationState({
    filters: { mutationKey: options.mutationKey, status: "pending" },
    select: (pending) => pending.state.variables as string,
  });
  return {
    lift: (action) => {
      mutation.mutate(action.id);
    },
    pendingIds,
  };
}

/** The form of a new domain action, filled in by the operator. */
type Draft = {
  domain: string;
  kind: DomainActionKind | null;
  publicComment: string;
  privateNote: string;
  obfuscate: boolean;
  /** The seconds the action lasts, or null until it is lifted. */
  expiresIn: number | null;
};

function emptyDraft(domain: string): Draft {
  return { domain, kind: null, publicComment: "", privateNote: "", obfuscate: false, expiresIn: null };
}

/** The request of a draft whose kind is chosen; empty texts are left out. */
function requestOf(draft: Draft, kind: DomainActionKind): NewDomainAction {
  return {
    domain: draft.domain.trim(),
    kind,
    obfuscate: draft.obfuscate,
    ...(draft.publicComment === "" ? {} : { public_comment: draft.publicComment }),
    ...(draft.privateNote === "" ? {} : { private_note: draft.privateNote }),
    ...(draft.expiresIn === null ? {} : { expires_in: draft.expiresIn }),
  };
}

/** What the form of a new domain action needs. */
export type DomainActionForm = {
  draft: Draft;
  change: (change: Partial<Draft>) => void;
  /** Takes the action once the browser has checked the fields; a suspension asks first. */
  submit: (event: SyntheticEvent<HTMLFormElement>) => void;
  isPending: boolean;
  /** The confirmation of a suspension: its domain, or null while it is closed. */
  confirmation: { domain: string; confirm: () => void; cancel: () => void } | null;
  /** Why the action failed, localized: in the confirmation while it is open, else under the form. */
  error: string | undefined;
  /** What the last action did, for the form's StatusMessage; undefined before the first one. */
  status: string | undefined;
};

/**
 * The form of a new domain action, starting with `initialDomain`. A success empties the form (keeping the
 * domain, which the next action often shares) and says what took effect once every cached list is fetched again.
 */
export function useDomainActionForm(initialDomain: string): DomainActionForm {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(() => emptyDraft(initialDomain));
  const [confirming, setConfirming] = useState(false);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const create = useMutation({
    ...createDomainActionMutation(api),
    onSuccess: async (action) => {
      // The action is in every list, of the other tabs and domains too, and in the audit log: it is taken once
      // they are fetched again (the form stays pending until then), so that another tab or the log shows it at
      // once (refreshDomainActionLists, refreshAuditLog).
      await Promise.all([refreshDomainActionLists(queryClient), refreshAuditLog(queryClient)]);
      setConfirming(false);
      setDraft(emptyDraft(draft.domain));
      setStatus(t("client:admin.domain_actions.issued", { kind: t(domainActionKindLabels[action.kind]), domain: domainInText(t, action.domain) }));
    },
  });
  const run = () => {
    if (draft.kind !== null) create.mutate(requestOf(draft, draft.kind));
  };
  return {
    draft,
    change: (change) => {
      setDraft({ ...draft, ...change });
    },
    submit: (event) => {
      event.preventDefault();
      if (create.isPending || draft.kind === null) return;
      create.reset();
      if (draft.kind === "suspend") setConfirming(true);
      else run();
    },
    isPending: create.isPending,
    confirmation: confirming
      ? {
          domain: draft.domain.trim(),
          confirm: run,
          cancel: () => {
            setConfirming(false);
            create.reset();
          },
        }
      : null,
    error: create.isError ? errorMessage(create.error) : undefined,
    status,
  };
}
