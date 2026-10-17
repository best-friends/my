// SPDX-License-Identifier: AGPL-3.0-only
// The dead jobs screen below its heading: the jobs that failed for good, most recently failed first, each with
// why it failed, its keys and payload, and a button that runs it again. A job that is enqueued again leaves
// the list, and focus moves to the next job's button so that it is not lost with the row.
import { Button, EmptyState } from "@my/ui";
import { useRef } from "react";
import { useIntl } from "react-intl";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { useT } from "../../../lib/i18n";
import type { DeadJob } from "../api/admin";
import { type RetryDeadJob, useDeadJobs, useRetryDeadJob } from "../hooks/useDeadJobs";
import { useHasPermission } from "../hooks/usePermissions";
import { DateTime, Detail, MissingPermission, MoreButton } from "./parts";

export function DeadJobList() {
  const allowed = useHasPermission("manage_jobs");
  if (!allowed) return <MissingPermission permission="manage_jobs" />;
  return <DeadJobs />;
}

function DeadJobs() {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useDeadJobs();
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((job) => job.id),
  );
  const retry = useRetryDeadJob(removed);
  if (list.items.length === 0) return <EmptyState heading={t("client:admin.dead_jobs.empty")} />;
  return (
    <div className="flex flex-col gap-4">
      <ul ref={listRef} className="flex flex-col divide-y divide-border">
        {list.items.map((job) => (
          <li key={job.id} data-row-id={job.id} className="@container py-3">
            <DeadJobRow job={job} retry={retry} />
          </li>
        ))}
      </ul>
      <MoreButton list={list} />
    </div>
  );
}

function DeadJobRow({ job, retry }: { job: DeadJob; retry: RetryDeadJob }) {
  const t = useT();
  const intl = useIntl();
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
        {/* Job types are identifiers, read left to right in every locale. */}
        <h2 className="font-mono font-semibold wrap-anywhere">
          <span dir="ltr">{job.type}</span>
        </h2>
        <Button
          size="sm"
          isPending={retry.pendingIds.includes(job.id)}
          aria-label={t("client:admin.dead_jobs.retry_job", { type: job.type, date: intl.formatDate(job.failed_at, { dateStyle: "medium", timeStyle: "short" }) })}
          onPress={() => {
            retry.retry(job);
          }}
        >
          {t("client:admin.dead_jobs.retry")}
        </Button>
      </div>
      <dl className="grid grid-cols-1 gap-2 @sm:grid-cols-2 @2xl:grid-cols-3">
        <Detail term={t("client:admin.dead_jobs.failed_at")}>
          <DateTime value={job.failed_at} />
        </Detail>
        <Detail term={t("client:admin.dead_jobs.created_at")}>
          <DateTime value={job.created_at} />
        </Detail>
        <Detail term={t("client:admin.dead_jobs.attempts")}>{intl.formatNumber(job.attempts)}</Detail>
        {job.idempotency_key !== undefined && (
          <Detail term={t("client:admin.dead_jobs.idempotency_key")}>
            <code dir="ltr">{job.idempotency_key}</code>
          </Detail>
        )}
        {job.ordering_key !== undefined && (
          <Detail term={t("client:admin.dead_jobs.ordering_key")}>
            <code dir="ltr">{job.ordering_key}</code>
          </Detail>
        )}
      </dl>
      <dl className="flex flex-col gap-2">
        <Detail term={t("client:admin.dead_jobs.last_error")}>
          {/* Errors and payloads are the server's own text, in English or JSON. */}
          <span dir="auto" className="whitespace-pre-wrap">
            {job.last_error}
          </span>
        </Detail>
        <Detail term={t("client:admin.dead_jobs.payload")}>
          <pre dir="ltr" className="rounded-md bg-surface-sunken p-2 text-sm whitespace-pre-wrap wrap-anywhere">
            <code>{job.payload}</code>
          </pre>
        </Detail>
      </dl>
    </div>
  );
}
