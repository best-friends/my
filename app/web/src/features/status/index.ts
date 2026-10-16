// SPDX-License-Identifier: AGPL-3.0-only
// The status feature: a post's card with the viewer's actions on it (boost, favourite, bookmark, delete,
// copy the link, reply), and a post's page with its thread (FR-106 to FR-108, FR-113). Routes and other
// features use it through this file only (web-client.md A3).
export { statusContextQueryOptions, statusPath, statusQueryOptions } from "./api/status";
export { StatusCard } from "./components/StatusCard";
export { StatusThread } from "./components/StatusThread";
