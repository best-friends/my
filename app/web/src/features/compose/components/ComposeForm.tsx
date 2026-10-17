// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Checkbox, Dialog, Spinner, TextArea, TextField } from "@my/ui";
import type { SubmitEvent } from "react";
import { useLocale, useT } from "../../../lib/i18n";
import { maxAttachments } from "../hooks/policy";
import { type ComposeController, composeTextId } from "../hooks/useCompose";
import { type ReadyComposeForm, useComposeForm } from "../hooks/useComposeForm";
import { useFormatRemaining } from "../hooks/useFormatRemaining";
import { AttachmentList } from "./AttachmentList";
import { ComposeTools } from "./ComposeTools";
import { Outbox } from "./Outbox";
import { ReplyContext } from "./ReplyContext";

export type ComposeFormProps = {
  controller: ComposeController;
  /** The id of the heading that names the form. */
  labelledBy: string;
  /** Focus the text area when the form appears (in the sheet, which the user just opened). */
  autoFocus?: boolean;
};

/** The new-post form; its logic is useComposeForm. */
export function ComposeForm({ controller, labelledBy, autoFocus = false }: ComposeFormProps) {
  const t = useT();
  const form = useComposeForm(controller);
  switch (form.status) {
    case "loading":
      return <Spinner label={t("client:shell.loading")} />;
    case "error":
      return (
        <div className="flex flex-col items-start gap-2">
          <p>{form.message}</p>
          <Button size="sm" onPress={form.retry}>
            {t("client:shell.error.retry")}
          </Button>
        </div>
      );
    case "ready":
      return <ReadyForm form={form} labelledBy={labelledBy} autoFocus={autoFocus} />;
  }
}

type ReadyFormProps = { form: ReadyComposeForm; labelledBy: string; autoFocus: boolean };

function ReadyForm({ form, labelledBy, autoFocus }: ReadyFormProps) {
  const t = useT();
  const locale = useLocale();
  const formatRemaining = useFormatRemaining();
  const { draft, attachments } = form;
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    form.submit();
  };
  return (
    <div className="flex flex-col gap-4 @container">
      <form aria-labelledby={labelledBy} onSubmit={onSubmit} className="flex flex-col gap-3">
        {form.replyTo.status !== "none" && <ReplyContext replyTo={form.replyTo} onCancel={form.cancelReply} />}
        {draft.hasWarning && (
          <TextField
            label={t("client:compose.warning.label")}
            description={t("client:compose.warning.description")}
            value={draft.spoilerText}
            onChange={(spoilerText) => {
              form.edit({ spoilerText });
            }}
          />
        )}
        <TextArea
          id={composeTextId}
          label={t("client:compose.text.label")}
          value={draft.text}
          onChange={(text) => {
            form.edit({ text });
          }}
          rows={4}
          characterLimit={form.textLimit}
          formatRemaining={formatRemaining}
          autoFocus={autoFocus}
        />
        <AttachmentList
          items={attachments.items}
          onDescribe={attachments.describe}
          onRemove={attachments.remove}
          requireDescriptions={form.requireDescriptions}
        />
        {attachments.items.length > 0 && (
          <p className="text-sm text-fg-muted">{t("client:compose.media.hint", { max: maxAttachments })}</p>
        )}
        {attachments.items.length > 0 && (
          <Checkbox
            isSelected={draft.sensitive}
            onChange={(sensitive) => {
              form.edit({ sensitive });
            }}
          >
            {t("client:compose.sensitive")}
          </Checkbox>
        )}
        {form.unreachedMentions.length > 0 && (
          <p className="text-sm text-warning">
            {t("client:compose.mention.unreached", {
              count: form.unreachedMentions.length,
              accounts: new Intl.ListFormat(locale, { type: "conjunction" }).format(form.unreachedMentions.map((acct) => `@${acct}`)),
            })}
          </p>
        )}
        {form.error !== null && <p className="text-danger">{form.error}</p>}
        {/* The tools and the submit button share the row under the text, so that the button stays on the first screen at 320 px. */}
        <div className="flex flex-wrap items-center gap-2">
          <ComposeTools form={form} />
          <Button type="submit" variant="primary" className="ms-auto" isDisabled={!form.canSubmit}>
            {t(form.replyTo.status === "none" ? "client:compose.submit" : "client:compose.submit.reply")}
          </Button>
        </div>
      </form>
      <Outbox items={form.outbox} onRetry={form.retry} onEdit={form.editOutbox} canEdit={form.canEditOutbox} onDiscard={form.discard} />
      <Dialog
        role="alertdialog"
        title={t("client:compose.missing_description.title")}
        isOpen={form.confirmation.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) form.confirm("describe");
        }}
        size="sm"
      >
        <div className="flex flex-col gap-4">
          <p>{t("client:compose.missing_description.body", { count: form.confirmation.missing })}</p>
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              autoFocus
              onPress={() => {
                form.confirm("describe");
              }}
            >
              {t("client:compose.missing_description.add")}
            </Button>
            <Button
              variant="primary"
              onPress={() => {
                form.confirm("send");
              }}
            >
              {t("client:compose.missing_description.post")}
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
