// SPDX-License-Identifier: AGPL-3.0-only
// The custom emoji picked recently on this browser (ADR-0051 §4: the recent emoji are kept per device). They are
// how this browser is used, like the theme, so they are kept in localStorage (lib/storage `recentEmojisKey`),
// outlive the session and are shared by the accounts that sign in here. Only shortcodes are kept; the picker
// shows those that are still offered.
import { recentEmojisKey, useStorage } from "../../../lib/storage";

/** How many emoji the picker shows as recent. */
const maxRecentEmojis = 16;

/** Puts `shortcode` first among the recent emoji. */
export function rememberEmoji(shortcode: string): void {
  const next = [shortcode, ...recentEmojisKey.read().filter((code) => code !== shortcode)].slice(0, maxRecentEmojis);
  recentEmojisKey.write(next);
}

/** The shortcodes of the recent emoji, most recent first, rendering again when they change. */
export function useRecentShortcodes(): readonly string[] {
  return useStorage(recentEmojisKey);
}
