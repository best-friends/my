// SPDX-License-Identifier: AGPL-3.0-only
import {
  Button,
  FileTrigger,
  ImageIcon,
  LanguageIcon,
  PollIcon,
  Select,
  SelectItem,
  ToggleButton,
  Tooltip,
  TooltipTrigger,
  VisuallyHidden,
  WarningIcon,
} from "@my/ui";
import { useId } from "react";
import { useT } from "../../../lib/i18n";
import { EmojiPickerButton } from "../../emoji";
import { acceptedMediaTypes, maxAttachments } from "../hooks/policy";
import { composeTextId } from "../hooks/useCompose";
import type { ReadyComposeForm } from "../hooks/useComposeForm";
import { VisibilityPicker } from "./VisibilityPicker";

export type ComposeToolsProps = { form: ReadyComposeForm };

/**
 * The tools of the form, in one row under the text: images, poll, content warning, custom emoji, visibility and language
 * (web-client.md §12). Every control is a button of the same height, so the row stays level at every width.
 * The icon-only buttons are named by aria-label and show the same words as a tooltip (G12); each control
 * also says its current value: the images attached (description), the poll and the warning (aria-pressed), the
 * visibility (its name and who can see it) and the language (the select shows it).
 */
export function ComposeTools({ form }: ComposeToolsProps) {
  const t = useT();
  const countId = useId();
  const hintId = useId();
  const { draft, attachments } = form;
  const count = attachments.items.length;
  const addLabel = t("client:compose.media.add");
  const warningLabel = t("client:compose.warning.toggle");
  const pollLabel = t("client:compose.poll.toggle");
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-1">
      <TooltipTrigger>
        <FileTrigger
          acceptedFileTypes={acceptedMediaTypes}
          allowsMultiple
          onSelect={(files) => {
            attachments.add(files === null ? [] : Array.from(files));
          }}
        >
          <Button
            variant="ghost"
            size="toolbar"
            aria-label={addLabel}
            aria-describedby={count > 0 ? `${countId} ${hintId}` : hintId}
            isDisabled={!form.canAttach}
          >
            <ImageIcon />
          </Button>
        </FileTrigger>
        <Tooltip>{addLabel}</Tooltip>
      </TooltipTrigger>
      {count > 0 && <VisuallyHidden id={countId}>{t("client:compose.media.count", { count })}</VisuallyHidden>}
      <VisuallyHidden id={hintId}>{t("client:compose.media.hint", { max: maxAttachments })}</VisuallyHidden>
      <TooltipTrigger>
        <ToggleButton
          aria-label={pollLabel}
          isSelected={form.poll.draft !== null}
          isDisabled={!form.poll.canAdd}
          onChange={form.poll.toggle}
        >
          <PollIcon filled={form.poll.draft !== null} />
        </ToggleButton>
        <Tooltip>{pollLabel}</Tooltip>
      </TooltipTrigger>
      <TooltipTrigger>
        <ToggleButton
          aria-label={warningLabel}
          isSelected={draft.hasWarning}
          onChange={(hasWarning) => {
            form.edit({ hasWarning });
          }}
        >
          <WarningIcon filled={draft.hasWarning} />
        </ToggleButton>
        <Tooltip>{warningLabel}</Tooltip>
      </TooltipTrigger>
      <EmojiPickerButton
        fieldId={composeTextId}
        value={draft.text}
        onChange={(text) => {
          form.edit({ text });
        }}
      />
      <VisibilityPicker
        value={form.visibility}
        limits={form.limits}
        onChange={(visibility) => {
          form.edit({ visibility });
        }}
      />
      <Select
        variant="toolbar"
        aria-label={t("client:compose.language.label")}
        icon={<LanguageIcon />}
        items={form.languages}
        value={form.language}
        onChange={(language) => {
          if (typeof language === "string") form.edit({ language });
        }}
      >
        {(option) => (
          <SelectItem id={option.id} textValue={option.name}>
            {option.name}
          </SelectItem>
        )}
      </Select>
    </div>
  );
}
