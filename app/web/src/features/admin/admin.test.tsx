// SPDX-License-Identifier: AGPL-3.0-only
// The server administration screens and the staff's menus through the whole app (web-client.md T4, ADR-0058): a
// signed-in session from the default handlers, a viewer whose roles have the screen's permission (or not), and
// MSW playing the admin API.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import type { Schemas } from "../../test/fixtures";
import { apiBase, operatorHandler, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

beforeEach(() => {
  server.use(signedInHandler);
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

/** The admin requests the app sent, as "METHOD path?query". */
function recordAdminRequests() {
  const requests: string[] = [];
  server.events.on("request:start", ({ request }) => {
    const url = new URL(request.url);
    if (url.pathname.startsWith(`${apiBase}/admin/`)) requests.push(`${request.method} ${url.pathname.slice(apiBase.length)}${url.search}`);
  });
  return requests;
}

describe("the staff's menus on the settings page", () => {
  test("are not there for a user without staff permissions", async () => {
    await renderApp({ path: "/settings" });

    expect(await screen.findByRole("region", { name: "Settings pages" })).toBeInTheDocument();
    // The account section loads with the viewer; the menus would be shown by then.
    expect(await within(screen.getByRole("main")).findByText("@alice")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Moderation" })).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Administration" })).not.toBeInTheDocument();
  });

  test("Administration links to the server administration screens whose permissions the user's roles have", async () => {
    server.use(operatorHandler(["manage_jobs"]));
    await renderApp({ path: "/settings" });

    const menu = await screen.findByRole("region", { name: "Administration" });
    expect(within(menu).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/admin/dead-jobs"]);
    expect(within(menu).getByRole("link", { name: "Failed jobs" })).toHaveAccessibleDescription("Background jobs that failed for good, and running them again.");
    expect(screen.queryByRole("region", { name: "Moderation" })).not.toBeInTheDocument();
  });

  test("a moderator has Moderation alone, with the screens of their permissions", async () => {
    server.use(operatorHandler(["manage_reports", "manage_accounts", "view_audit"]));
    await renderApp({ path: "/settings" });

    const menu = await screen.findByRole("region", { name: "Moderation" });
    expect(within(menu).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      "/moderation/reports",
      "/moderation/appeals",
      "/moderation/accounts",
      "/moderation/registrations",
      "/moderation/signup-blocks",
      "/moderation/audit-log",
    ]);
    expect(within(menu).getByRole("link", { name: "Audit log" })).toHaveAccessibleDescription("Who did what in the administration, and when.");
    expect(within(menu).getByRole("link", { name: "Registrations" })).toHaveAccessibleDescription("Sign-ups that await approval, and approving or rejecting them.");
    expect(within(menu).getByRole("link", { name: "Appeals" })).toHaveAccessibleDescription("Review appeals of moderation actions and uphold or overturn them.");
    expect(screen.queryByRole("region", { name: "Administration" })).not.toBeInTheDocument();
  });

  test("an administrator has both, Moderation first", async () => {
    server.use(
      operatorHandler(["manage_reports", "manage_accounts", "manage_domains", "manage_rules", "manage_settings", "manage_roles", "manage_invites", "view_audit", "manage_jobs"]),
    );
    await renderApp({ path: "/settings" });

    const moderation = await screen.findByRole("region", { name: "Moderation" });
    const admin = screen.getByRole("region", { name: "Administration" });
    expect(moderation.compareDocumentPosition(admin) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(moderation).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      "/moderation/reports",
      "/moderation/appeals",
      "/moderation/accounts",
      "/moderation/registrations",
      "/moderation/signup-blocks",
      "/moderation/domain-actions",
      "/moderation/instances",
      "/moderation/audit-log",
    ]);
    expect(within(admin).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/admin/settings", "/admin/documents", "/admin/dead-jobs"]);
  });
});

describe("failed jobs", () => {
  const deliver: Schemas["AdminDeadJob"] = {
    id: "0192f0c4-7a8e-7000-8000-0000000000d1",
    type: "federation.deliver",
    payload: '{"delivery_id":"x"}',
    idempotency_key: "deliver:x",
    attempts: 9,
    last_error: "502 Bad Gateway",
    created_at: "2026-10-13T10:00:00Z",
    failed_at: "2026-10-17T10:00:00Z",
  };
  const cleanup: Schemas["AdminDeadJob"] = {
    id: "0192f0c4-7a8e-7000-8000-0000000000d0",
    type: "kv.cleanup",
    payload: "null",
    attempts: 3,
    last_error: "db down",
    created_at: "2026-10-16T10:00:00Z",
    failed_at: "2026-10-16T11:00:00Z",
  };

  test("runs jobs again: each leaves the list, focus moves to the next one, and the result is announced", async () => {
    let dead = [deliver, cleanup];
    const retried: string[] = [];
    server.use(
      operatorHandler(["manage_jobs"]),
      http.get(`${apiBase}/admin/dead_jobs`, () => HttpResponse.json({ items: dead })),
      http.post(`${apiBase}/admin/dead_jobs/:id/retry`, ({ params }) => {
        const id = String(params["id"]);
        retried.push(id);
        dead = dead.filter((job) => job.id !== id);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = await renderApp({ path: "/admin/dead-jobs" });

    expect(await screen.findByRole("heading", { level: 1, name: "Failed jobs" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "federation.deliver" })).toBeInTheDocument();
    expect(screen.getByText("502 Bad Gateway")).toBeInTheDocument();
    expect(screen.getByText("deliver:x")).toBeInTheDocument();
    expect(screen.getByText('{"delivery_id":"x"}')).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^Run again: federation\.deliver, failed Oct 17, 2026/ }));

    await waitFor(() => {
      expect(screen.queryByRole("heading", { level: 2, name: "federation.deliver" })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /^Run again: kv\.cleanup/ })).toHaveFocus();
    expect(await screen.findByText("The job federation.deliver is queued to run again.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^Run again: kv\.cleanup/ }));
    expect(await screen.findByRole("heading", { level: 2, name: "No failed jobs" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveFocus();
    expect(retried).toEqual([deliver.id, cleanup.id]);
  });

  test("a job that cannot be run again stays, and the failure says why", async () => {
    server.use(
      operatorHandler(["manage_jobs"]),
      http.get(`${apiBase}/admin/dead_jobs`, () => HttpResponse.json({ items: [deliver] })),
      http.post(`${apiBase}/admin/dead_jobs/:id/retry`, () => problem(404, "about:blank", "Not Found")),
    );
    const { user } = await renderApp({ path: "/admin/dead-jobs" });

    await user.click(await screen.findByRole("button", { name: /^Run again: federation\.deliver/ }));

    expect(await screen.findByText("Could not run the job again: Not Found")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "federation.deliver" })).toBeInTheDocument();
  });
});

describe("permissions", () => {
  test("a screen says why a user without its permission cannot use it, and asks the server for nothing", async () => {
    server.use(operatorHandler(["manage_domains"]));
    const requests = recordAdminRequests();
    await renderApp({ path: "/admin/dead-jobs" });

    expect(await screen.findByRole("heading", { level: 2, name: "You cannot use this screen" })).toBeInTheDocument();
    expect(screen.getByText(/the permission manage_jobs/)).toBeInTheDocument();
    expect(requests).toEqual([]);
  });
});
