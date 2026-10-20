// SPDX-License-Identifier: AGPL-3.0-only
// The webhooks screen below its heading (ADR-0060, manage_webhooks): the form that sets up a webhook, then the
// webhooks, newest first. A new webhook's secret, and the new secret of a rotation, are shown once with a copy
// button. Each webhook shows its URL, events, state and why the server disabled it, and its failed attempts since
// the last success; it is changed in a dialog, enabled or disabled, sent a test message, given a new secret and
// deleted after an alert dialog. Its deliveries are on a page of their own. What a change did stays above the list
// in a StatusMessage (role="status"), and a failure stays there too (role="alert").
import { Button, ConfirmDialog, DescriptionItem, DescriptionList, Dialog, DialogTrigger, EmptyState, Link, StatusMessage } from "@my/ui";
import { type SubmitEvent, useRef, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { DateTime, DetailSection, RecordList, RequirePermission } from "../../moderation";
import type { Webhook, WebhookStateReason } from "../api/webhooks";
import {
  draftOf,
  useWebhookActions,
  useWebhookForm,
  useWebhooks,
  type WebhookActions,
  type WebhookDraft,
  type WebhookFieldError,
  webhookHost,
  webhookName,
} from "../hooks/useWebhooks";
import { SecretNotice, webhookEventLabels, WebhookFields } from "./webhookParts";

const reasonLabels: Readonly<Record<WebhookStateReason, ClientMessageKey>> = {
  admin: "client:admin.webhooks.reason.admin",
  gone: "client:admin.webhooks.reason.gone",
  failing: "client:admin.webhooks.reason.failing",
};

export function WebhookSettings() {
  const t = useT();
  return (
    <RequirePermission permission="manage_webhooks">
      <div className="flex flex-col gap-8">
        <DetailSection title={t("client:admin.webhooks.new.title")}>
          <NewWebhookForm />
        </DetailSection>
        <DetailSection title={t("client:admin.webhooks.list.title")}>
          <WebhookList />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

function NewWebhookForm() {
  const t = useT();
  const form = useWebhookForm();
  return (
    <form onSubmit={form.submit} className="@container flex flex-col gap-6">
      <WebhookFields draft={form.draft} change={form.change} error={form.error} />
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.webhooks.submit")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
      {form.created !== undefined && <SecretNotice shown={form.created} />}
    </form>
  );
}

function WebhookList() {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const webhooks = useWebhooks();
  const removed = useFocusAfterRemoval(
    listRef,
    webhooks.map((webhook) => webhook.id),
  );
  const actions = useWebhookActions(removed);
  const list = { items: webhooks, hasMore: false, isLoadingMore: false, loadMore: () => undefined };
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{actions.status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.webhooks.change_failed", { reason: actions.error })}
        </p>
      )}
      {actions.secret !== undefined && <SecretNotice shown={actions.secret} />}
      <RecordList
        list={list}
        getId={(webhook) => webhook.id}
        listRef={listRef}
        empty={<EmptyState headingLevel={3} heading={t("client:admin.webhooks.empty")} description={t("client:admin.webhooks.empty.description")} />}
      >
        {(webhook) => <WebhookRow webhook={webhook} actions={actions} />}
      </RecordList>
    </div>
  );
}

function WebhookHeading({ webhook }: { webhook: Webhook }) {
  return (
    <h3 className="min-w-0 font-semibold wrap-anywhere">
      {webhook.description === undefined || webhook.description === "" ? <span dir="ltr">{webhookHost(webhook.url)}</span> : <span dir="auto">{webhook.description}</span>}
    </h3>
  );
}

/** The facts of a webhook: its URL, events, state and failures. */
export function WebhookFacts({ webhook }: { webhook: Webhook }) {
  const t = useT();
  return (
    <DescriptionList columns={2}>
      <DescriptionItem term={t("client:admin.webhooks.url")}>
        <span dir="ltr" className="font-mono wrap-anywhere">
          {webhook.url}
        </span>
      </DescriptionItem>
      <DescriptionItem term={t("client:admin.webhooks.events")}>{webhook.events.map((event) => t(webhookEventLabels[event])).join(t("client:admin.webhooks.events.separator"))}</DescriptionItem>
      <DescriptionItem term={t("client:admin.webhooks.state")}>
        <span className="flex flex-wrap gap-x-1">
          <span>{t(webhook.state === "enabled" ? "client:admin.webhooks.state.enabled" : "client:admin.webhooks.state.disabled")}</span>
          {webhook.state === "disabled" && webhook.state_reason !== "admin" && <span className="text-warning">{t(reasonLabels[webhook.state_reason])}</span>}
        </span>
      </DescriptionItem>
      {webhook.failure !== undefined && (
        <DescriptionItem term={t("client:admin.webhooks.failure")}>
          <span className="flex flex-wrap gap-x-1">
            <span className="text-warning">{t("client:admin.webhooks.failure.count", { count: webhook.failure.consecutive_failures })}</span>
            <span>{t("client:admin.webhooks.failure.since")}</span>
            <DateTime value={webhook.failure.first_failed_at} />
          </span>
        </DescriptionItem>
      )}
      <DescriptionItem term={t("client:admin.webhooks.updated_at")}>
        <DateTime value={webhook.updated_at} />
      </DescriptionItem>
    </DescriptionList>
  );
}

function WebhookRow({ webhook, actions }: { webhook: Webhook; actions: WebhookActions }) {
  const t = useT();
  const name = webhookName(webhook);
  const isPending = actions.pendingId === webhook.id;
  const enabled = webhook.state === "enabled";
  return (
    <div className="flex flex-col gap-3">
      <WebhookHeading webhook={webhook} />
      <WebhookFacts webhook={webhook} />
      <div className="flex flex-wrap items-center gap-2">
        <Link href={`/admin/webhooks/${encodeURIComponent(webhook.id)}`} variant="standalone" aria-label={t("client:admin.webhooks.deliveries_of", { name })}>
          {t("client:admin.webhooks.deliveries")}
        </Link>
        <EditWebhook webhook={webhook} actions={actions} isPending={isPending} />
        <Button
          size="sm"
          isPending={isPending}
          aria-label={t(enabled ? "client:admin.webhooks.disable_webhook" : "client:admin.webhooks.enable_webhook", { name })}
          onPress={() => {
            actions.setEnabled(webhook, !enabled);
          }}
        >
          {t(enabled ? "client:admin.webhooks.disable" : "client:admin.webhooks.enable")}
        </Button>
        <Button
          size="sm"
          isDisabled={isPending || !enabled}
          aria-label={t("client:admin.webhooks.test_webhook", { name })}
          onPress={() => {
            actions.test(webhook);
          }}
        >
          {t("client:admin.webhooks.test")}
        </Button>
        <RotateSecret webhook={webhook} actions={actions} isPending={isPending} />
        <DeleteWebhook webhook={webhook} actions={actions} isPending={isPending} />
      </div>
    </div>
  );
}

type RowActionProps = { webhook: Webhook; actions: WebhookActions; isPending: boolean };

/** The settings of a webhook in a dialog; it closes once they are saved, and keeps a refusal on its field. */
function EditWebhook({ webhook, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = webhookName(webhook);
  return (
    <DialogTrigger>
      <Button size="sm" isDisabled={isPending} aria-label={t("client:admin.webhooks.edit_webhook", { name })}>
        {t("client:admin.webhooks.edit")}
      </Button>
      <Dialog title={t("client:admin.webhooks.edit.title", { name })} closeLabel={t("client:admin.webhooks.edit.close")} isDismissable={false} size="md">
        {({ close }) => <EditWebhookForm webhook={webhook} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditWebhookForm({ webhook, actions, close }: { webhook: Webhook; actions: WebhookActions; close: () => void }) {
  const t = useT();
  const [draft, setDraft] = useState<WebhookDraft>(() => draftOf(webhook));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<WebhookFieldError | undefined>(undefined);
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSaving(true);
    setError(undefined);
    void actions.update(webhook, { url: draft.url.trim(), description: draft.description.trim(), events: [...draft.events] }).then((refused) => {
      setIsSaving(false);
      if (refused === undefined) close();
      else setError(refused);
    });
  };
  return (
    <form onSubmit={onSubmit} className="@container flex flex-col gap-4">
      <WebhookFields
        draft={draft}
        change={(change) => {
          setDraft((previous) => ({ ...previous, ...change }));
        }}
        error={error}
        autoFocus
      />
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.webhooks.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.webhooks.save")}
        </Button>
      </div>
    </form>
  );
}

/** Giving a webhook a new secret, after a dialog: the receiver must take it within a day. */
function RotateSecret({ webhook, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = webhookName(webhook);
  const [isRotating, setIsRotating] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" isDisabled={isPending} aria-label={t("client:admin.webhooks.rotate_webhook", { name })}>
        {t("client:admin.webhooks.rotate")}
      </Button>
      <ConfirmDialog
        tone="primary"
        title={t("client:admin.webhooks.rotate.title", { name })}
        description={t("client:admin.webhooks.rotate.description")}
        confirmLabel={t("client:admin.webhooks.rotate")}
        cancelLabel={t("client:admin.webhooks.cancel")}
        isPending={isRotating}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.webhooks.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsRotating(true);
          setFailed(false);
          void actions.rotate(webhook).then((rotated) => {
            setIsRotating(false);
            if (rotated) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}

/** Deleting a webhook, after an alert dialog: it cannot be undone. */
function DeleteWebhook({ webhook, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = webhookName(webhook);
  const [isDeleting, setIsDeleting] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" isDisabled={isPending} aria-label={t("client:admin.webhooks.delete_webhook", { name })}>
        {t("client:admin.webhooks.delete")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.webhooks.delete.title", { name })}
        description={t("client:admin.webhooks.delete.description")}
        confirmLabel={t("client:admin.webhooks.delete")}
        cancelLabel={t("client:admin.webhooks.cancel")}
        isPending={isDeleting}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.webhooks.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsDeleting(true);
          setFailed(false);
          void actions.remove(webhook).then((deleted) => {
            setIsDeleting(false);
            if (deleted) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}
