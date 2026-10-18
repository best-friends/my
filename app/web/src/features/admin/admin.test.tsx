// SPDX-License-Identifier: AGPL-3.0-only
// The admin screens through the whole app (web-client.md T4): a signed-in session from the default handlers,
// a viewer whose roles have the screen's permission (or not), and MSW playing the admin API.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import type { Schemas } from "../../test/fixtures";
import { apiBase, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { operatorHandler } from "./test/handlers";

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

const paused: Schemas["AdminInstanceHealth"] = { domain: "down.example", state: "unavailable", since: "2026-10-10T08:00:00Z", failure_days: 7, last_failure_day: "2026-10-10" };
const flaky: Schemas["AdminInstanceHealth"] = { domain: "flaky.example", state: "available", failure_days: 2, last_failure_day: "2026-10-16" };

describe("the administration menu", () => {
  test("is not on the settings page of a user without admin permissions", async () => {
    await renderApp({ path: "/settings" });

    expect(await screen.findByRole("region", { name: "Settings pages" })).toBeInTheDocument();
    // The account section loads with the viewer; the menu would be shown by then.
    expect(await within(screen.getByRole("main")).findByText("@alice@my.example")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Administration" })).not.toBeInTheDocument();
  });

  test("links to the screens whose permissions the user's roles have", async () => {
    server.use(operatorHandler(["manage_jobs"]));
    await renderApp({ path: "/settings" });

    const menu = await screen.findByRole("region", { name: "Administration" });
    expect(within(menu).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/admin/dead-jobs"]);
    expect(within(menu).getByRole("link", { name: "Failed jobs" })).toHaveAccessibleDescription("Background jobs that failed for good, and running them again.");
  });
});

describe("remote servers", () => {
  test("lists the servers with their state and failures, filters them by state in the URL, and loads more", async () => {
    server.use(
      operatorHandler(["manage_domains"]),
      http.get(`${apiBase}/admin/instances`, ({ request }) => {
        const query = new URL(request.url).searchParams;
        if (query.get("state") === "unavailable") return HttpResponse.json({ items: [paused] });
        return query.get("cursor") === "next" ? HttpResponse.json({ items: [flaky] }) : HttpResponse.json({ items: [paused], next_cursor: "next" });
      }),
    );
    const requests = recordAdminRequests();
    const { user, router } = await renderApp({ path: "/admin/instances" });

    expect(await screen.findByRole("heading", { level: 1, name: "Remote servers" })).toBeInTheDocument();
    const [row] = within(screen.getByRole("tabpanel")).getAllByRole("listitem");
    if (row === undefined) throw new Error("no server is listed");
    const details = within(row);
    expect(details.getByRole("heading", { level: 2, name: "down.example" })).toBeInTheDocument();
    expect(details.getByRole("link", { name: "down.example" })).toHaveAttribute("href", "/admin/instances/down.example");
    expect(details.getByText("Paused")).toBeInTheDocument();
    expect(details.getByText("7 days")).toBeInTheDocument();
    expect(details.getByText("Oct 10, 2026")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show more" }));
    expect(await screen.findByRole("heading", { level: 2, name: "flaky.example" })).toBeInTheDocument();
    expect(screen.getByText("2 days")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Paused" }));
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ state: "unavailable" });
    });
    expect(screen.getByRole("tab", { name: "Paused" })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("heading", { level: 2, name: "down.example" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 2, name: "flaky.example" })).not.toBeInTheDocument();
    expect(requests).toEqual(["GET /admin/instances?limit=40", "GET /admin/instances?limit=40&cursor=next", "GET /admin/instances?limit=40&state=unavailable"]);
  });

  test("says when no server has failed deliveries", async () => {
    server.use(operatorHandler(["manage_domains"]));
    await renderApp({ path: "/admin/instances" });

    expect(await screen.findByRole("heading", { level: 2, name: "No servers with failed deliveries" })).toBeInTheDocument();
  });

  test("a server's page shows why it is paused and its history, and resumes deliveries", async () => {
    let health = paused;
    let events: Schemas["AdminAvailabilityEvent"][] = [
      { id: "0192f0c4-7a8e-7000-8000-0000000000e2", kind: "marked_unavailable", reason: "failure_days", occurred_at: "2026-10-10T08:00:00Z" },
      { id: "0192f0c4-7a8e-7000-8000-0000000000e1", kind: "marked_available", reason: "probe_succeeded", occurred_at: "2026-09-01T00:00:00Z" },
    ];
    server.use(
      operatorHandler(["manage_domains"]),
      http.get(`${apiBase}/admin/instances/:domain`, () => HttpResponse.json(health)),
      http.get(`${apiBase}/admin/instances/:domain/availability_events`, () => HttpResponse.json({ items: events })),
      http.post(`${apiBase}/admin/instances/:domain/mark_available`, () => {
        health = { domain: paused.domain, state: "available", since: "2026-10-17T00:00:00Z", failure_days: 0 };
        events = [{ id: "0192f0c4-7a8e-7000-8000-0000000000e3", kind: "marked_available", reason: "operator", occurred_at: "2026-10-17T00:00:00Z" }, ...events];
        return HttpResponse.json(health);
      }),
    );
    const requests = recordAdminRequests();
    const { user } = await renderApp({ path: "/admin/instances/down.example" });

    expect(await screen.findByRole("heading", { level: 1, name: "down.example" })).toBeInTheDocument();
    expect(screen.getByText("Paused")).toBeInTheDocument();
    // The reason of the newest event, and in the history.
    expect(screen.getAllByText("Deliveries failed on 7 different days.")).toHaveLength(2);
    const history = screen.getByRole("region", { name: "History" });
    expect(within(history).getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent)).toEqual(["Deliveries paused", "Deliveries resumed"]);
    expect(within(history).getByText("The daily check reached the server.")).toBeInTheDocument();

    const resume = screen.getByRole("button", { name: "Resume deliveries" });
    expect(resume).toHaveAccessibleDescription("Clears the failed days, so that the server gets every delivery again.");
    await user.click(resume);

    expect(await screen.findByText("Deliveries to down.example resume.")).toBeInTheDocument();
    expect(screen.getByText("Delivering")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Resume deliveries" })).not.toBeInTheDocument();
    expect(await within(history).findByText("An operator resumed deliveries.")).toBeInTheDocument();
    expect(requests).toContain("POST /admin/instances/down.example/mark_available");
  });

  test("a failure to resume deliveries keeps the server's state and says why", async () => {
    server.use(
      operatorHandler(["manage_domains"]),
      http.get(`${apiBase}/admin/instances/:domain`, () => HttpResponse.json(paused)),
      http.post(`${apiBase}/admin/instances/:domain/mark_available`, () => problem(503, "about:blank", "Service Unavailable")),
    );
    const { user } = await renderApp({ path: "/admin/instances/down.example" });

    await user.click(await screen.findByRole("button", { name: "Resume deliveries" }));

    expect(await screen.findByText("Could not resume deliveries: Service Unavailable")).toBeInTheDocument();
    expect(screen.getByText("Paused")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resume deliveries" })).toBeInTheDocument();
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

  test("a refusal of the server for a missing permission is shown as such", async () => {
    server.use(
      operatorHandler(["manage_domains"]),
      http.get(`${apiBase}/admin/instances`, () => problem(403, "/api/v1/problems/missing-permission", "Forbidden")),
    );
    await renderApp({ path: "/admin/instances" });

    expect(await screen.findByRole("heading", { level: 2, name: "You cannot use this screen" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Remote servers" })).toBeInTheDocument();
  });

  test("a signed-out user is sent to sign in first", async () => {
    server.resetHandlers();
    const { router } = await renderApp({ path: "/admin/instances" });

    expect(await screen.findByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
    expect(router.state.location.search).toEqual({ return_to: "/admin/instances" });
  });
});
