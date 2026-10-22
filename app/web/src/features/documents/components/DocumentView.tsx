// SPDX-License-Identifier: AGPL-3.0-only
// A document of the instance as the body of an information page: the sanitized HTML the server renders, headed by
// the date of its revision.
import { PostContent, serverSanitizedHtml, Text } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useIntl } from "react-intl";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { type DocumentKind, documentQueryOptions } from "../api/documents";

/**
 * Gives each `h2` of the body that has no `id` one, so that the page's table of contents can link to it. Only an
 * attribute is added to the markup the server sanitized.
 */
function numberHeadings(body: HTMLDivElement | null) {
  body?.querySelectorAll("h2").forEach((heading, index) => {
    if (heading.id === "") heading.id = `document-section-${index + 1}`;
  });
}

/** The document of `kind`, which the route's loader has read; nothing while it is not there. */
export function DocumentView({ kind }: { kind: DocumentKind }) {
  const t = useT();
  const intl = useIntl();
  const api = useApi();
  const { data } = useQuery({ ...documentQueryOptions(api, kind), staleTime: Infinity });
  if (data === undefined) return null;
  return (
    <>
      <Text as="p" tone="muted" size="sm">
        {t("client:documents.revised", { date: intl.formatDate(data.revised_at, { dateStyle: "long" }) })}
      </Text>
      <PostContent ref={numberHeadings} html={serverSanitizedHtml(data.html)} lang="" />
    </>
  );
}
