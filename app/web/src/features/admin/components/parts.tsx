// SPDX-License-Identifier: AGPL-3.0-only
// Pieces that the admin screens share: a server's state in words, dates in the viewer's locale and time zone,
// the "show more" button of the lists, and the message for a viewer without the permission.
import { Button, EmptyState } from "@my/ui";
import type { ReactNode } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { InstanceState, Permission } from "../api/admin";
import type { PagedList } from "../hooks/pagedList";

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

/** One term and its value of a description list. */
export function Detail({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col">
      <dt className="text-sm text-fg-muted">{term}</dt>
      <dd className="min-w-0 wrap-anywhere">{children}</dd>
    </div>
  );
}

/** The button under a list that loads its next page, while there is one. */
export function MoreButton<T>({ list }: { list: PagedList<T> }) {
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
