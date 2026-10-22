// SPDX-License-Identifier: AGPL-3.0-only
// The user's custom emoji feature (ADR-0070 §1–§4): for a viewer whose roles have the member permission
// create_custom_emojis, the page of the settings (`/settings/custom-emojis`) that requests new custom emoji, which an
// administrator approves on the admin screen (features/admin), lists the requests, and changes and deletes the emoji
// the user made, within the limit that a supporter plan sets. The server checks the permission on every request.
// Routes and other features use it through this file only (web-client.md A3).
export { ownCustomEmojiRequestsQueryOptions, ownCustomEmojisQueryOptions } from "./api/ownEmojis";
export { OwnCustomEmojis } from "./components/OwnCustomEmojis";
