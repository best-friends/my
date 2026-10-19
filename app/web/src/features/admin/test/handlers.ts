// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the admin feature (web-client.md §7): the health of remote servers, the domain
// actions, the dead jobs, the audit log and the registrations awaiting approval, all empty and every server available by default. The default viewer has no
// permissions, so the admin screens' tests start with `server.use(operatorHandler(...))`.
import { http, HttpResponse } from "msw";
import { type Schemas, viewerFixture } from "../../../test/fixtures";
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

export const handlers = [
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
];
