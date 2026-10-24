// SPDX-License-Identifier: AGPL-3.0-only
import { Button, EmojiIcon, Popover, PopoverTrigger, Tooltip, TooltipTrigger } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { type ReactionChoice, useReactionPicker } from "../hooks/useReactionPicker";
import { EmojiPickerPanel } from "./EmojiPickerPanel";

export type ReactionPickerButtonProps = {
  /** Called with the emoji picked; the popover is closed by then. */
  onSelect: (choice: ReactionChoice) => void;
  isDisabled?: boolean | undefined;
};

/**
 * An icon button of a post's action bar that opens the picker of reactions in a popover (WAI-ARIA APG, dialog
 * pattern): the post form's emoji picker (ADR-0091 §7), with the emoji used often, the custom emoji of this server
 * and every Unicode emoji in the skin tone chosen here. Hovering or focusing it starts loading the Unicode emoji, and
 * the popover shows a spinner until they arrive. The search field takes focus; picking an emoji closes the popover,
 * which returns focus to the button, and reacts with it, also when Shift is held (one reaction per post, ADR-0052 §1).
 * The button is named by aria-label and shows the same words as a tooltip
 * (web-client/16-look-states-and-wording.md G12).
 */
export function ReactionPickerButton({ onSelect, isDisabled = false }: ReactionPickerButtonProps) {
  const t = useT();
  const picker = useReactionPicker();
  const name = t("client:reaction.picker.button");
  return (
    <TooltipTrigger>
      <PopoverTrigger
        onOpenChange={(isOpen) => {
          if (isOpen) picker.preload();
        }}
      >
        <Button variant="subtle" size="toolbar" aria-label={name} isDisabled={isDisabled} onHoverStart={picker.preload} onFocus={picker.preload}>
          <EmojiIcon />
        </Button>
        <Popover aria-label={t("client:reaction.picker.title")} placement="bottom start">
          {({ close }) => (
            <EmojiPickerPanel
              picker={picker}
              onSelect={(selection) => {
                picker.select(selection, close, onSelect);
              }}
            />
          )}
        </Popover>
      </PopoverTrigger>
      <Tooltip>{name}</Tooltip>
    </TooltipTrigger>
  );
}
