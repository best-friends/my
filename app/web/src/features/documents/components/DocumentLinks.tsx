// SPDX-License-Identifier: AGPL-3.0-only
// The links to the server's terms of service and privacy policy. Their pages are the server's own (rendered by
// the server, readable without signing in and without JavaScript), so a link loads them in place of the app
// (lib/paths.ts isServerPath).
import { Link } from "@my/ui";
import { useId } from "react";
import { useT } from "../../../lib/i18n";
import { useDocumentLinks } from "../hooks/useDocumentLinks";

/** Where the links are: among other content, or under the items of the navigation's menu sheet, set apart by a line. */
const placements = {
  inline: undefined,
  sheet: "mt-3 border-t border-border px-3 pt-3",
} as const;

type DocumentLinksProps = {
  /** The id of a heading that names the links; without it they are named "Terms and policies". */
  labelledBy?: string;
  placement?: keyof typeof placements;
};

/** The links to the published documents, in a row that wraps; nothing when the server publishes none. */
export function DocumentLinks({ labelledBy, placement = "inline" }: DocumentLinksProps) {
  const t = useT();
  const links = useDocumentLinks();
  if (links.length === 0) return null;
  return (
    <nav
      {...(labelledBy === undefined ? { "aria-label": t("client:documents.label") } : { "aria-labelledby": labelledBy })}
      className={placements[placement]}
    >
      <ul className="flex flex-wrap gap-x-4 gap-y-1 text-sm">
        {links.map((link) => (
          <li key={link.href}>
            <Link href={link.href} variant="standalone">
              {link.label}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}

/** The settings page's section of the links to the published documents; nothing when the server publishes none. */
export function DocumentsSection() {
  const t = useT();
  const headingId = useId();
  const links = useDocumentLinks();
  if (links.length === 0) return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:documents.label")}
      </h2>
      <DocumentLinks labelledBy={headingId} />
    </section>
  );
}
