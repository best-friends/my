// SPDX-License-Identifier: AGPL-3.0-only
// The documents feature (#489): the links to the server's public pages of its terms of service and privacy
// policy, which administrators write on the admin screen (features/admin) and the server renders. The shell's
// third column and menu, the settings, and the sign-in page show them; a document the server has not published
// has no link. Routes and other features use it through this file only (web-client.md A3).
export { DocumentLinks, DocumentsSection } from "./components/DocumentLinks";
