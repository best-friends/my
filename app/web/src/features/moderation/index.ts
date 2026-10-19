// SPDX-License-Identifier: AGPL-3.0-only
// The moderation feature on the user's side: the account status page, with the moderation actions on the
// user's own account and appealing them (FR-715, FR-707). The moderators' screens are in the admin feature.
// Routes and other features use it through this file only (web-client.md A3).
export { ownActionsQueryOptions } from "./api/actions";
export { AccountStatus } from "./components/AccountStatus";
