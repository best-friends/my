// SPDX-License-Identifier: AGPL-3.0-only
import { Button, EmojiIcon, EmojiPicker, Popover, PopoverTrigger, Tooltip, TooltipTrigger } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { type ReactionChoice, useReactionPicker } from "../hooks/useReactionPicker";

export type ReactionPickerButtonProps = {
  /** Called with the emoji picked; the popover is closed by then. */
  onSelect: (choice: ReactionChoice) => void;
  isDisabled?: boolean | undefined;
};

/**
 * An icon button of a post's action bar that opens the picker of reactions in a popover (WAI-ARIA APG, dialog
 * pattern): common Unicode emoji and the custom emoji of this server (ADR-0052 §2). The search field takes focus;
 * picking an emoji closes the popover, which returns focus to the button, and reacts with it. The button is named by
 * aria-label and shows the same words as a tooltip (web-client/16-look-states-and-wording.md G12).
 */
export function ReactionPickerButton({ onSelect, isDisabled = false }: ReactionPickerButtonProps) {
  const t = useT();
  const picker = useReactionPicker();
  const name = t("client:reaction.picker.button");
  return (
    <TooltipTrigger>
      <PopoverTrigger>
        <Button variant="subtle" size="toolbar" aria-label={name} isDisabled={isDisabled}>
          <EmojiIcon />
        </Button>
        <Popover aria-label={t("client:reaction.picker.title")} placement="bottom start">
          {({ close }) => (
            <EmojiPicker
              emojis={picker.emojis}
              recent={picker.recent}
              unicode={picker.unicode}
              labels={picker.labels}
              autoFocus
              onSelect={(emoji) => {
                picker.remember(emoji);
                close();
                onSelect({ custom: emoji });
              }}
              onSelectUnicode={(emoji) => {
                close();
                onSelect({ unicode: emoji });
              }}
            />
          )}
        </Popover>
      </PopoverTrigger>
      <Tooltip>{name}</Tooltip>
    </TooltipTrigger>
  );
}
