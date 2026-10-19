// SPDX-License-Identifier: AGPL-3.0-only
import { useId } from "react";
import { DocumentLinks } from "../../features/documents";
import { useConfig } from "../../lib/config";
import { useT } from "../../lib/i18n";
import { useInstanceName } from "../../lib/instanceName";

/**
 * In the third column on wide screens (xl): which server this is, and the links to its terms of service and
 * privacy policy when it publishes them. Narrower layouts leave it out; the menu and the settings have the links.
 */
export function ServerInfo() {
  const t = useT();
  const instanceName = useInstanceName();
  // Handles end with the account domain, whatever the instance is named.
  const { accountDomain } = useConfig();
  const headingId = useId();
  return (
    <aside aria-labelledby={headingId} className="flex flex-col gap-2">
      <h2 id={headingId} className="text-sm font-semibold text-fg-muted">
        {t("client:shell.about.title")}
      </h2>
      {/* The name is the administrator's text, in any language (web-client.md Y10). */}
      <p dir="auto" className="text-lg font-semibold align-match-parent">
        {instanceName}
      </p>
      <p className="text-sm text-fg-muted">{t("client:shell.about.domain", { domain: accountDomain })}</p>
      <DocumentLinks />
    </aside>
  );
}
