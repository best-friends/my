// SPDX-License-Identifier: AGPL-3.0-only
// The dead jobs screen below its heading: the jobs that failed for good, most recently failed first, each with
// why it failed, its keys and payload, and a button that runs it again. A job that is enqueued again leaves
// the list, and focus moves to the next job's button so that it is not lost with the row.
import { Button, DescriptionItem, DescriptionList, EmptyState } from "@my/ui";
import { useRef } from "react";
import { useIntl } from "react-intl";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { useT } from "../../../lib/i18n";
import type { DeadJob } from "../api/deadJobs";
import { type RetryDeadJob, useDeadJobs, useRetryDeadJob } from "../hooks/useDeadJobs";
import { DateTime, RecordList, RequirePermission } from "../../moderation";

export function DeadJobList() {
  return (
    <RequirePermission permission="manage_jobs">
      <DeadJobs />
    </RequirePermission>
  );
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
  return (
    <RecordList list={list} getId={(job) => job.id} listRef={listRef} empty={<EmptyState heading={t("client:admin.dead_jobs.empty")} />}>
      {(job) => <DeadJobRow job={job} retry={retry} />}
    </RecordList>
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
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.dead_jobs.failed_at")}>
          <DateTime value={job.failed_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.dead_jobs.created_at")}>
          <DateTime value={job.created_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.dead_jobs.attempts")}>{intl.formatNumber(job.attempts)}</DescriptionItem>
        {job.idempotency_key !== undefined && (
          <DescriptionItem term={t("client:admin.dead_jobs.idempotency_key")}>
            <code dir="ltr">{job.idempotency_key}</code>
          </DescriptionItem>
        )}
        {job.ordering_key !== undefined && (
          <DescriptionItem term={t("client:admin.dead_jobs.ordering_key")}>
            <code dir="ltr">{job.ordering_key}</code>
          </DescriptionItem>
        )}
      </DescriptionList>
      <DescriptionList>
        <DescriptionItem term={t("client:admin.dead_jobs.last_error")}>
          {/* Errors and payloads are the server's own text, in English or JSON. */}
          <span dir="auto" className="whitespace-pre-wrap">
            {job.last_error}
          </span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.dead_jobs.payload")}>
          <pre dir="ltr" className="rounded-md bg-surface-sunken p-2 text-sm whitespace-pre-wrap wrap-anywhere">
            <code>{job.payload}</code>
          </pre>
        </DescriptionItem>
      </DescriptionList>
    </div>
  );
}
