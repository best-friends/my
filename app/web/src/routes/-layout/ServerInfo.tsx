// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { useId } from "react";
import { DocumentLinks } from "../../features/documents";
import { useConfig } from "../../lib/config";
import { useT } from "../../lib/i18n";
import { aboutPath } from "../../lib/paths";
import { ServerNameLink } from "./ServerNameLink";

/**
 * In the third column on wide screens (xl): which server this is (its name links to the root address), a link to
 * its information page, and the links to its terms of service and privacy policy when it publishes them. Narrower
 * layouts leave it out; the menu and the settings have the links.
 */
export function ServerInfo() {
  const t = useT();
  // Handles end with the account domain, whatever the instance is named.
  const { accountDomain } = useConfig();
  const headingId = useId();
  return (
    <aside aria-labelledby={headingId} className="flex flex-col gap-2">
      <h2 id={headingId} className="text-sm font-semibold text-fg-muted">
        {t("client:shell.about.title")}
      </h2>
      <ServerNameLink className="text-lg align-match-parent" />
      <p className="text-sm text-fg-muted">{t("client:shell.about.domain", { domain: accountDomain })}</p>
      <p className="text-sm">
        <Link href={aboutPath} variant="standalone">
          {t("client:shell.about.more")}
        </Link>
      </p>
      <DocumentLinks />
    </aside>
  );
}
