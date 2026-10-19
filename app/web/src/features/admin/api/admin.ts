// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the admin screens (web-client.md S1): the health of remote servers
// (federation.md §7.3) with the history of their availability, the moderation actions on domains (FR-702),
// and the dead jobs of the job queue. The
// lists follow the API's keyset pagination (each page's `next_cursor` is the `cursor` of the next) as TanStack
// Query infinite queries on the same client; their keys start like the client's own `[method, path]` and end
// with "pages", so that they never collide with a plain query of the same operation (web-client.md §10).
import { infiniteQueryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";

type Schemas = components["schemas"];

export type Permission = Schemas["Permission"];
export type InstanceHealth = Schemas["AdminInstanceHealth"];
export type InstanceState = Schemas["AdminInstanceState"];
export type AvailabilityEvent = Schemas["AdminAvailabilityEvent"];
export type DeadJob = Schemas["AdminDeadJob"];

/** The filters of the list of remote servers, in the order of its tabs; `all` sends no `state`. */
export const instanceFilters = ["all", "unavailable", "available"] as const;

export type InstanceFilter = (typeof instanceFilters)[number];

/** The page size of the lists. */
const pageSize = 40;

/** The query of one page of a list: its size and, after the first page, the cursor that continues it. */
export function pageQuery(cursor: string | undefined): { limit: number; cursor?: string } {
  return cursor === undefined ? { limit: pageSize } : { limit: pageSize, cursor };
}

/** The remote servers with delivery failures or marked unavailable, in domain order (GET /admin/instances). */
export function instancesQueryOptions(api: Api, filter: InstanceFilter) {
  const state = filter === "all" ? undefined : filter;
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/instances", { state }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const query = { ...pageQuery(pageParam), ...(state === undefined ? {} : { state }) };
      const { data, error } = await api.fetch.GET("/admin/instances", { params: { query }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The health of one remote server (GET /admin/instances/{domain}). */
export function instanceQueryOptions(api: Api, domain: string) {
  return api.query.queryOptions("get", "/admin/instances/{domain}", { params: { path: { domain } } });
}

/** When and why a remote server was marked unavailable or available, newest first (GET /admin/instances/{domain}/availability_events). */
export function availabilityEventsQueryOptions(api: Api, domain: string) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/instances/{domain}/availability_events", { domain }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/instances/{domain}/availability_events", {
        params: { path: { domain }, query: pageQuery(pageParam) },
        signal,
      });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/**
 * Marks a remote server available (POST /admin/instances/{domain}/mark_available), as the options of TanStack
 * Query's useMutation (the client's own useMutation is a hook read from an object, which React Compiler
 * cannot compile around; web-client.md §10). Returns the server's health after it.
 */
export function markAvailableMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/instances/{domain}/mark_available"] as const,
    mutationFn: async (domain: string): Promise<InstanceHealth> => {
      const { data, error } = await api.fetch.POST("/admin/instances/{domain}/mark_available", { params: { path: { domain } } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** The jobs that failed for good, most recently failed first (GET /admin/dead_jobs). */
export function deadJobsQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/dead_jobs", "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/dead_jobs", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The problem type of an admin operation that the viewer's roles do not allow (api/openapi.yaml, Problem). */
const missingPermissionType = "/api/v1/problems/missing-permission";

/** Whether `error` is the server's refusal of an admin operation for the viewer's lack of its permission. */
export function isMissingPermission(error: unknown): boolean {
  return error instanceof ApiError && error.type === missingPermissionType;
}

/** Runs a dead job again (POST /admin/dead_jobs/{id}/retry), as the options of useMutation. */
export function retryDeadJobMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/dead_jobs/{id}/retry"] as const,
    mutationFn: async (id: string): Promise<void> => {
      // The middleware throws for every error status; a 204 has no body to read.
      await api.fetch.POST("/admin/dead_jobs/{id}/retry", { params: { path: { id } } });
    },
  };
}

export type DomainAction = Schemas["AdminDomainAction"];
export type DomainActionKind = Schemas["AdminDomainActionKind"];
export type DomainActionState = Schemas["AdminDomainActionState"];
export type NewDomainAction = Schemas["CreateDomainActionRequest"];

/** The kinds of domain action, from the lightest to the heaviest, in the order of the form. */
export const domainActionKinds = ["force_sensitive", "reject_media", "reject_reports", "limit", "suspend"] as const satisfies readonly DomainActionKind[];

/** The most characters of a domain action's public comment and private note (CreateDomainActionRequest). */
export const domainActionTextLimit = 5000;

/** The filters of the list of domain actions, in the order of its tabs; `active` sends `active=true`. */
export const domainActionFilters = ["active", "all"] as const;

export type DomainActionFilter = (typeof domainActionFilters)[number];

/** Which domain actions a list shows: those in effect or all, of every domain or of one. */
export type DomainActionScope = { filter: DomainActionFilter; domain?: string | undefined };

/** The moderation actions on domains, newest first (GET /admin/domain_actions). */
export function domainActionsQueryOptions(api: Api, { filter, domain }: DomainActionScope) {
  const active = filter === "active" ? true : undefined;
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/domain_actions", { active, domain }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const query = { ...pageQuery(pageParam), ...(active === undefined ? {} : { active }), ...(domain === undefined ? {} : { domain }) };
      const { data, error } = await api.fetch.GET("/admin/domain_actions", { params: { query }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** Whether a query is one of the lists of domain actions, of any filter or domain. */
export function isDomainActionsQuery(queryKey: readonly unknown[]): boolean {
  return queryKey[1] === "/admin/domain_actions";
}

/** Whether a list of domain actions keeps only those in effect, which a lifted action leaves. */
export function isActiveOnlyQuery(queryKey: readonly unknown[]): boolean {
  const params = queryKey[2];
  return isDomainActionsQuery(queryKey) && typeof params === "object" && params !== null && "active" in params && params.active === true;
}

/** Takes a moderation action on a domain (POST /admin/domain_actions), as the options of useMutation. */
export function createDomainActionMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/domain_actions"] as const,
    mutationFn: async (body: NewDomainAction): Promise<DomainAction> => {
      const { data, error } = await api.fetch.POST("/admin/domain_actions", { body });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Lifts a moderation action on a domain (POST /admin/domain_actions/{id}/lift), as the options of useMutation. */
export function liftDomainActionMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/domain_actions/{id}/lift"] as const,
    mutationFn: async (id: string): Promise<DomainAction> => {
      const { data, error } = await api.fetch.POST("/admin/domain_actions/{id}/lift", { params: { path: { id } } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

export type AuditEntry = Schemas["AdminAuditEntry"];
export type AuditAction = Schemas["AdminAuditAction"];
export type AuditActorKind = Schemas["AdminAuditActorKind"];
export type AuditTargetKind = Schemas["AdminAuditTargetKind"];

/** The kinds of audited operation, grouped by the kind of their target, in the order of the filter. */
export const auditActions = [
  "account_action_issued",
  "account_action_lifted",
  "domain_action_issued",
  "domain_action_lifted",
  "report_assigned",
  "report_unassigned",
  "report_resolved",
  "report_reopened",
  "report_note_added",
  "registration_approved",
  "registration_rejected",
  "setting_changed",
  "text_changed",
  "rule_added",
  "rule_revised",
  "rule_moved",
  "rule_retired",
  "signup_block_added",
  "signup_block_lifted",
] as const satisfies readonly AuditAction[];

/** The kinds of target of an audited operation, in the order of the filter. */
export const auditTargetKinds = [
  "account",
  "status",
  "domain",
  "report",
  "appeal",
  "user",
  "role",
  "setting",
  "rule",
  "signup_block",
] as const satisfies readonly AuditTargetKind[];

/**
 * Which operations the audit log shows: those of one actor's account, of one kind, on one kind of target, and
 * on one target of that kind; each left undefined keeps every operation. `target` counts only with `targetKind`.
 */
export type AuditLogFilter = {
  actor?: string | undefined;
  action?: AuditAction | undefined;
  targetKind?: AuditTargetKind | undefined;
  target?: string | undefined;
};

/** The operations of the moderators and administrators under `filter`, newest first (GET /admin/audit_log). */
export function auditLogQueryOptions(api: Api, filter: AuditLogFilter) {
  const params = {
    actor_account_id: filter.actor,
    action: filter.action,
    target_kind: filter.targetKind,
    target_id: filter.targetKind === undefined ? undefined : filter.target,
  };
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/audit_log", params, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const query = {
        ...pageQuery(pageParam),
        ...(params.actor_account_id === undefined ? {} : { actor_account_id: params.actor_account_id }),
        ...(params.action === undefined ? {} : { action: params.action }),
        ...(params.target_kind === undefined ? {} : { target_kind: params.target_kind }),
        ...(params.target_id === undefined ? {} : { target_id: params.target_id }),
      };
      const { data, error } = await api.fetch.GET("/admin/audit_log", { params: { query }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/**
 * The most pages of the audit log that its address keeps (`?pages=`), 1,000 entries: the list reads them again
 * after a reload, and loading more than these keeps working without changing the address.
 */
export const auditLogMaxPages = 25;
