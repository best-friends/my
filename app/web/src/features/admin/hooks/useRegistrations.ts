// SPDX-License-Identifier: AGPL-3.0-only
// The review of registrations (FR-721): the sign-ups that await approval and deciding one through its
// confirmation, with an optional note for the moderators. A decided registration no longer awaits approval,
// so it leaves the cached pages at once, and what the decision did stays on the page (role="status").
import { useMutation, useQueryClient, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type Translate, useT } from "../../../lib/i18n";
import { refreshAuditLog } from "../api/admin";
import { decideRegistrationMutation, isAdminCountsQuery, type Registration, type RegistrationDecision, registrationsQueryOptions } from "../api/registrations";
import { type PagedList, toPagedList } from "./pagedList";

/** The registrations that await approval, longest waiting first. */
export function useRegistrations(): PagedList<Registration> {
  const api = useApi();
  return toPagedList(useSuspenseInfiniteQuery(registrationsQueryOptions(api)));
}

/**
 * Who signed up, for a sentence: `@username`, isolated from the text around it (a username reads left to
 * right in every locale; U+2066 LEFT-TO-RIGHT ISOLATE … U+2069 POP DIRECTIONAL ISOLATE), or a word for a user
 * whose account is gone.
 */
export function applicantInText(t: Translate, registration: Registration): string {
  if (registration.username === undefined) return t("client:admin.registrations.account_gone");
  return `⁦@${registration.username}⁩`;
}

/**
 * What a decision did: the state it left the registration in, and whether that was another moderator's
 * earlier decision, which stands. A decision always leaves a registration decided.
 */
function resultMessage(t: Translate, registration: Registration, decision: RegistrationDecision): string {
  const applicant = applicantInText(t, registration);
  if (registration.state === "approved") {
    return decision === "approve" ? t("client:admin.registrations.approved", { applicant }) : t("client:admin.registrations.already_approved", { applicant });
  }
  return decision === "reject" ? t("client:admin.registrations.rejected", { applicant }) : t("client:admin.registrations.already_rejected", { applicant });
}

/** The decision being confirmed: on which registration, which, and the note typed so far. */
type Confirmation = { registration: Registration; decision: RegistrationDecision; note: string };

/** The confirmation dialog: open, or closing and still showing the last confirmation while it fades out. */
type DialogState = { isOpen: boolean; confirmation: Confirmation } | null;

/** What the list of registrations needs to decide them. */
export type RegistrationReview = {
  /** Opens the confirmation of `decision` on `registration`. */
  open: (registration: Registration, decision: RegistrationDecision) => void;
  /** Whether the confirmation is open. */
  isOpen: boolean;
  /** The confirmation open now or last, which a closing dialog still shows; null before the first one. */
  confirmation: Confirmation | null;
  changeNote: (note: string) => void;
  /** Sends the decision; on success the confirmation closes and the registration leaves the list. */
  confirm: () => void;
  /** Closes the confirmation without deciding. */
  cancel: () => void;
  isPending: boolean;
  /** Why the decision failed, localized, for the confirmation (which stays open to try again). */
  error: string | undefined;
  /** What the last decision did, for the list's StatusMessage; undefined before the first one. */
  status: string | undefined;
};

/**
 * Deciding registrations. `onDecided(userId)` is called, once the audit log is fetched again, just before the
 * registration leaves the list (to keep focus, lib/useFocusAfterRemoval). A registration that another moderator
 * decided first keeps that decision; it leaves the list too, and the status says which decision stands.
 */
export function useRegistrationReview(onDecided: (userId: string) => void): RegistrationReview {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const [dialog, setDialog] = useState<DialogState>(null);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const listKey = registrationsQueryOptions(api).queryKey;
  const mutation = useMutation({
    ...decideRegistrationMutation(api),
    onSuccess: async (registration, { userId, decision }) => {
      // The audit log records the decision: it is done once the log is fetched again (the confirmation stays
      // pending until then), so that going back to the log shows it at once (refreshAuditLog).
      await refreshAuditLog(queryClient);
      setDialog((current) => current && { ...current, isOpen: false });
      onDecided(userId);
      queryClient.setQueryData(
        listKey,
        (data) => data && { ...data, pages: data.pages.map((page) => ({ ...page, items: page.items.filter((item) => item.user_id !== userId) })) },
      );
      void queryClient.invalidateQueries({ predicate: (query) => isAdminCountsQuery(query.queryKey) });
      setStatus(resultMessage(t, registration, decision));
    },
  });
  const isOpen = dialog?.isOpen ?? false;
  return {
    open: (registration, decision) => {
      mutation.reset();
      setDialog({ isOpen: true, confirmation: { registration, decision, note: "" } });
    },
    isOpen,
    confirmation: dialog?.confirmation ?? null,
    changeNote: (note) => {
      if (dialog !== null) setDialog({ ...dialog, confirmation: { ...dialog.confirmation, note } });
    },
    confirm: () => {
      if (dialog === null || !dialog.isOpen || mutation.isPending) return;
      const { registration, decision, note } = dialog.confirmation;
      mutation.mutate({ userId: registration.user_id, decision, note: note.trim() });
    },
    cancel: () => {
      if (dialog !== null) setDialog({ ...dialog, isOpen: false });
      mutation.reset();
    },
    isPending: mutation.isPending,
    error: isOpen && mutation.isError ? errorMessage(mutation.error) : undefined,
    status,
  };
}
