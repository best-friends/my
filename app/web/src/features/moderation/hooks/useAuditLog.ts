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
  role_created: "client:admin.audit_log.action.role_created",
  role_updated: "client:admin.audit_log.action.role_updated",
  role_deleted: "client:admin.audit_log.action.role_deleted",
  appeal_upheld: "client:admin.audit_log.action.appeal_upheld",
  appeal_overturned: "client:admin.audit_log.action.appeal_overturned",
  account_deletion_cancelled: "client:admin.audit_log.action.account_deletion_cancelled",
  signup_block_added: "client:admin.audit_log.action.signup_block_added",
  signup_block_lifted: "client:admin.audit_log.action.signup_block_lifted",
  custom_emoji_created: "client:admin.audit_log.action.custom_emoji_created",
  custom_emoji_image_replaced: "client:admin.audit_log.action.custom_emoji_image_replaced",
  custom_emoji_updated: "client:admin.audit_log.action.custom_emoji_updated",
  custom_emoji_enabled: "client:admin.audit_log.action.custom_emoji_enabled",
  custom_emoji_disabled: "client:admin.audit_log.action.custom_emoji_disabled",
  custom_emoji_deleted: "client:admin.audit_log.action.custom_emoji_deleted",
  webhook_created: "client:admin.audit_log.action.webhook_created",
  webhook_updated: "client:admin.audit_log.action.webhook_updated",
  webhook_deleted: "client:admin.audit_log.action.webhook_deleted",
  webhook_enabled: "client:admin.audit_log.action.webhook_enabled",
  webhook_disabled: "client:admin.audit_log.action.webhook_disabled",
  webhook_secret_rotated: "client:admin.audit_log.action.webhook_secret_rotated",
  announcement_created: "client:admin.audit_log.action.announcement_created",
  announcement_updated: "client:admin.audit_log.action.announcement_updated",
  announcement_published: "client:admin.audit_log.action.announcement_published",
  announcement_withdrawn: "client:admin.audit_log.action.announcement_withdrawn",
  invite_created: "client:admin.audit_log.action.invite_created",
  invite_revoked: "client:admin.audit_log.action.invite_revoked",
  relay_added: "client:admin.audit_log.action.relay_added",
  relay_followed: "client:admin.audit_log.action.relay_followed",
  relay_unfollowed: "client:admin.audit_log.action.relay_unfollowed",
  relay_removed: "client:admin.audit_log.action.relay_removed",
  relay_accepted: "client:admin.audit_log.action.relay_accepted",
  relay_rejected: "client:admin.audit_log.action.relay_rejected",
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
  custom_emoji: "client:admin.audit_log.target_kind.custom_emoji",
  webhook: "client:admin.audit_log.target_kind.webhook",
  announcement: "client:admin.audit_log.target_kind.announcement",
  invite: "client:admin.audit_log.target_kind.invite",
  relay: "client:admin.audit_log.target_kind.relay",
};

/** Who performed an operation without a user's account to show. */
export const auditActorKindLabels: Readonly<Record<AuditActorKind, ClientMessageKey>> = {
  user: "client:admin.audit_log.actor.deleted_user",
  cli: "client:admin.audit_log.actor.cli",
  system: "client:admin.audit_log.actor.system",
  remote: "client:admin.audit_log.actor.remote",
};

/**
 * The admin screen of an operation's target, where it has one; an account, a report, a domain, a sign-up block, a
 * role, a custom emoji, a webhook, an announcement, an invite or a relay.
 * The other kinds of target (settings, rules, registrations) have no screen of their own, and a target that is
 * gone (erased past its retention, or deleted) has nothing left to show, so their entries show the label only.
 */
export function auditTargetHref(entry: Pick<AuditEntry, "target_kind" | "target_id" | "target_label" | "target_exists">): string | undefined {
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
    case "custom_emoji":
      // An emoji has no screen of its own: the list searched by its shortcode, of this server or of its domain.
      return customEmojiHref(entry.target_label);
    case "role":
      // A role has no screen of its own: the list of roles shows it.
      return "/admin/roles";
    case "webhook":
      return `/admin/webhooks/${id}`;
    case "announcement":
      // An announcement has no screen of its own: the list, which keeps every announcement.
      return "/admin/announcements";
    case "invite":
      // An invite has no screen of its own: the list of invites shows it.
      return "/moderation/invites";
    case "relay":
      // The list, which a deleted relay has left; a relay has no screen of its own.
      return "/admin/relays";
    default:
      return undefined;
  }
}

/** The custom emoji screen searched by the emoji of a label, `:shortcode:` or `:shortcode:@domain`; undefined for another label. */
function customEmojiHref(label: string): string | undefined {
  const match = /^:([A-Za-z0-9_]+):(?:@(.+))?$/u.exec(label);
  if (match === null) return undefined;
  const [, shortcode = "", domain] = match;
  const search = new URLSearchParams(domain === undefined ? { shortcode } : { origin: "remote", shortcode, domain });
  return `/admin/custom-emojis?${search.toString()}`;
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
