// SPDX-License-Identifier: AGPL-3.0-only
import type { ErrorComponentProps } from "@tanstack/react-router";
import { type DocumentKind, documentQueryOptions, DocumentView, instanceQueryOptions } from "../../features/documents";
import type { RouterContext } from "../../lib/query";
import { type PlainClientMessageKey, useT } from "../../lib/i18n";
import { InfoLayout } from "./InfoLayout";
import { PageHeading } from "./PageHeading";
import { RouteError } from "./RouteStates";

/** The name of each document, which is the page's heading. */
const documentTitles: Readonly<Record<DocumentKind, PlainClientMessageKey>> = {
  terms: "client:documents.terms",
  privacy_policy: "client:documents.privacy_policy",
  commerce_disclosure: "client:documents.commerce_disclosure",
};

/**
 * The loader of a document's page: the document, from the cache at once when it is there (web-client/README.md S7). A
 * document the instance has not published (404) fails the page, which shows the not-found screen.
 */
export function documentLoader(kind: DocumentKind) {
  return async ({ context: { queryClient, api } }: { context: RouterContext }) => {
    await Promise.all([
      // The shell reads the instance too and handles its failure.
      queryClient.query({ ...instanceQueryOptions(api), staleTime: Infinity }).catch(() => undefined),
      queryClient.query({ ...documentQueryOptions(api, kind), staleTime: Infinity }),
    ]);
  };
}

/** A document of the instance as an information page (ADR-0072 §13), for everyone, signed in or not. */
export function DocumentPage({ kind }: { kind: DocumentKind }) {
  return (
    <InfoLayout heading={<DocumentHeading kind={kind} />}>
      <DocumentView kind={kind} />
    </InfoLayout>
  );
}

function DocumentHeading({ kind }: { kind: DocumentKind }) {
  const t = useT();
  return <PageHeading>{t(documentTitles[kind])}</PageHeading>;
}

/** The error of a document's page, under the document's name. */
export function DocumentError({ kind, ...props }: ErrorComponentProps & { kind: DocumentKind }) {
  const t = useT();
  return <RouteError {...props} heading={t(documentTitles[kind])} />;
}
