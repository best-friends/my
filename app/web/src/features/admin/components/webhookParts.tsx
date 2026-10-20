// SPDX-License-Identifier: AGPL-3.0-only
// What the webhook screens share (ADR-0060): the labels of the events, the fields of a webhook's settings, and the
// secret shown once after it is made.
import { Button, Checkbox, TextField } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { webhookDescriptionLimit, type WebhookEventType, webhookEvents, type WebhookMessageType, webhookUrlLimit } from "../api/webhooks";
import { type ShownSecret, useCopySecret, type WebhookDraft, type WebhookFieldError, webhookName } from "../hooks/useWebhooks";

export const webhookEventLabels: Readonly<Record<WebhookMessageType, ClientMessageKey>> = {
  "report.created": "client:admin.webhooks.event.report_created",
  "report.resolved": "client:admin.webhooks.event.report_resolved",
  "report.reopened": "client:admin.webhooks.event.report_reopened",
  "registration.pending": "client:admin.webhooks.event.registration_pending",
  "account.created": "client:admin.webhooks.event.account_created",
  "account.updated": "client:admin.webhooks.event.account_updated",
  "moderation_action.created": "client:admin.webhooks.event.moderation_action_created",
  "moderation_action.lifted": "client:admin.webhooks.event.moderation_action_lifted",
  "appeal.created": "client:admin.webhooks.event.appeal_created",
  "domain_moderation_action.created": "client:admin.webhooks.event.domain_moderation_action_created",
  "domain_moderation_action.lifted": "client:admin.webhooks.event.domain_moderation_action_lifted",
  "status.created": "client:admin.webhooks.event.status_created",
  "custom_emoji.created": "client:admin.webhooks.event.custom_emoji_created",
  "custom_emoji.updated": "client:admin.webhooks.event.custom_emoji_updated",
  "custom_emoji.deleted": "client:admin.webhooks.event.custom_emoji_deleted",
  "webhook.test": "client:admin.webhooks.event.webhook_test",
};

type WebhookFieldsProps = {
  draft: WebhookDraft;
  change: (change: Partial<WebhookDraft>) => void;
  /** The server's refusal, shown on its field. */
  error: WebhookFieldError | undefined;
  autoFocus?: boolean;
};

/** The URL, the description and the events of a webhook. */
export function WebhookFields({ draft, change, error, autoFocus = false }: WebhookFieldsProps) {
  const t = useT();
  const eventsId = useId();
  const eventsHintId = useId();
  const eventsErrorId = useId();
  const fieldError = (field: WebhookFieldError["field"]) => (error?.field === field ? { isInvalid: true, errorMessage: error.message } : {});
  const toggle = (event: WebhookEventType, selected: boolean) => {
    change({ events: selected ? webhookEvents.filter((e) => e === event || draft.events.includes(e)) : draft.events.filter((e) => e !== event) });
  };
  return (
    <>
      <TextField
        type="url"
        label={t("client:admin.webhooks.url")}
        description={t("client:admin.webhooks.url.description")}
        value={draft.url}
        onChange={(url) => {
          change({ url });
        }}
        isRequired
        maxLength={webhookUrlLimit}
        autoComplete="off"
        spellCheck="false"
        autoFocus={autoFocus}
        {...fieldError("url")}
      />
      <TextField
        label={t("client:admin.webhooks.description")}
        description={t("client:admin.webhooks.description.description", { max: String(webhookDescriptionLimit) })}
        value={draft.description}
        onChange={(description) => {
          change({ description });
        }}
        maxLength={webhookDescriptionLimit}
        autoComplete="off"
        {...fieldError("description")}
      />
      <div
        role="group"
        aria-labelledby={eventsId}
        aria-describedby={error?.field === "events" ? `${eventsHintId} ${eventsErrorId}` : eventsHintId}
        className="flex flex-col gap-2"
      >
        <span id={eventsId} className="text-sm font-medium text-fg">
          {t("client:admin.webhooks.events")}
        </span>
        <p id={eventsHintId} className="text-sm text-fg-muted">
          {t("client:admin.webhooks.events.description")}
        </p>
        <div className="grid gap-2 @md:grid-cols-2">
          {webhookEvents.map((event) => (
            <Checkbox
              key={event}
              isSelected={draft.events.includes(event)}
              onChange={(selected) => {
                toggle(event, selected);
              }}
            >
              {t(webhookEventLabels[event])}
            </Checkbox>
          ))}
        </div>
        {error?.field === "events" && (
          <div id={eventsErrorId} role="alert" className="flex flex-col gap-1 text-sm text-danger">
            <p>{error.message}</p>
            {error.missing.length > 0 && <p>{t("client:admin.webhooks.missing_permissions", { permissions: `⁦${error.missing.join(", ")}⁩` })}</p>}
          </div>
        )}
      </div>
      {error?.field === "form" && (
        <p role="alert" className="text-danger">
          {t("client:admin.webhooks.save_failed", { reason: error.message })}
        </p>
      )}
    </>
  );
}

/** A secret, shown once: the webhook it is for, the secret to copy, and that it is not shown again. */
export function SecretNotice({ shown }: { shown: ShownSecret }) {
  const t = useT();
  const copy = useCopySecret();
  const labelId = useId();
  return (
    <div className="flex flex-col gap-2 rounded-md border border-border p-3">
      <p id={labelId} className="font-medium">
        {t("client:admin.webhooks.secret.title", { name: webhookName(shown.webhook) })}
      </p>
      <p className="text-sm text-fg-muted">{t("client:admin.webhooks.secret.notice")}</p>
      <p aria-labelledby={labelId} dir="ltr" className="font-mono wrap-anywhere select-all">
        {shown.secret}
      </p>
      <div>
        <Button
          size="sm"
          onPress={() => {
            copy(shown.secret);
          }}
        >
          {t("client:admin.webhooks.secret.copy")}
        </Button>
      </div>
    </div>
  );
}
