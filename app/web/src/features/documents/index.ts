// SPDX-License-Identifier: AGPL-3.0-only
// The documents feature (#489, #1037): the links to the pages of the server's terms of service, privacy policy and
// commerce disclosures, which administrators write on the admin screen (features/admin) and the information pages
// at /about/terms, /about/privacy and /about/commerce show. The shell's
// third column and menu, the settings, and the sign-in page show them; a document the server has not published
// has no link. Routes and other features use it through this file only (web-client/README.md A3).
export { DocumentLinks, DocumentsSection } from "./components/DocumentLinks";
export { documentQueryOptions, instanceQueryOptions, type DocumentKind } from "./api/documents";
export { DocumentLink } from "./components/DocumentLink";
export { DocumentView } from "./components/DocumentView";
