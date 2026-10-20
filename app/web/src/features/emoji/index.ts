// SPDX-License-Identifier: AGPL-3.0-only
// The custom emoji feature (FR-111, ADR-0051 §4): the picker of the custom emoji that the server offers, which puts
// a picked emoji's `:shortcode:` into a text field (the post form, the edit of a post, the profile), with the emoji
// picked recently on this browser, and the picker of the emoji to react to a post with (ADR-0052 §5). The admin screen
// of the emoji (features/admin) marks the list stale.
// Routes and other features use it through this file only (web-client.md A3).
export { EmojiPickerButton } from "./components/EmojiPickerButton";
export { ReactionPickerButton } from "./components/ReactionPickerButton";
export type { ReactionChoice } from "./hooks/useReactionPicker";
