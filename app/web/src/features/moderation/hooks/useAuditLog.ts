// SPDX-License-Identifier: AGPL-3.0-only
// The audit log (FR-704): the operations under a filter, newest first, and loading more of them. The route keeps
// the filter and the number of loaded pages in the address (web-client.md S3), so that a reload or a shared link
// shows the same entries; the list tells it how many pages it has after each one it loads.
import { useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type ClientMessageKey } from "../../../lib/i18n";
import { type AuditAction, type AuditActorKind, type AuditEntry, type AuditLogFilter, auditLogQueryOptions, type AuditTargetKind } from "../api/staff";
import type { PagedList } from "./pagedList";

export const auditActionLabels: Readonly<Record<AuditAction, ClientMessageKey>> = {
  account_action_issued: "client:admin.audit_log.action.account_action_issued",
  account_action_lifted: "client:admin.audit_log.action.account_action_lifted",
  revealed_private_data: "client:admin.audit_log.action.revealed_private_data",
  two_factor_disabled: "client:admin.audit_log.action.two_factor_disabled",
  domain_action_issued: "client:admin.audit_log.action.domain_action_issued",
  domain_action_lifted: "client:admin.audit_log.action.domain_action_lifted",
  report_assigned: "client:admin.audit_log.action.report_assigned",
  report_unassigned: "client:admin.audit_log.action.report_unassigned",
  report_resolved: "client:admin.audit_log.action.report_resolved",
  report_reopened: "client:admin.audit_log.action.report_reopened",
  report_note_added: "client:admin.audit_log.action.report_note_added",
  registration_approved: "client:admin.audit_log.action.registration_approved",
  registration_rejected: "client:admin.audit_log.action.registration_rejected",
  setting_changed: "client:admin.audit_log.action.setting_changed",
  text_changed: "client:admin.audit_log.action.text_changed",
  rule_added: "client:admin.audit_log.action.rule_added",
  rule_revised: "client:admin.audit_log.action.rule_revised",
  rule_moved: "client:admin.audit_log.action.rule_moved",
  rule_retired: "client:admin.audit_log.action.rule_retired",
  role_granted: "client:admin.audit_log.action.role_granted",
  role_revoked: "client:admin.audit_log.action.role_revoked",
  appeal_upheld: "client:admin.audit_log.action.appeal_upheld",
  appeal_overturned: "client:admin.audit_log.action.appeal_overturned",
  account_deletion_cancelled: "client:admin.audit_log.action.account_deletion_cancelled",
  signup_block_added: "client:admin.audit_log.action.signup_block_added",
  signup_block_lifted: "client:admin.audit_log.action.signup_block_lifted",
};

export const auditTargetKindLabels: Readonly<Record<AuditTargetKind, ClientMessageKey>> = {
  account: "client:admin.audit_log.target_kind.account",
  status: "client:admin.audit_log.target_kind.status",
  domain: "client:admin.audit_log.target_kind.domain",
  report: "client:admin.audit_log.target_kind.report",
  appeal: "client:admin.audit_log.target_kind.appeal",
  user: "client:admin.audit_log.target_kind.user",
  role: "client:admin.audit_log.target_kind.role",
  setting: "client:admin.audit_log.target_kind.setting",
  rule: "client:admin.audit_log.target_kind.rule",
  signup_block: "client:admin.audit_log.target_kind.signup_block",
};

/** Who performed an operation without a user's account to show. */
export const auditActorKindLabels: Readonly<Record<AuditActorKind, ClientMessageKey>> = {
  user: "client:admin.audit_log.actor.deleted_user",
  cli: "client:admin.audit_log.actor.cli",
  system: "client:admin.audit_log.actor.system",
  remote: "client:admin.audit_log.actor.remote",
};

/**
 * The admin screen of an operation's target, where it has one; an account, a report, a domain or a sign-up block.
 * The other kinds of target (settings, rules, registrations) have no screen of their own, and a target that is
 * gone (erased past its retention) has nothing left to show, so their entries show the label only.
 */
export function auditTargetHref(entry: Pick<AuditEntry, "target_kind" | "target_id" | "target_exists">): string | undefined {
  if (!entry.target_exists) return undefined;
  const id = encodeURIComponent(entry.target_id);
  switch (entry.target_kind) {
    case "account":
      return `/moderation/accounts/${id}`;
    case "report":
      return `/moderation/reports/${id}`;
    case "domain":
      // Every action on the domain, lifted ones too: the entry may be about one that is no longer in effect.
      return `/moderation/domain-actions?filter=all&domain=${id}`;
    case "signup_block":
      // The screen lists every block, lifted and ended ones too; a block has no screen of its own.
      return "/moderation/signup-blocks?filter=all";
    default:
      return undefined;
  }
}

export type AuditLog = PagedList<AuditEntry> & {
  /** Why the last page failed to load, localized; the list keeps what it has and "show more" tries again. */
  loadMoreError: string | undefined;
  /** The account of the actor of the filter, from the loaded entries; undefined when none shows it. */
  actorAccount: AuditEntry["actor_account"];
  /** The label of the target of the filter, from the loaded entries; undefined when none shows it. */
  targetLabel: string | undefined;
};

/**
 * The audit log under `filter`, newest first. `onPagesLoaded(pages)` is called with the number of loaded pages
 * after loading more, for the route to keep in the address.
 */
export function useAuditLog(filter: AuditLogFilter, onPagesLoaded: (pages: number) => void): AuditLog {
  const api = useApi();
  const query = useSuspenseInfiniteQuery(auditLogQueryOptions(api, filter));
  const items = query.data.pages.flatMap((page) => page.items);
  const actorEntry = filter.actor === undefined ? undefined : items.find((entry) => entry.actor_account_id === filter.actor && entry.actor_account !== undefined);
  const targetEntry = filter.target === undefined ? undefined : items.find((entry) => entry.target_kind === filter.targetKind && entry.target_id === filter.target);
  return {
    items,
    hasMore: query.hasNextPage,
    isLoadingMore: query.isFetchingNextPage,
    loadMore: () => {
      // A fetch in progress would be overwritten by the page fetched now (TanStack Query, "Infinite Queries").
      if (query.isFetching) return;
      void query.fetchNextPage().then((result) => {
        const pages = result.data?.pages.length;
        if (!result.isFetchNextPageError && pages !== undefined) onPagesLoaded(pages);
      });
    },
    loadMoreError: query.isFetchNextPageError ? errorMessage(query.error) : undefined,
    actorAccount: actorEntry?.actor_account,
    targetLabel: targetEntry?.target_label,
  };
}
