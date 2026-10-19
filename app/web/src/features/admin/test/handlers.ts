// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the admin feature (web-client.md §7): the health of remote servers, the domain
// actions, the dead jobs, the audit log, the registrations awaiting approval and the reports, all empty and every server available by default. The default viewer has no
// permissions, so the admin screens' tests start with `server.use(operatorHandler(...))`.
import { http, HttpResponse } from "msw";
import { otherAccountFixture, remoteAccountFixture, type Schemas, statusFixture, viewerFixture } from "../../../test/fixtures";
import { apiBase, withViewer } from "../../../test/msw/http";

/** The viewer's account with the permissions `permissions`, for GET /accounts/verify_credentials. */
export function operatorHandler(permissions: Schemas["Permission"][]) {
  return http.get(`${apiBase}/accounts/verify_credentials`, ({ request }) => withViewer(request, () => HttpResponse.json({ ...viewerFixture, permissions })));
}

/** The health of a server without failures, as GET /admin/instances/{domain} answers for it. */
export function availableHealth(domain: string): Schemas["AdminInstanceHealth"] {
  return { domain, state: "available", failure_days: 0 };
}

/** A domain action on `domain` in effect since 2026-10-10, as the server answers it; `overrides` replace its fields. */
export function domainActionFixture(overrides: Partial<Schemas["AdminDomainAction"]> = {}): Schemas["AdminDomainAction"] {
  return { id: "0192f0c4-7a8e-7000-8000-0000000000a1", domain: "spam.example", kind: "limit", obfuscate: false, state: "active", issued_at: "2026-10-10T08:00:00Z", ...overrides };
}

/** The action that POST /admin/domain_actions records for `body`, taken at 2026-10-17T00:00:00Z. */
export function newDomainAction(body: Schemas["CreateDomainActionRequest"]): Schemas["AdminDomainAction"] {
  const issuedAt = "2026-10-17T00:00:00Z";
  return {
    ...domainActionFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000af", domain: body.domain, kind: body.kind, obfuscate: body.obfuscate, issued_at: issuedAt }),
    ...(body.public_comment === undefined ? {} : { public_comment: body.public_comment }),
    ...(body.private_note === undefined ? {} : { private_note: body.private_note }),
    ...(body.expires_in === undefined ? {} : { expires_at: new Date(Date.parse(issuedAt) + body.expires_in * 1000).toISOString() }),
  };
}

/** A registration of `bob` awaiting approval since 2026-10-16, as the server lists it; `overrides` replace its fields. */
export function registrationFixture(overrides: Partial<Schemas["AdminRegistration"]> = {}): Schemas["AdminRegistration"] {
  return {
    user_id: "0192f0c4-7a8e-7000-8000-0000000000b1",
    account_id: "0192f0c4-7a8e-7000-8000-0000000000c1",
    username: "bob",
    state: "pending",
    since: "2026-10-16T09:00:00Z",
    email: "bob@example.com",
    email_verified: true,
    reason: "I would like to post about birds.",
    submitted_at: "2026-10-16T09:00:00Z",
    ...overrides,
  };
}

/** GET /admin/counts answering `counts`, the counts that the viewer's permissions allow. */
export function countsHandler(counts: Schemas["AdminCounts"]) {
  return http.get(`${apiBase}/admin/counts`, ({ request }) => withViewer(request, () => HttpResponse.json(counts)));
}

/**
 * An open report by Carol of another server on Bob, filed on 2026-10-16 with one post and no notes, as the server
 * answers it; `overrides` replace its fields.
 */
export function reportFixture(overrides: Partial<Schemas["AdminReportDetail"]> = {}): Schemas["AdminReportDetail"] {
  const statusIds = overrides.status_ids ?? ["0192f0c4-7a8e-7000-8000-0000000000b1"];
  return {
    id: "0192f0c4-7a8e-7000-8000-0000000c0001",
    category: "spam",
    comment: "Posts the same link again and again.",
    forwarded: false,
    filed_at: "2026-10-16T09:00:00Z",
    state: "open",
    state_changed_at: "2026-10-16T09:00:00Z",
    reporter_account_id: remoteAccountFixture.id,
    reporter_account: remoteAccountFixture,
    target_account_id: otherAccountFixture.id,
    target_account: otherAccountFixture,
    status_ids: statusIds,
    rule_ids: [],
    statuses: statusIds.map((id) => ({ status: statusFixture(id) })),
    notes: [],
    target_history: { reports_count: 0, actions_count: 0 },
    ...overrides,
  };
}

