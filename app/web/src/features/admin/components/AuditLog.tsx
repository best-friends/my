// SPDX-License-Identifier: AGPL-3.0-only
// The audit log screen below its heading (FR-704): the filters, then the operations of the moderators and
// administrators, newest first. The kind of operation and the kind of target are selects (React Aria Select, the
// WAI-ARIA APG select-only combobox); one actor and one target are chosen from a row's links. The route keeps the
// filters and the number of loaded pages in the address (web-client.md S3). Each row links to its target's admin
// screen where that kind of target has one, and shows the label recorded with the operation otherwise.
import { DescriptionItem, DescriptionList, EmptyState, Link, RegionError, Select, SelectItem } from "@my/ui";
import { type RefObject, useEffect, useRef } from "react";
import { useT } from "../../../lib/i18n";
import { type AuditAction, auditActions, type AuditEntry, type AuditLogFilter, type AuditTargetKind, auditTargetKinds } from "../api/admin";
import { auditActionLabels, auditActorKindLabels, auditTargetHref, auditTargetKindLabels, useAuditLog } from "../hooks/useAuditLog";
import { DateTime, DetailSection, DomainName, RecordList, RequirePermission } from "./parts";

/** The option of a select that keeps every kind. */
const every = "all";

export type AuditLogProps = {
  /** The filter, from the URL. */
  filter: AuditLogFilter;
  /** Chooses another filter: the route puts it into the URL. */
  onFilterChange: (filter: AuditLogFilter) => void;
  /** The address of the list under `filter`, from its first page. */
  filterHref: (filter: AuditLogFilter) => string;
  /** The list has loaded more: the route keeps the number of its pages in the URL. */
  onPagesLoaded: (pages: number) => void;
};

export function AuditLog(props: AuditLogProps) {
  return (
    <RequirePermission permission="view_audit">
      <AuditLogView {...props} />
    </RequirePermission>
  );
}

function isAction(key: unknown): key is AuditAction {
  return auditActions.some((action) => action === key);
}

function isTargetKind(key: unknown): key is AuditTargetKind {
  return auditTargetKinds.some((kind) => kind === key);
}

function hasFilter(filter: AuditLogFilter): boolean {
  return filter.actor !== undefined || filter.action !== undefined || filter.targetKind !== undefined;
}

/** Text written by people inside a sentence, isolated from it in its own direction (U+2068 FIRST STRONG ISOLATE … U+2069). */
function isolate(text: string): string {
  return `\u2068${text}\u2069`;
}

/** The name of an account for a sentence or a row: its display name, or its handle when it has none. */
function accountName(account: NonNullable<AuditEntry["actor_account"]>): string {
  return account.display_name === "" ? handleOf(account) : account.display_name;
}

/** An account's handle as this server knows it: `@alice`, or `@bob@example.com` for a remote one. */
function handleOf(account: NonNullable<AuditEntry["actor_account"]>): string {
  return `@${account.acct}`;
}

/**
 * Keeps focus in the filters when a link that lets go of a filter goes away with it: once the filters have
 * rendered without the pressed link, focus moves to their first control. Returns `pressed()`, for the links' onPress.
 */
function useFocusAfterClearing(filters: RefObject<HTMLElement | null>): () => void {
  const pendingRef = useRef(false);
  // Runs after every render: focus is moved in the DOM, outside React, once the link is gone.
  useEffect(() => {
    const container = filters.current;
    if (!pendingRef.current || container === null || container.contains(document.activeElement)) return;
    pendingRef.current = false;
    container.querySelector<HTMLElement>("button")?.focus();
  });
  return () => {
    pendingRef.current = true;
  };
}

