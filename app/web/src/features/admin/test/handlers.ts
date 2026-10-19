// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the admin feature (web-client.md §7): the health of remote servers, the domain
// actions, the sign-up blocks, the dead jobs, the audit log, the registrations awaiting approval, the reports, the accounts and their roles, all empty and every server available by default. The default viewer has no
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

/** A sign-up block on the email domain `mail.example` in effect since 2026-10-10, as the server answers it; `overrides` replace its fields. */
export function signupBlockFixture(overrides: Partial<Schemas["AdminSignupBlock"]> = {}): Schemas["AdminSignupBlock"] {
  return { id: "0192f0c4-7a8e-7000-8000-0000000001a1", kind: "email_domain", value: "mail.example", severity: "reject", state: "active", created_at: "2026-10-10T08:00:00Z", ...overrides };
}

/** The block that POST /admin/signup_blocks records for `body`, added at 2026-10-17T00:00:00Z; the value is kept as sent. */
export function newSignupBlock(body: Schemas["CreateSignupBlockRequest"]): Schemas["AdminSignupBlock"] {
  const createdAt = "2026-10-17T00:00:00Z";
  return {
    ...signupBlockFixture({ id: "0192f0c4-7a8e-7000-8000-0000000001af", kind: body.kind, value: body.value, severity: body.severity, created_at: createdAt }),
    ...(body.comment === undefined ? {} : { comment: body.comment }),
    ...(body.expires_in === undefined ? {} : { expires_at: new Date(Date.parse(createdAt) + body.expires_in * 1000).toISOString() }),
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

/** `account` as the moderators see it: active, without restrictions, local when it is on this server. */
export function adminAccountFixture(account: Schemas["Account"], overrides: Partial<Schemas["AdminAccount"]> = {}): Schemas["AdminAccount"] {
  const local = !account.acct.includes("@");
  return { id: account.id, account, local, state: "active", restrictions: [], ...(local ? { registration_state: "approved" } : {}), ...overrides };
}

/**
 * The details of `account` as GET /admin/accounts/{id} answers them: a local one registered on 2026-10-01 with
 * a verified email address and no roles, actions or reports; `overrides` replace its fields.
 */
export function adminAccountDetailFixture(account: Schemas["Account"], overrides: Partial<Schemas["AdminAccountDetail"]> = {}): Schemas["AdminAccountDetail"] {
  const summary = adminAccountFixture(account);
  return {
    ...summary,
    ...(summary.local ? { registration: { submitted_at: "2026-10-01T00:00:00Z", state_changed_at: "2026-10-01T01:00:00Z" }, email_verified: true } : {}),
    roles: [],
    actions: [],
    reports: [],
    ...overrides,
  };
}

/** What POST /admin/accounts/{id}/reveal answers by default: an email address, no IP addresses kept. */
export const privateDataFixture: Schemas["AdminAccountPrivateData"] = { email: "bob@example.com", email_verified: true, sign_ins: [], same_ip_accounts: [] };

/** A moderation action `kind` on Bob in effect since 2026-10-10, as the server answers it; `overrides` replace its fields. */
export function accountActionFixture(overrides: Partial<Schemas["AdminAccountAction"]> = {}): Schemas["AdminAccountAction"] {
  return {
    id: "0192f0c4-7a8e-7000-8000-0000000d00a1",
    account_id: otherAccountFixture.id,
    kind: "limit",
    reason_category: "spam",
    issuer: "moderator",
    issued_by_account_id: viewerFixture.id,
    notify_target: true,
    state: "active",
    issued_at: "2026-10-10T08:00:00Z",
    ...overrides,
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

/** The roles that every instance has (GET /admin/roles), highest first: admin with every permission, and moderator. */
export const adminRoleFixture: Schemas["AdminRole"] = {
  id: "0192f0c4-7a8e-7000-8000-000000000a01",
  key: "admin",
  position: 1000,
  permissions: ["manage_reports", "manage_accounts", "manage_domains", "manage_rules", "manage_settings", "manage_roles", "manage_invites", "view_audit", "manage_jobs"],
};
export const moderatorRoleFixture: Schemas["AdminRole"] = {
  id: "0192f0c4-7a8e-7000-8000-000000000a02",
  key: "moderator",
  position: 100,
  permissions: ["manage_reports", "manage_accounts", "view_audit"],
};
export const rolesFixture: Schemas["AdminRole"][] = [adminRoleFixture, moderatorRoleFixture];

/** An instance text as the server answers it, saved at 2026-10-17T00:00:00Z. */
export function instanceTextFixture(key: Schemas["InstanceTextKey"], locale: string, body: string): Schemas["AdminInstanceText"] {
  return { key, locale, body, revised_at: "2026-10-17T00:00:00Z" };
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
  // operationIds listSignupBlocks, createSignupBlock, getSignupBlock, liftSignupBlock. A new block keeps the value
  // as sent (the real server keeps it in canonical form).
  http.get(`${apiBase}/admin/signup_blocks`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/signup_blocks`, async ({ request }) => {
    const body = (await request.clone().json()) as Schemas["CreateSignupBlockRequest"];
    return withViewer(request, () => HttpResponse.json(newSignupBlock(body)));
  }),
  http.get(`${apiBase}/admin/signup_blocks/:id`, ({ request, params }) => withViewer(request, () => HttpResponse.json(signupBlockFixture({ id: String(params["id"]) })))),
  http.post(`${apiBase}/admin/signup_blocks/:id/lift`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(signupBlockFixture({ id: String(params["id"]), state: "lifted", lifted_at: "2026-10-17T00:00:00Z" }))),
  ),
  // operationId importSignupBlocks: every value of the list is added.
  http.post(`${apiBase}/admin/signup_blocks/import`, async ({ request }) => {
    const body = (await request.clone().json()) as Schemas["ImportSignupBlocksRequest"];
    const added = body.text.split("\n").filter((line) => line.trim() !== "" && !line.trim().startsWith("#")).length;
    return withViewer(request, () => HttpResponse.json({ added, skipped: [] } satisfies Schemas["SignupBlockImportResult"]));
  }),
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
  // operationIds listAdminAccounts, getAdminAccount, revealAdminAccount: no accounts found; Bob, local, for any
  // ID; no private data but an email address.
  http.get(`${apiBase}/admin/accounts`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.get(`${apiBase}/admin/accounts/:id`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(adminAccountDetailFixture({ ...otherAccountFixture, id: String(params["id"]) }))),
  ),
  http.post(`${apiBase}/admin/accounts/:id/reveal`, ({ request }) => withViewer(request, () => HttpResponse.json(privateDataFixture))),
  // operationIds listRoles, listAccountRoles, grantAccountRole, revokeAccountRole: the instance's two roles; the
  // viewer holds admin and every other account none; a change answers the role granted, or none after a revoke.
  http.get(`${apiBase}/admin/roles`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: rolesFixture }))),
  http.get(`${apiBase}/admin/accounts/:id/roles`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json({ items: params["id"] === viewerFixture.id ? [adminRoleFixture] : [] })),
  ),
  http.put(`${apiBase}/admin/accounts/:id/roles/:role`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json({ items: rolesFixture.filter((role) => role.key === params["role"]) })),
  ),
  http.delete(`${apiBase}/admin/accounts/:id/roles/:role`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  // operationIds getAccountAction, liftAccountAction.
  http.get(`${apiBase}/admin/account_actions/:id`, ({ request, params }) => withViewer(request, () => HttpResponse.json(accountActionFixture({ id: String(params["id"]) })))),
  http.post(`${apiBase}/admin/account_actions/:id/lift`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["LiftAccountActionRequest"];
    return withViewer(request, () =>
      HttpResponse.json(
        accountActionFixture({
          id: String(params["id"]),
          state: "lifted",
          lifted_at: "2026-10-17T00:00:00Z",
          ...(body.reason === undefined ? {} : { lift_reason: body.reason }),
        }),
      ),
    );
  }),
  // operationIds listInstanceTexts, setInstanceText: no texts; a new text is recorded as sent, at 2026-10-17T00:00:00Z.
  http.get(`${apiBase}/admin/instance_texts`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.put(`${apiBase}/admin/instance_texts/:key/:locale`, async ({ request, params }) => {
    const body = (await request.clone().json()) as Schemas["SetInstanceTextRequest"];
    return withViewer(request, () => HttpResponse.json(instanceTextFixture(String(params["key"]) as Schemas["InstanceTextKey"], String(params["locale"]), body.body)));
  }),
  // operationId getAccount: the moderators of the reports, Bob for any other ID. The path also matches the
  // operations under /accounts with a name (lookup, relationships…), which go on to their own handlers.
  http.get(`${apiBase}/accounts/:id`, ({ request, params }) => {
    const id = String(params["id"]);
    if (!isId(id)) return undefined;
    return withViewer(request, () => HttpResponse.json(id === viewerFixture.id ? viewerFixture : { ...otherAccountFixture, id }));
  }),
];