/** The action that POST /admin/accounts/{id}/actions records for `body` on account `accountId`, taken at 2026-10-17T00:00:00Z. */
export function newAccountAction(accountId: string, body: Schemas["CreateAccountActionRequest"]): Schemas["AdminAccountAction"] {
  const issuedAt = "2026-10-17T00:00:00Z";
  return {
    id: "0192f0c4-7a8e-7000-8000-0000000d0001",
    account_id: accountId,
    kind: body.kind,
    reason_category: body.reason_category,
    issuer: "moderator",
    issued_by_account_id: viewerFixture.id,
    notify_target: body.notify_target,
    state: "active",
    issued_at: issuedAt,
    ...(body.public_reason === undefined ? {} : { public_reason: body.public_reason }),
    ...(body.private_note === undefined ? {} : { private_note: body.private_note }),
    ...(body.report_id === undefined ? {} : { report_id: body.report_id }),
    ...(body.status_ids === undefined ? {} : { status_ids: body.status_ids }),
    ...(body.expires_in === undefined ? {} : { expires_at: new Date(Date.parse(issuedAt) + body.expires_in * 1000).toISOString() }),
  };
}

/**
 * A pending appeal by Bob, made on 2026-10-16, of an active suspension that a moderator took on 2026-10-15, as
 * the server answers it; `overrides` replace its fields.
 */
export function appealFixture(overrides: Partial<Schemas["AdminAppeal"]> = {}): Schemas["AdminAppeal"] {
  return {
    id: "0192f0c4-7a8e-7000-8000-0000000f0001",
    account_id: otherAccountFixture.id,
    text: "I posted the link once, to my own blog.",
    state: "pending",
    submitted_at: "2026-10-16T09:00:00Z",
    action: {
      id: "0192f0c4-7a8e-7000-8000-0000000d0101",
      account_id: otherAccountFixture.id,
      kind: "suspend",
      reason_category: "spam",
      public_reason: "Repeated advertising.",
      private_note: "Third report this week.",
      issuer: "moderator",
      issued_by_account_id: viewerFixture.id,
      notify_target: true,
      state: "active",
      issued_at: "2026-10-15T09:00:00Z",
    },
    ...overrides,
  };
}

/** The appeal `appeal` after `decision` at 2026-10-17T00:00:00Z by the viewer; an overturn lifts an active action. */
export function decidedAppeal(appeal: Schemas["AdminAppeal"], decision: "uphold" | "overturn", reason: string | undefined): Schemas["AdminAppeal"] {
  const decidedAt = "2026-10-17T00:00:00Z";
  const action = decision === "overturn" && appeal.action.state === "active" ? { ...appeal.action, state: "lifted" as const, lifted_at: decidedAt } : appeal.action;
  return {
    ...appeal,
    state: decision === "uphold" ? "upheld" : "overturned",
    decided_at: decidedAt,
    decided_by_account_id: viewerFixture.id,
    action,
    ...(reason === undefined ? {} : { decision_reason: reason }),
  };
}

/** Whether a path segment is an ID of the API (a UUID), not the name of another operation under the same path. */
function isId(segment: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(segment);
}

