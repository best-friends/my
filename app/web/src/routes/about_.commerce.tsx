// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { DocumentError, DocumentPage, documentLoader } from "./-layout/DocumentPage";

/**
 * The instance's commerce disclosure as an information page (ADR-0072 §13), for everyone, signed in or not; a 404 page when the
 * instance has not published it. The `_` keeps the page out of the layout of `/about`. The server's shell of this
 * address carries the link preview (ADR-0028).
 */
export const Route = createFileRoute("/about_/commerce")({
  loader: documentLoader("commerce_disclosure"),
  component: () => <DocumentPage kind="commerce_disclosure" />,
  errorComponent: (props: ErrorComponentProps) => <DocumentError {...props} kind="commerce_disclosure" />,
});
