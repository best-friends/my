// SPDX-License-Identifier: AGPL-3.0-only
import { tv } from "tailwind-variants";
import { CustomEmoji, type CustomEmojiImage } from "./CustomEmoji";

const emojiTile = tv({
  base: "flex size-12 shrink-0 items-center justify-center rounded-md bg-surface-sunken text-3xl",
});

export type EmojiTileProps = {
  /** The emoji to show; the tile stays empty while its image is unknown. */
  emoji?: CustomEmojiImage | undefined;
};

/**
 * A custom emoji on a tile, beside the heading that names it (the tile is hidden from assistive technology, so the
 * heading must carry the shortcode).
 */
export function EmojiTile({ emoji }: EmojiTileProps) {
  return (
    <span aria-hidden="true" className={emojiTile()}>
      {emoji !== undefined && <CustomEmoji emoji={emoji} />}
    </span>
  );
}