export const handlers = [
  // operationId getAdminCounts: nothing awaits the moderators.
  countsHandler({}),
  // operationIds listInstanceHealth, getInstanceHealth, listInstanceAvailabilityEvents, markInstanceAvailable.
  http.get(`${apiBase}/admin/instances`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.get(`${apiBase}/admin/instances/:domain`, ({ request, params }) => withViewer(request, () => HttpResponse.json(availableHealth(String(params["domain"]))))),
  http.get(`${apiBase}/admin/instances/:domain/availability_events`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/instances/:domain/mark_available`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(availableHealth(String(params["domain"])))),
  ),
  // operationIds listDomainActions, createDomainAction, getDomainAction, liftDomainAction. A new action keeps the
  // domain as sent (the real server converts U-labels to A-labels).
  http.get(`${apiBase}/admin/domain_actions`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/domain_actions`, async ({ request }) => {
    const body = (await request.clone().json()) as Schemas["CreateDomainActionRequest"];
    return withViewer(request, () => HttpResponse.json(newDomainAction(body)));
  }),
  http.get(`${apiBase}/admin/domain_actions/:id`, ({ request, params }) => withViewer(request, () => HttpResponse.json(domainActionFixture({ id: String(params["id"]) })))),
  http.post(`${apiBase}/admin/domain_actions/:id/lift`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(domainActionFixture({ id: String(params["id"]), state: "lifted", lifted_at: "2026-10-17T00:00:00Z" }))),
  ),
  // operationIds listDeadJobs, retryDeadJob.
  http.get(`${apiBase}/admin/dead_jobs`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/dead_jobs/:id/retry`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
  // operationId listAuditLog.
  http.get(`${apiBase}/admin/audit_log`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  // operationIds listPendingRegistrations, approveRegistration, rejectRegistration.
  http.get(`${apiBase}/admin/registrations`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/registrations/:userId/approve`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(registrationFixture({ user_id: String(params["userId"]), state: "approved", since: "2026-10-17T00:00:00Z" }))),
  ),
  http.post(`${apiBase}/admin/registrations/:userId/reject`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(registrationFixture({ user_id: String(params["userId"]), state: "rejected", since: "2026-10-17T00:00:00Z" }))),
  ),
  // operationIds listReports, getReport, assignReportToSelf, unassignReport, resolveReport, addReportNote, reopenReport.
  http.get(`${apiBase}/admin/reports`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.get(`${apiBase}/admin/reports/:id`, ({ request, params }) => withViewer(request, () => HttpResponse.json(reportFixture({ id: String(params["id"]) })))),
  http.post(`${apiBase}/admin/reports/:id/assign_to_self`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(reportFixture({ id: String(params["id"]), assignee_account_id: viewerFixture.id }))),
  ),
  http.post(`${apiBase}/admin/reports/:id/unassign`, ({ request, params }) => withViewer(request, () => HttpResponse.json(reportFixture({ id: String(params["id"]) })))),
  http.post(`${apiBase}/admin/reports/:id/resolve`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["ResolveReportRequest"];
    return withViewer(request, () =>
      HttpResponse.json(reportFixture({ id: String(params["id"]), state: "resolved", outcome: body.outcome, state_changed_at: "2026-10-17T00:00:00Z" })),
    );
  }),
  http.post(`${apiBase}/admin/reports/:id/notes`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["CreateReportNoteRequest"];
    return withViewer(request, () =>
      HttpResponse.json(
        reportFixture({
          id: String(params["id"]),
          notes: [{ id: "0192f0c4-7a8e-7000-8000-0000000e0001", author_account_id: viewerFixture.id, body: body.body, created_at: "2026-10-17T00:00:00Z" }],
        }),
      ),
    );
  }),
  http.post(`${apiBase}/admin/reports/:id/reopen`, ({ request, params }) => withViewer(request, () => HttpResponse.json(reportFixture({ id: String(params["id"]) })))),
  // operationIds listAppeals, getAppeal, upholdAppeal, overturnAppeal.
  http.get(`${apiBase}/admin/appeals`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.get(`${apiBase}/admin/appeals/:id`, ({ request, params }) => withViewer(request, () => HttpResponse.json(appealFixture({ id: String(params["id"]) })))),
  ...(["uphold", "overturn"] as const).map((decision) =>
    http.post(`${apiBase}/admin/appeals/:id/${decision}`, async ({ request, params }) => {
      const body = (await request.clone().json()) as Schemas["DecideAppealRequest"];
      return withViewer(request, () => HttpResponse.json(decidedAppeal(appealFixture({ id: String(params["id"]) }), decision, body.public_reason)));
    }),
  ),
  // operationIds listAccountActions, createAccountAction.
  http.get(`${apiBase}/admin/accounts/:id/actions`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/accounts/:id/actions`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["CreateAccountActionRequest"];
    return withViewer(request, () => HttpResponse.json(newAccountAction(String(params["id"]), body)));
  }),
  // operationId getAccount: the moderators of the reports, Bob for any other ID. The path also matches the
  // operations under /accounts with a name (lookup, relationships…), which go on to their own handlers.
  http.get(`${apiBase}/accounts/:id`, ({ request, params }) => {
    const id = String(params["id"]);
    if (!isId(id)) return undefined;
    return withViewer(request, () => HttpResponse.json(id === viewerFixture.id ? viewerFixture : { ...otherAccountFixture, id }));
  }),
];
