// SPDX-License-Identifier: AGPL-3.0-only
import { Button, EmojiIcon, EmojiPicker, Popover, PopoverTrigger, Tooltip, TooltipTrigger } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { type EmojiTarget, useEmojiPicker } from "../hooks/useEmojiPicker";

export type EmojiPickerButtonProps = EmojiTarget & {
  /** The accessible name of the button, which says what field it adds to; the default is "Add emoji". */
  label?: string | undefined;
  isDisabled?: boolean | undefined;
};

/**
 * An icon button that opens the picker of custom emoji in a popover (WAI-ARIA APG, dialog pattern): the search field
 * takes focus, the grid of emoji follows it, and picking one puts its `:shortcode:` into the field at its caret,
 * closes the popover and moves focus back to the field. Nothing shows when the server offers no emoji. The button
 * is named by aria-label and shows the same words as a tooltip (web-client/16-look-states-and-wording.md G12).
 */
export function EmojiPickerButton({ label, isDisabled = false, ...target }: EmojiPickerButtonProps) {
  const t = useT();
  const picker = useEmojiPicker(target);
  if (!picker.available) return null;
  const name = label ?? t("client:emoji.picker.button");
  return (
    <TooltipTrigger>
      <PopoverTrigger>
        <Button variant="ghost" size="toolbar" aria-label={name} isDisabled={isDisabled}>
          <EmojiIcon />
        </Button>
        <Popover aria-label={t("client:emoji.picker.title")} placement="bottom start">
          {({ close }) => (
            <EmojiPicker
              emojis={picker.emojis}
              recent={picker.recent}
              labels={picker.labels}
              autoFocus
              onSelect={(emoji) => {
                picker.select(emoji, close);
              }}
            />
          )}
        </Popover>
      </PopoverTrigger>
      <Tooltip>{name}</Tooltip>
    </TooltipTrigger>
  );
}
