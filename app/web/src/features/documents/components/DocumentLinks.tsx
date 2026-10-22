// SPDX-License-Identifier: AGPL-3.0-only
// The links to the server's terms of service and privacy policy. Their pages are the server's own (rendered by
// the server, readable without signing in and without JavaScript) and outside the app, so a link opens them in a
// new tab (ExternalLink, web-client.md Y12).
import { Box, Cluster, ExternalLink, Heading, Stack } from "@my/ui";
import { useId } from "react";
import { useT } from "../../../lib/i18n";
import { useDocumentLinks } from "../hooks/useDocumentLinks";


type DocumentLinksProps = {
  /** The id of a heading that names the links; without it they are named "Terms and policies". */
  labelledBy?: string;
  /** Where the links are: among other content, or under the items of the navigation's menu sheet, set apart by a line. */
  placement?: "inline" | "sheet";
};

/** The links to the published documents, in a row that wraps; nothing when the server publishes none. */
export function DocumentLinks({ labelledBy, placement = "inline" }: DocumentLinksProps) {
  const t = useT();
  const links = useDocumentLinks();
  if (links.length === 0) return null;
  return (
    <Box
      as="nav"
      {...(labelledBy === undefined ? { "aria-label": t("client:documents.label") } : { "aria-labelledby": labelledBy })}
      {...(placement === "sheet" ? { marginTop: "3", border: "top", paddingX: "3", paddingTop: "3" } as const : {})}
    >
      <Cluster as="ul" gap="4" rowGap="1" textSize="sm">
        {links.map((link) => (
          <li key={link.href}>
            <ExternalLink href={link.href} variant="standalone">
              {link.label}
            </ExternalLink>
          </li>
        ))}
      </Cluster>
    </Box>
  );
}

/** The section of the links to the published documents (settings, the information page, the landing page); nothing when the server publishes none. */
export function DocumentsSection() {
  const t = useT();
  const headingId = useId();
  const links = useDocumentLinks();
  if (links.length === 0) return null;
  return (
    <Stack as="section" aria-labelledby={headingId} gap="3">
      <Heading level={2} id={headingId}>
        {t("client:documents.label")}
      </Heading>
      <DocumentLinks labelledBy={headingId} />
    </Stack>
  );
}
