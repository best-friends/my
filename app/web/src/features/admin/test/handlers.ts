// SPDX-License-Identifier: AGPL-3.0-only
// The default MSW handlers of the admin feature (web-client.md §7): the health of remote servers and the dead
// jobs, all empty and every server available by default. The default viewer has no permissions, so the
// admin screens' tests start with `server.use(operatorHandler(...))`.
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

export const handlers = [
  // operationIds listInstanceHealth, getInstanceHealth, listInstanceAvailabilityEvents, markInstanceAvailable.
  http.get(`${apiBase}/admin/instances`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.get(`${apiBase}/admin/instances/:domain`, ({ request, params }) => withViewer(request, () => HttpResponse.json(availableHealth(String(params["domain"]))))),
  http.get(`${apiBase}/admin/instances/:domain/availability_events`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/instances/:domain/mark_available`, ({ request, params }) =>
    withViewer(request, () => HttpResponse.json(availableHealth(String(params["domain"])))),
  ),
  // operationIds listDeadJobs, retryDeadJob.
  http.get(`${apiBase}/admin/dead_jobs`, ({ request }) => withViewer(request, () => HttpResponse.json({ items: [] }))),
  http.post(`${apiBase}/admin/dead_jobs/:id/retry`, ({ request }) => withViewer(request, () => new HttpResponse(null, { status: 204 }))),
];
