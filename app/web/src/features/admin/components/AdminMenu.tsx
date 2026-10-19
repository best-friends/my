// SPDX-License-Identifier: AGPL-3.0-only
// The links to the admin screens, each shown only to a viewer whose roles have its permission: a section of
// the settings page (a viewer with none of them sees no section at all), and the administration page that
// the navigation's admin entry opens. The screens with things awaiting the moderators (open reports,
// registrations awaiting approval; FR-712) show the count as a badge whose words are in the link's name, so
// the count is read with the screen and shown by its digits, not by color alone.
import { Badge, EmptyState, Link } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { Permission } from "../api/admin";
import type { AdminCount } from "../api/counts";
import { useAdminCounts } from "../hooks/useAdminCounts";
import { useViewerPermissions } from "../hooks/usePermissions";

/** The admin screens, in the order of the menu, with the permission each needs. */
const pages: readonly { href: string; permission: Permission; label: ClientMessageKey; description: ClientMessageKey }[] = [
  { href: "/admin/instances", permission: "manage_domains", label: "client:admin.instances.title", description: "client:admin.instances.summary" },
  { href: "/admin/domain-actions", permission: "manage_domains", label: "client:admin.domain_actions.title", description: "client:admin.domain_actions.summary" },
  { href: "/admin/dead-jobs", permission: "manage_jobs", label: "client:admin.dead_jobs.title", description: "client:admin.dead_jobs.summary" },
  { href: "/admin/audit-log", permission: "view_audit", label: "client:admin.audit_log.title", description: "client:admin.audit_log.summary" },
  { href: "/admin/registrations", permission: "manage_accounts", label: "client:admin.registrations.title", description: "client:admin.registrations.summary" },
  { href: "/admin/reports", permission: "manage_reports", label: "client:admin.reports.title", description: "client:admin.reports.summary" },
];

/** The count that a screen's link shows, by the screen's address, and the words of its badge. */
const pageCounts: Readonly<
  Record<string, { count: AdminCount; label: "client:admin.menu.open_reports" | "client:admin.menu.pending_registrations" | "client:admin.menu.pending_appeals" }>
> = {
  "/admin/reports": { count: "open_reports", label: "client:admin.menu.open_reports" },
  "/admin/registrations": { count: "pending_registrations", label: "client:admin.menu.pending_registrations" },
  "/admin/appeals": { count: "pending_appeals", label: "client:admin.menu.pending_appeals" },
};

/** The admin screens that the viewer's roles allow. */
function useShownPages() {
  const permissions = useViewerPermissions();
  return pages.filter((page) => permissions.includes(page.permission));
}

/** The list of the links to the screens in `shown`, each with its description and its count. */
function AdminPageList({ shown, idPrefix }: { shown: typeof pages; idPrefix: string }) {
  const t = useT();
  const counts = useAdminCounts();
  return (
    <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
      {shown.map((page) => {
        const descriptionId = `${idPrefix}-${page.href}`;
        const counted = pageCounts[page.href];
        const count = counted === undefined ? 0 : (counts[counted.count] ?? 0);
        return (
          <li key={page.href} className="flex flex-col px-4 py-3">
            <Link href={page.href} variant="standalone" aria-describedby={descriptionId} className="gap-2">
              {t(page.label)}
              {counted !== undefined && count > 0 && (
                <>
                  {/* The space keeps the words apart in the link's name. */}{" "}
                  <Badge count={count} label={t(counted.label, { count })} />
                </>
              )}
            </Link>
            <span id={descriptionId} className="text-sm text-fg-muted">
              {t(page.description)}
            </span>
          </li>
        );
      })}
    </ul>
  );
}

/** The settings page's section of links to the admin screens; nothing for a viewer without any. */
export function AdminMenu() {
  const t = useT();
  const headingId = useId();
  const shown = useShownPages();
  if (shown.length === 0) return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:admin.menu.title")}
      </h2>
      <AdminPageList shown={shown} idPrefix={headingId} />
    </section>
  );
}

/** The administration page's links to the admin screens, or why there are none for the viewer's roles. */
export function AdminPages() {
  const t = useT();
  const id = useId();
  const shown = useShownPages();
  if (shown.length === 0) return <EmptyState heading={t("client:admin.home.none.heading")} description={t("client:admin.home.none.description")} />;
  return <AdminPageList shown={shown} idPrefix={id} />;
}
