// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";

/** The pages of the settings, in the order of the menu. */
const pages: readonly { href: string; label: ClientMessageKey; description: ClientMessageKey }[] = [
  { href: "/settings/profile", label: "client:settings.profile.title", description: "client:settings.profile.summary" },
  { href: "/settings/preferences", label: "client:settings.preferences.title", description: "client:settings.preferences.summary" },
  { href: "/settings/two-factor", label: "client:settings.two_factor.title", description: "client:settings.two_factor.summary" },
  { href: "/settings/auto-delete", label: "client:settings.auto_delete.title", description: "client:settings.auto_delete.summary" },
  { href: "/settings/follow-requests", label: "client:settings.follow_requests.title", description: "client:settings.follow_requests.summary" },
  { href: "/settings/blocks", label: "client:settings.blocks.title", description: "client:settings.blocks.summary" },
  { href: "/settings/mutes", label: "client:settings.mutes.title", description: "client:settings.mutes.summary" },
  { href: "/settings/domain-mutes", label: "client:settings.domain_mutes.title", description: "client:settings.domain_mutes.summary" },
  { href: "/settings/import-export", label: "client:settings.import_export.title", description: "client:settings.import_export.summary" },
  { href: "/settings/account-status", label: "client:account_status.title", description: "client:account_status.summary" },
  { href: "/settings/account", label: "client:settings.account.title", description: "client:settings.account.summary" },
];

/** The links to the pages of the settings, each with what it is about. */
export function SettingsMenu() {
  const t = useT();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:settings.menu.title")}
      </h2>
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
        {pages.map((page) => {
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
