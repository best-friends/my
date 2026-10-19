// SPDX-License-Identifier: AGPL-3.0-only
// The links to the admin screens on the settings page, each shown only to a viewer whose roles have its
// permission; a viewer with none of them sees no section at all.
import { Link } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { Permission } from "../api/admin";
import { useViewerPermissions } from "../hooks/usePermissions";

/** The admin screens, in the order of the menu, with the permission each needs. */
const pages: readonly { href: string; permission: Permission; label: ClientMessageKey; description: ClientMessageKey }[] = [
  { href: "/admin/instances", permission: "manage_domains", label: "client:admin.instances.title", description: "client:admin.instances.summary" },
  { href: "/admin/domain-actions", permission: "manage_domains", label: "client:admin.domain_actions.title", description: "client:admin.domain_actions.summary" },
  { href: "/admin/dead-jobs", permission: "manage_jobs", label: "client:admin.dead_jobs.title", description: "client:admin.dead_jobs.summary" },
  { href: "/admin/audit-log", permission: "view_audit", label: "client:admin.audit_log.title", description: "client:admin.audit_log.summary" },
  { href: "/admin/registrations", permission: "manage_accounts", label: "client:admin.registrations.title", description: "client:admin.registrations.summary" },
];

export function AdminMenu() {
  const t = useT();
  const headingId = useId();
  const permissions = useViewerPermissions();
  const shown = pages.filter((page) => permissions.includes(page.permission));
  if (shown.length === 0) return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:admin.menu.title")}
      </h2>
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
        {shown.map((page) => {
          const descriptionId = `${headingId}-${page.href}`;
          return (
            <li key={page.href} className="flex flex-col px-4 py-3">
              <Link href={page.href} variant="standalone" aria-describedby={descriptionId}>
                {t(page.label)}
              </Link>
              <span id={descriptionId} className="text-sm text-fg-muted">
                {t(page.description)}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
