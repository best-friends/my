// SPDX-License-Identifier: AGPL-3.0-only
import { Button, EmojiIcon, EmojiPicker, InlineError, Popover, PopoverTrigger, Spinner, Tooltip, TooltipTrigger } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { type EmojiTarget, useEmojiPicker } from "../hooks/useEmojiPicker";

export type EmojiPickerButtonProps = EmojiTarget & {
  /** The accessible name of the button, which says what field it adds to; the default is "Add emoji". */
  label?: string | undefined;
  isDisabled?: boolean | undefined;
};

/**
 * An icon button that opens the emoji picker in a popover (WAI-ARIA APG, dialog pattern; ADR-0091 §4), for the
 * `endAction` of a TextArea or TextField. Hovering or focusing it starts loading the Unicode emoji, and the popover
 * shows a spinner until they arrive. Then the search field takes focus, and picking an emoji puts it into the field at
 * its caret, closes the popover and moves focus back to the field; picking with Shift keeps the popover open. The
 * button is named by aria-label and shows the same words as a tooltip (web-client/16-look-states-and-wording.md G12).
 */
export function EmojiPickerButton({ label, isDisabled = false, ...target }: EmojiPickerButtonProps) {
  const t = useT();
  const picker = useEmojiPicker(target);
  const name = label ?? t("client:emoji.picker.button");
  return (
    <TooltipTrigger>
      <PopoverTrigger
        onOpenChange={(isOpen) => {
          if (isOpen) picker.preload();
        }}
      >
        <Button variant="ghost" size="icon" aria-label={name} isDisabled={isDisabled} onHoverStart={picker.preload} onFocus={picker.preload}>
          <EmojiIcon />
        </Button>
        <Popover aria-label={t("client:emoji.picker.title")} placement="bottom end">
          {({ close }) =>
            picker.status === "loading" ? (
              <Spinner label={t("client:emoji.picker.loading")} />
            ) : (
              <>
                {picker.status === "error" && <InlineError>{t("client:emoji.picker.load_failed")}</InlineError>}
                <EmojiPicker
                  sections={picker.sections}
                  labels={picker.labels}
                  skinTone={picker.skinTone}
                  onSkinToneChange={picker.setSkinTone}
                  autoFocus
                  onSelect={(selection) => {
                    picker.select(selection, close);
                  }}
                />
              </>
            )
          }
        </Popover>
      </PopoverTrigger>
      <Tooltip>{name}</Tooltip>
    </TooltipTrigger>
  );
}
