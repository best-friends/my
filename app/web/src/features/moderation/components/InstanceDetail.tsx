// SPDX-License-Identifier: AGPL-3.0-only
// A remote server's page below its heading (federation.md §7.3): its health, why it is in its state, when
// deliveries last resumed, the button that resumes them, the way to its moderation actions (FR-702), and the
// history of its availability, newest first.
import { Button, DescriptionItem, DescriptionList, EmptyState, Link } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { AvailabilityEvent } from "../api/staff";
import { useInstance } from "../hooks/useInstances";
import { Day, DateTime, DetailSection, InstanceStateLabel, RecordList, RequirePermission } from "./parts";

const eventLabels: Readonly<Record<AvailabilityEvent["kind"], ClientMessageKey>> = {
  marked_unavailable: "client:admin.instance.event.marked_unavailable",
  marked_available: "client:admin.instance.event.marked_available",
};

const reasonLabels: Readonly<Record<AvailabilityEvent["reason"], ClientMessageKey>> = {
  failure_days: "client:admin.instance.reason.failure_days",
  delivery_succeeded: "client:admin.instance.reason.delivery_succeeded",
  inbound_request: "client:admin.instance.reason.inbound_request",
  probe_succeeded: "client:admin.instance.reason.probe_succeeded",
  operator: "client:admin.instance.reason.operator",
};

export type InstanceDetailProps = { domain: string };

export function InstanceDetail({ domain }: InstanceDetailProps) {
  return (
    <RequirePermission permission="manage_domains">
      <InstanceView domain={domain} />
    </RequirePermission>
  );
}

function InstanceView({ domain }: InstanceDetailProps) {
  const t = useT();
  const descriptionId = useId();
  const moderationId = useId();
  const view = useInstance(domain);
  const { health, events } = view;
  // The newest event says why the server is in its state; the newest marked_available one, when deliveries
  // last resumed. Both are among the loaded events only if the server has any.
  const latest = events.items[0];
  const resumed = events.items.find((event) => event.kind === "marked_available");
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4">
        <DescriptionList columns={3}>
          <DescriptionItem term={t("client:admin.instance.state")}>
            <InstanceStateLabel state={health.state} />
          </DescriptionItem>
          <DescriptionItem term={t("client:admin.instance.since")}>{health.since === undefined ? t("client:admin.instance.none") : <DateTime value={health.since} />}</DescriptionItem>
          <DescriptionItem term={t("client:admin.instance.reason")}>{latest === undefined ? t("client:admin.instance.none") : t(reasonLabels[latest.reason])}</DescriptionItem>
          <DescriptionItem term={t("client:admin.instance.failure_days")}>{t("client:admin.instance.failure_days.value", { count: health.failure_days })}</DescriptionItem>
          <DescriptionItem term={t("client:admin.instance.last_failure")}>
            {health.last_failure_day === undefined ? t("client:admin.instance.none") : <Day value={health.last_failure_day} />}
          </DescriptionItem>
          <DescriptionItem term={t("client:admin.instance.last_resumed")}>{resumed === undefined ? t("client:admin.instance.none") : <DateTime value={resumed.occurred_at} />}</DescriptionItem>
        </DescriptionList>
        {view.canMarkAvailable && (
          <div className="flex flex-col items-start gap-1">
            <Button variant="primary" isPending={view.isMarking} onPress={view.markAvailable} aria-describedby={descriptionId}>
              {t("client:admin.instance.mark_available")}
            </Button>
            <p id={descriptionId} className="text-sm text-fg-muted">
              {t("client:admin.instance.mark_available.description")}
            </p>
          </div>
        )}
      </div>
      <DetailSection title={t("client:admin.instance.moderation.title")}>
        <div className="flex flex-col items-start gap-1">
          <Link href={`/moderation/domain-actions?domain=${encodeURIComponent(domain)}`} variant="standalone" aria-describedby={moderationId}>
            {t("client:admin.instance.moderation.link")}
          </Link>
          <p id={moderationId} className="text-sm text-fg-muted">
            {t("client:admin.instance.moderation.description")}
          </p>
        </div>
      </DetailSection>
      <DetailSection title={t("client:admin.instance.history.title")}>
        <RecordList list={events} getId={(event) => event.id} empty={<EmptyState headingLevel={3} heading={t("client:admin.instance.history.empty")} />}>
          {(event) => (
            <div className="flex flex-col">
              <h3 className="font-semibold">{t(eventLabels[event.kind])}</h3>
              <span className="text-sm text-fg-muted">
                <DateTime value={event.occurred_at} />
              </span>
              <span>{t(reasonLabels[event.reason])}</span>
            </div>
          )}
        </RecordList>
      </DetailSection>
    </div>
  );
}
