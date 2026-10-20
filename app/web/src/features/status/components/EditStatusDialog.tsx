// SPDX-License-Identifier: AGPL-3.0-only
// The dialog that edits the viewer's own post (FR-109), opened from the post's "More actions" menu. It is a
// modal dialog (WAI-ARIA APG, dialog pattern) holding the compose form's fields for what an edit may change:
// the content warning, the text with its counter, the images' descriptions, the sensitivity of the media,
// and the language. The visibility and the reply target stay as they were.
import { Button, Checkbox, Dialog, LanguageIcon, Select, SelectItem, Spinner, TextArea, TextField, ToggleButton, Tooltip, TooltipTrigger, WarningIcon } from "@my/ui";
import { type SubmitEvent, useId } from "react";
import { useT } from "../../../lib/i18n";
import { AttachmentList, useFormatRemaining } from "../../compose";
import type { Status, StatusSource } from "../api/status";
import { useEditSource, useEditStatusForm } from "../hooks/useEditStatus";

export type EditStatusDialogProps = {
  /** The viewer's post. */
  status: Status;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
};

/** The edit dialog. Its content mounts when it opens, so every opening starts from the post as it is now. */
export function EditStatusDialog({ status, isOpen, onOpenChange }: EditStatusDialogProps) {
  const t = useT();
  return (
    <Dialog title={t("client:status.edit.title")} closeLabel={t("client:status.edit.close")} isOpen={isOpen} onOpenChange={onOpenChange} size="lg">
      {({ close }) => <EditStatusContent status={status} close={close} />}
    </Dialog>
  );
}

type EditStatusContentProps = { status: Status; close: () => void };

function EditStatusContent({ status, close }: EditStatusContentProps) {
  const t = useT();
  const source = useEditSource(status.id);
  switch (source.status) {
    case "loading":
      return <Spinner label={t("client:status.edit.loading")} />;
    case "error":
      return (
        <div className="flex flex-col items-start gap-2">
          <p>{t("client:status.edit.load_failed", { reason: source.message })}</p>
          <Button size="sm" onPress={source.retry}>
            {t("client:status.edit.retry")}
          </Button>
        </div>
      );
    case "ready":
      return <EditStatusForm status={status} source={source.source} maxCharacters={source.maxCharacters} close={close} />;
  }
}

type EditStatusFormProps = {
  status: Status;
  source: StatusSource;
  maxCharacters: number;
  close: () => void;
};

function EditStatusForm({ status, source, maxCharacters, close }: EditStatusFormProps) {
  const t = useT();
  const formatRemaining = useFormatRemaining();
  const errorId = useId();
  const form = useEditStatusForm(status, source, maxCharacters);
  const warningLabel = t("client:compose.warning.toggle");
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    form.save(close);
  };
  return (
    <form aria-label={t("client:status.edit.title")} onSubmit={onSubmit} className="flex flex-col gap-3 @container">
      {form.hasWarning && (
        <TextField
          label={t("client:compose.warning.label")}
          description={t("client:compose.warning.description")}
          value={form.spoilerText}
          onChange={(spoilerText) => {
            form.edit({ spoilerText });
          }}
        />
      )}
      <TextArea
        label={t("client:compose.text.label")}
        value={form.text}
        onChange={(text) => {
          form.edit({ text });
        }}
        rows={4}
        characterLimit={form.textLimit}
        formatRemaining={formatRemaining}
        autoFocus
      />
      <AttachmentList items={form.attachments} onDescribe={form.describe} requireDescriptions={false} />
      {form.attachments.length > 0 && (
        <Checkbox
          isSelected={form.sensitive}
          onChange={(sensitive) => {
            form.edit({ sensitive });
          }}
        >
          {t("client:compose.sensitive")}
        </Checkbox>
      )}
      <p className="text-sm text-fg-muted">{t("client:status.edit.fixed")}</p>
      {form.error !== null && (
        <p id={errorId} role="alert" className="text-danger">
          {form.error}
        </p>
      )}
      {/* The same row as the compose form's tools: the content warning and the language, then the buttons. */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex min-w-0 flex-wrap items-center gap-1">
          <TooltipTrigger>
            <ToggleButton
              aria-label={warningLabel}
              isSelected={form.hasWarning}
              onChange={(hasWarning) => {
                form.edit({ hasWarning });
              }}
            >
              <WarningIcon filled={form.hasWarning} />
            </ToggleButton>
            <Tooltip>{warningLabel}</Tooltip>
          </TooltipTrigger>
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
        <div className="ms-auto flex flex-wrap gap-2">
          <Button onPress={close}>{t("client:status.edit.cancel")}</Button>
          <Button
            type="submit"
            variant="primary"
            isDisabled={!form.canSave && !form.isSaving}
            isPending={form.isSaving}
            {...(form.error !== null && { "aria-describedby": errorId })}
          >
            {t("client:status.edit.save")}
          </Button>
        </div>
      </div>
    </form>
  );
}