function AuditLogView({ filter, onFilterChange, filterHref, onPagesLoaded }: AuditLogProps) {
  const t = useT();
  const log = useAuditLog(filter, onPagesLoaded);
  const filtered = hasFilter(filter);
  const filtersRef = useRef<HTMLDivElement>(null);
  const cleared = useFocusAfterClearing(filtersRef);
  const clearAll = (
    <Link href={filterHref({})} variant="standalone" onPress={cleared}>
      {t("client:admin.audit_log.filter.clear")}
    </Link>
  );
  const empty = filtered ? (
    <EmptyState headingLevel={3} heading={t("client:admin.audit_log.empty.filtered")} description={t("client:admin.audit_log.empty.filtered.description")} action={clearAll} />
  ) : (
    <EmptyState headingLevel={3} heading={t("client:admin.audit_log.empty")} description={t("client:admin.audit_log.empty.description")} />
  );
  return (
    <div className="flex flex-col gap-8">
      <DetailSection title={t("client:admin.audit_log.filter.title")}>
        <div ref={filtersRef} className="@container flex flex-col gap-4">
          <div className="grid grid-cols-1 gap-4 @md:grid-cols-2">
            <Select
              label={t("client:admin.audit_log.filter.action")}
              value={filter.action ?? every}
              onChange={(key) => {
                onFilterChange({ ...filter, action: isAction(key) ? key : undefined });
              }}
            >
              <SelectItem id={every}>{t("client:admin.audit_log.filter.action.all")}</SelectItem>
              {auditActions.map((action) => (
                <SelectItem key={action} id={action}>
                  {t(auditActionLabels[action])}
                </SelectItem>
              ))}
            </Select>
            <Select
              label={t("client:admin.audit_log.filter.target_kind")}
              value={filter.targetKind ?? every}
              onChange={(key) => {
                // One target is of one kind: another kind lets go of it.
                onFilterChange({ ...filter, targetKind: isTargetKind(key) ? key : undefined, target: undefined });
              }}
            >
              <SelectItem id={every}>{t("client:admin.audit_log.filter.target_kind.all")}</SelectItem>
              {auditTargetKinds.map((kind) => (
                <SelectItem key={kind} id={kind}>
                  {t(auditTargetKindLabels[kind])}
                </SelectItem>
              ))}
            </Select>
          </div>
          {(filter.actor !== undefined || (filter.targetKind !== undefined && filter.target !== undefined)) && (
            <DescriptionList columns={2}>
              {filter.actor !== undefined && (
                <DescriptionItem term={t("client:admin.audit_log.filter.actor")}>
                  <span className="flex flex-wrap items-baseline gap-x-3">
                    <span dir="auto">{log.actorAccount === undefined ? t("client:admin.audit_log.filter.actor.unknown") : accountName(log.actorAccount)}</span>
                    <Link href={filterHref({ ...filter, actor: undefined })} variant="standalone" onPress={cleared}>
                      {t("client:admin.audit_log.filter.actor.clear")}
                    </Link>
                  </span>
                </DescriptionItem>
              )}
              {filter.targetKind !== undefined && filter.target !== undefined && (
                <DescriptionItem term={t("client:admin.audit_log.filter.target")}>
                  <span className="flex flex-wrap items-baseline gap-x-3">
                    <span dir="auto">{log.targetLabel ?? filter.target}</span>
                    <Link href={filterHref({ ...filter, target: undefined })} variant="standalone" onPress={cleared}>
                      {t("client:admin.audit_log.filter.target.clear")}
                    </Link>
                  </span>
                </DescriptionItem>
              )}
            </DescriptionList>
          )}
          {filtered && <div>{clearAll}</div>}
        </div>
      </DetailSection>
      <DetailSection title={t("client:admin.audit_log.list.title")}>
        <RecordList
          list={log}
          getId={(entry) => entry.id}
          empty={empty}
          moreError={
            log.loadMoreError !== undefined && <RegionError heading={t("client:admin.audit_log.more_failed")} reason={log.loadMoreError} />
          }
        >
          {(entry) => <AuditEntryRow entry={entry} filter={filter} filterHref={filterHref} />}
        </RecordList>
      </DetailSection>
    </div>
  );
}

type AuditEntryRowProps = { entry: AuditEntry; filter: AuditLogFilter; filterHref: (filter: AuditLogFilter) => string };

function AuditEntryRow({ entry, filter, filterHref }: AuditEntryRowProps) {
  const t = useT();
  const actor = entry.actor_account;
  const actorName = actor === undefined ? t(auditActorKindLabels[entry.actor_kind]) : accountName(actor);
  const actorId = entry.actor_account_id;
  const isActorFilter = actorId !== undefined && actorId === filter.actor;
  const isTargetFilter = filter.targetKind === entry.target_kind && filter.target === entry.target_id;
  return (
    <div className="flex flex-col gap-2">
      <h3 className="font-semibold">{t(auditActionLabels[entry.action])}</h3>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.audit_log.actor")}>
          {actor === undefined ? (
            actorName
          ) : (
            <span className="flex flex-col">
              {/* A name the user chose, in any language. */}
              <span dir="auto" className="text-start">
                {actorName}
              </span>
              <span dir="ltr" className="text-start text-sm text-fg-muted">{handleOf(actor)}</span>
            </span>
          )}
        </DescriptionItem>
        <DescriptionItem term={t(auditTargetKindLabels[entry.target_kind])}>
          <AuditTarget entry={entry} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.audit_log.occurred_at")}>
          <DateTime value={entry.occurred_at} />
        </DescriptionItem>
      </DescriptionList>
      {/*
        The row stays in the list under the filter its links choose, and so do the links, which then let go of it:
        focus stays on the link that was pressed.
      */}
      <ul className="flex flex-wrap gap-x-4">
        {actorId !== undefined && (
          <li>
            <Link href={filterHref({ ...filter, actor: isActorFilter ? undefined : actorId })} variant="standalone" className="wrap-anywhere">
              {isActorFilter ? t("client:admin.audit_log.every_actor") : t("client:admin.audit_log.only_actor", { name: isolate(actorName) })}
            </Link>
          </li>
        )}
        <li>
          <Link
            href={filterHref(isTargetFilter ? { ...filter, target: undefined } : { ...filter, targetKind: entry.target_kind, target: entry.target_id })}
            variant="standalone"
            className="wrap-anywhere"
          >
            {isTargetFilter ? t("client:admin.audit_log.every_target") : t("client:admin.audit_log.only_target", { target: isolate(entry.target_label) })}
          </Link>
        </li>
      </ul>
    </div>
  );
}

/** The target as it was named when the operation happened, linking to its admin screen where it has one. */
function AuditTarget({ entry }: { entry: AuditEntry }) {
  const href = auditTargetHref(entry);
  const label = entry.target_kind === "domain" ? <DomainName domain={entry.target_id} /> : <span dir="auto">{entry.target_label}</span>;
  if (href === undefined) return label;
  return (
    <Link href={href} variant="inline" className="wrap-anywhere">
      {label}
    </Link>
  );
}
