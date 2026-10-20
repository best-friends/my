// SPDX-License-Identifier: AGPL-3.0-only
// The dead jobs of the job queue and running them again. A job that is enqueued again leaves the dead jobs on
// the server, so it leaves the cached pages at once.
import { useAnnouncer, useToast } from "@my/ui";
import { useMutation, useMutationState, useQueryClient, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useT } from "../../../lib/i18n";
import { type DeadJob, deadJobsQueryOptions, removeRetriedDeadJob, retryDeadJobMutation } from "../api/deadJobs";
import { type PagedList, toPagedList } from "../../moderation";

/** The dead jobs, most recently failed first. */
export function useDeadJobs(): PagedList<DeadJob> {
  const api = useApi();
  return toPagedList(useSuspenseInfiniteQuery(deadJobsQueryOptions(api)));
}

export type RetryDeadJob = {
  /** Enqueues the job again; the result is announced, and a failure is shown in a toast. */
  retry: (job: DeadJob) => void;
  /** The IDs of the jobs being enqueued again. */
  pendingIds: readonly string[];
};

/**
 * Running dead jobs again. `onRetried(id)` is called when a job is enqueued, just before it leaves the list
 * (to keep focus, lib/useFocusAfterRemoval). Several jobs may be retried at once, so everything a retry does
 * after it succeeds is in the mutation's own callbacks, which run for every call (unlike those of `mutate`;
 * TanStack Query, "Mutations").
 */
export function useRetryDeadJob(onRetried: (id: string) => void): RetryDeadJob {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const announce = useAnnouncer();
  const toast = useToast();
  const options = retryDeadJobMutation(api);
  const mutation = useMutation({
    ...options,
    onSuccess: (_, id) => {
      onRetried(id);
      const job = removeRetriedDeadJob(api, queryClient, id);
      announce(t("client:admin.dead_jobs.retried", { type: job?.type ?? id }));
    },
    // The job stays a dead job, and its row and focus stay (web-client.md E2).
    onError: (error) => {
      toast.show({ message: t("client:admin.dead_jobs.retry_failed", { reason: errorMessage(error) }), tone: "danger" });
    },
  });
  const pendingIds = useMutationState({
    filters: { mutationKey: options.mutationKey, status: "pending" },
    select: (pending) => pending.state.variables as string,
  });
  return {
    retry: (job) => {
      mutation.mutate(job.id);
    },
    pendingIds,
  };
}
