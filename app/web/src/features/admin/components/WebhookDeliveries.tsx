// SPDX-License-Identifier: AGPL-3.0-only
// The page of one webhook below its heading (ADR-0060, manage_webhooks): its facts, then its deliveries of the last
// 14 days, newest first, each with its event, whether it succeeded, how many times it was sent and its last attempt,
// and a button that sends it again with the same webhook-id. What a redelivery did stays above the list in a
// StatusMessage, and a failure in an alert.
import { Button, DescriptionItem, DescriptionList, EmptyState, StatusMessage } from "@my/ui";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { DateTime, DetailSection, RecordList, RequirePermission } from "../../moderation";
import type { WebhookAttemptOutcome, WebhookDelivery, WebhookDeliveryStatus } from "../api/webhooks";
import { useRedeliver, useWebhook, useWebhookDeliveries } from "../hooks/useWebhooks";
import { webhookEventLabels } from "./webhookParts";
import { WebhookFacts } from "./WebhookSettings";

const statusLabels: Readonly<Record<WebhookDeliveryStatus, ClientMessageKey>> = {
  pending: "client:admin.webhooks.deliveries.status.pending",
  succeeded: "client:admin.webhooks.deliveries.status.succeeded",
  failing: "client:admin.webhooks.deliveries.status.failing",
};

const outcomeLabels: Readonly<Record<WebhookAttemptOutcome, ClientMessageKey>> = {
  succeeded: "client:admin.webhooks.deliveries.outcome.succeeded",
  http_error: "client:admin.webhooks.deliveries.outcome.http_error",
  timeout: "client:admin.webhooks.deliveries.outcome.timeout",
  connection_error: "client:admin.webhooks.deliveries.outcome.connection_error",
  forbidden_destination: "client:admin.webhooks.deliveries.outcome.forbidden_destination",
};

export function WebhookDeliveries({ id }: { id: string }) {
  const t = useT();
  return (
    <RequirePermission permission="manage_webhooks">
      <div className="flex flex-col gap-8">
        <DetailSection title={t("client:admin.webhooks.details.title")}>
          <Facts id={id} />
        </DetailSection>
        <DetailSection title={t("client:admin.webhooks.deliveries.title")}>
          <DeliveryList id={id} />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

function Facts({ id }: { id: string }) {
  return <WebhookFacts webhook={useWebhook(id)} />;
}

function DeliveryList({ id }: { id: string }) {
  const t = useT();
  const list = useWebhookDeliveries(id);
  const redeliver = useRedeliver(id);
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-fg-muted">{t("client:admin.webhooks.deliveries.description")}</p>
      <StatusMessage>{redeliver.status}</StatusMessage>
      {redeliver.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.webhooks.deliveries.redeliver_failed", { reason: redeliver.error })}
        </p>
      )}
      <RecordList list={list} getId={(delivery) => delivery.id} empty={<EmptyState headingLevel={3} heading={t("client:admin.webhooks.deliveries.empty")} />}>
        {(delivery) => (
          <DeliveryRow
            delivery={delivery}
            isPending={redeliver.pendingId === delivery.id}
            onRedeliver={() => {
              redeliver.redeliver(delivery);
            }}
          />
        )}
      </RecordList>
    </div>
  );
}

function DeliveryRow({ delivery, isPending, onRedeliver }: { delivery: WebhookDelivery; isPending: boolean; onRedeliver: () => void }) {
  const t = useT();
  const event = t(webhookEventLabels[delivery.event_type]);
  const last = delivery.last_attempt;
  return (
    <div className="flex flex-col gap-3">
      <h3 className="font-semibold">{event}</h3>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:admin.webhooks.deliveries.status")}>
          <span className={delivery.status === "failing" ? "text-warning" : undefined}>{t(statusLabels[delivery.status])}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.webhooks.deliveries.attempts")}>{t("client:admin.webhooks.deliveries.attempts.count", { count: delivery.attempts })}</DescriptionItem>
        <DescriptionItem term={t("client:admin.webhooks.deliveries.created_at")}>
          <DateTime value={delivery.created_at} />
        </DescriptionItem>
        {last !== undefined && (
          <DescriptionItem term={t("client:admin.webhooks.deliveries.last_attempt")}>
            <span className="flex flex-wrap gap-x-1">
              <span>{t(outcomeLabels[last.outcome])}</span>
              {last.status_code !== undefined && <span dir="ltr">{t("client:admin.webhooks.deliveries.status_code", { code: String(last.status_code) })}</span>}
              <DateTime value={last.attempted_at} />
            </span>
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.webhooks.deliveries.message_id")}>
          <span dir="ltr" className="font-mono wrap-anywhere">
            {delivery.message_id}
          </span>
        </DescriptionItem>
      </DescriptionList>
      <div>
        <Button size="sm" isPending={isPending} aria-label={t("client:admin.webhooks.deliveries.redeliver_of", { event, id: delivery.message_id })} onPress={onRedeliver}>
          {t("client:admin.webhooks.deliveries.redeliver")}
        </Button>
      </div>
    </div>
  );
}
