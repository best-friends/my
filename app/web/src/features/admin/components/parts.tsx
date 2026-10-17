// SPDX-License-Identifier: AGPL-3.0-only
// Pieces that the admin screens share, so that every list and every detail page of the admin area has the
// same shape: the permission gate and its message, the paged list of records (rows with dividers, the empty
// state and "show more"), the titled section of a detail page, a server's state in words, and dates in the
// viewer's locale and time zone. The facts of a record are @my/ui's DescriptionList.
import { Button, EmptyState } from "@my/ui";
import { type ReactNode, type Ref, useId } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { InstanceState, Permission } from "../api/admin";
import type { PagedList } from "../hooks/pagedList";
import { useHasPermission } from "../hooks/usePermissions";

const stateLabels: Readonly<Record<InstanceState, ClientMessageKey>> = {
  available: "client:admin.instance.state.available",
  unavailable: "client:admin.instance.state.unavailable",
};

/** A server's state in words, not by color alone (WCAG 1.4.1): the color only repeats it. */
export function InstanceStateLabel({ state }: { state: InstanceState }) {
  const t = useT();
  return <span className={state === "unavailable" ? "font-semibold text-danger" : "font-semibold text-success"}>{t(stateLabels[state])}</span>;
}

/** A moment in the viewer's locale and time zone, as a `<time>`. */
export function DateTime({ value }: { value: string }) {
  const intl = useIntl();
  return <time dateTime={value}>{intl.formatDate(value, { dateStyle: "medium", timeStyle: "short" })}</time>;
}

/** A UTC calendar day (`YYYY-MM-DD`) as a date in the viewer's locale; it is the same day in every time zone. */
export function Day({ value }: { value: string }) {
  const intl = useIntl();
  return <time dateTime={value}>{intl.formatDate(value, { dateStyle: "medium", timeZone: "UTC" })}</time>;
}

/** The button under a list that loads its next page, while there is one. */
function MoreButton<T>({ list }: { list: PagedList<T> }) {
  const t = useT();
  if (!list.hasMore) return null;
  return (
    <div>
      <Button onPress={list.loadMore} isPending={list.isLoadingMore}>
        {t("client:admin.list.more")}
      </Button>
    </div>
  );
}

/** What an admin screen shows instead of its content to a viewer whose roles lack `permission`. */
export function MissingPermission({ permission }: { permission: Permission }) {
  const t = useT();
  return <EmptyState heading={t("client:admin.missing_permission.heading")} description={t("client:admin.missing_permission.description", { permission })} />;
}

/**
 * Shows `children` only to a viewer whose roles have `permission`, and the reason to the others. The route
 * reads nothing for them either (routes/-admin/adminRoute.tsx); the server checks every request.
 */
export function RequirePermission({ permission, children }: { permission: Permission; children: ReactNode }) {
  const allowed = useHasPermission(permission);
  if (!allowed) return <MissingPermission permission={permission} />;
  return children;
}

export type RecordListProps<T> = {
  /** The records loaded so far and loading more. */
  list: PagedList<T>;
  /** A stable id of a record: the row's key and its `data-row-id` (lib/useFocusAfterRemoval.ts). */
  getId: (item: T) => string;
  /** One row: a heading (`h2`, or one below the region's heading) and its facts. */
  children: (item: T) => ReactNode;
  /** What an empty list shows, an `EmptyState`. */
  empty: ReactNode;
  /** The `<ul>`, for lists whose rows go away (useFocusAfterRemoval). */
  listRef?: Ref<HTMLUListElement>;
};

/**
 * A list of records of an admin screen (web-client.md G3): one row per record, divided by lines, each row a
 * size container for its facts, then "show more" while there are more. An empty list shows `empty` instead.
 */
export function RecordList<T>({ list, getId, children, empty, listRef }: RecordListProps<T>) {
  if (list.items.length === 0) return empty;
  return (
    <div className="flex flex-col gap-4">
      <ul ref={listRef} className="flex flex-col divide-y divide-border">
        {list.items.map((item) => {
          const id = getId(item);
          return (
            <li key={id} data-row-id={id} className="@container py-3">
              {children(item)}
            </li>
          );
        })}
      </ul>
      <MoreButton list={list} />
    </div>
  );
}

/** A titled region of a detail page below its `h1`: an `h2` that names a `<section>`. */
export function DetailSection({ title, children }: { title: ReactNode; children: ReactNode }) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}
