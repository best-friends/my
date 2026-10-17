// SPDX-License-Identifier: AGPL-3.0-only
// The notifications page and the unread badge through the whole app (web-client.md T4): the real router
// and QueryClient, with MSW as the server (FR-205).
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { apiBase, problem, resumableSession } from "../../test/msw/http";
import { notificationsFixture } from "./test/handlers";
import { otherAccountFixture, statusFixture } from "../../test/fixtures";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import type { Notification, NotificationMarker, NotificationType } from "./api/notifications";


/** The server's read marker as a variable, with the updates it received. */
function markerServer(initial: NotificationMarker) {
  let marker = initial;
  const updates: string[] = [];
  server.use(
    http.get(`${apiBase}/notifications/marker`, () => HttpResponse.json(marker)),
    http.put(`${apiBase}/notifications/marker`, async ({ request }) => {
      const { last_read_id: id } = (await request.json()) as { last_read_id: string };
      updates.push(id);
      marker = { last_read_id: id, unread_count: 0 };
      return HttpResponse.json(marker);
    }),
  );
  return { updates };
}

/** The link of the notifications page in the main navigation. */
function notificationsLink() {
  return within(screen.getByRole("navigation", { name: "Main" })).getByRole("link", { name: /^Notifications/ });
}

describe("the unread badge", () => {
  test("reads the unread count as part of the navigation link", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-000000000001", unread_count: 3 });

    await renderApp({ path: "/" });

    expect(await screen.findByRole("link", { name: "Notifications, 3 unread" })).toBeInTheDocument();
  });

  test("is absent when everything is read and while signed out", async () => {
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-000000000001", unread_count: 3 });

    await renderApp({ path: "/" });

    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    expect(notificationsLink()).toHaveAccessibleName("Notifications");
  });
});

describe("the notifications page", () => {
  test("shows each type, marks the newest read, and clears the badge", async () => {
    server.use(resumableSession());
    const marker = markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a1", unread_count: 1 });
    // The posts come embedded in the notifications; none is fetched on its own.
    const statusRequests: string[] = [];
    server.use(
      http.get(`${apiBase}/statuses/:id`, ({ request }) => {
        statusRequests.push(request.url);
        return new HttpResponse(null, { status: 404 });
      }),
    );

    await renderApp({ path: "/notifications" });

    const feed = await screen.findByRole("feed", { name: "Notifications" });
    const articles = within(feed).getAllByRole("article");
    expect(articles).toHaveLength(2);
    // Names are wrapped in directional isolates (FSI … PDI). The mention came after the marker: it stays marked new while the page is open.
    expect(articles[0]).toHaveAccessibleName(/^Unread: \u2068Bob\u2069 mentioned you/);
    expect(articles[1]).toHaveAccessibleName(/^\u2068Bob\u2069 followed you/);
    // The post the mention is about.
    expect(await within(articles[0] ?? feed).findByText("hello", { exact: false })).toBeInTheDocument();
    expect(within(articles[0] ?? feed).getByRole("link", { name: "Open post" })).toHaveAttribute("href", "/@bob/0192f0c4-7a8e-7000-8000-0000000000b1");

    // Showing the list moved the marker to the newest notification, once, and the badge went away.
    await waitFor(() => {
      expect(marker.updates).toEqual([notificationsFixture[0]?.id]);
    });
    await waitFor(() => {
      expect(notificationsLink()).toHaveAccessibleName("Notifications");
    });
    expect(articles[0]).toHaveAccessibleName(/^Unread: /);
    expect(statusRequests).toEqual([]);
  });

  test("does not update the marker when nothing new is shown", async () => {
    server.use(resumableSession());
    const marker = markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a2", unread_count: 0 });

    await renderApp({ path: "/notifications" });

    const feed = await screen.findByRole("feed", { name: "Notifications" });
    expect(within(feed).getAllByRole("article")[0]).toHaveAccessibleName(/^\u2068Bob\u2069 mentioned you/);
    expect(marker.updates).toEqual([]);
  });

  test("opens the post a notification is about on its page in the app", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a2", unread_count: 0 });
    const { user, router } = await renderApp({ path: "/notifications" });
    const feed = await screen.findByRole("feed", { name: "Notifications" });

    await user.click(within(feed).getByRole("link", { name: "Open post" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Post" })).toHaveFocus();
    expect(router.state.location.pathname).toBe("/@bob/0192f0c4-7a8e-7000-8000-0000000000b1");
    expect(await screen.findByRole("region", { name: "Replies" })).toBeInTheDocument();
  });

  test("filters by type from the URL and the tabs, without moving the marker", async () => {
    server.use(resumableSession());
    const marker = markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-000000000001", unread_count: 2 });
    const requested: string[][] = [];
    server.use(
      http.get(`${apiBase}/notifications`, ({ request }) => {
        const types = new URL(request.url).searchParams.getAll("types");
        requested.push(types);
        return HttpResponse.json({ items: notificationsFixture.filter((n) => types.includes(n.type)) });
      }),
    );

    const { user, router } = await renderApp({ path: "/notifications?filter=follows" });

    const tabs = await screen.findByRole("tablist", { name: "Notification types" });
    expect(within(tabs).getByRole("tab", { name: "Follows" })).toHaveAttribute("aria-selected", "true");
    const feed = await screen.findByRole("feed", { name: "Notifications" });
    expect(within(feed).getAllByRole("article")).toHaveLength(1);
    expect(within(feed).getByRole("article")).toHaveAccessibleName(/\u2068Bob\u2069 followed you/);
    expect(requested).toEqual([["follow", "follow_request", "follow_accepted"]]);

    await user.click(within(tabs).getByRole("tab", { name: "Mentions" }));

    await waitFor(() => {
      expect(router.state.location.search).toEqual({ filter: "mentions" });
    });
    expect(await screen.findByRole("article", { name: /\u2068Bob\u2069 mentioned you/ })).toBeInTheDocument();
    expect(within(tabs).getByRole("tab", { name: "Mentions" })).toHaveAttribute("aria-selected", "true");
    expect(marker.updates).toEqual([]);
  });

  test("ignores an unknown filter", async () => {
    server.use(resumableSession());

    await renderApp({ path: "/notifications?filter=nonsense" });

    const tabs = await screen.findByRole("tablist", { name: "Notification types" });
    expect(within(tabs).getByRole("tab", { name: "All" })).toHaveAttribute("aria-selected", "true");
  });

  test("loads older notifications a page at a time", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a2", unread_count: 0 });
    const [mention, follow] = notificationsFixture;
    server.use(
      http.get(`${apiBase}/notifications`, ({ request }) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        return HttpResponse.json(cursor === null ? { items: [mention], next_cursor: "page-2" } : { items: [follow] });
      }),
    );

    const { user } = await renderApp({ path: "/notifications" });

    const feed = await screen.findByRole("feed", { name: "Notifications" });
    expect(within(feed).getAllByRole("article")).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "Load more notifications" }));
    expect(await within(feed).findByRole("article", { name: /\u2068Bob\u2069 followed you/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more notifications" })).not.toBeInTheDocument();
  });

  test("says when there is nothing to show", async () => {
    server.use(resumableSession(), http.get(`${apiBase}/notifications`, () => HttpResponse.json({ items: [] })));

    await renderApp({ path: "/notifications" });

    expect(await screen.findByRole("heading", { level: 2, name: "No notifications yet" })).toBeInTheDocument();
  });

  test("names the actor of every type and hides a post behind its content warning", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000ff", unread_count: 0 });
    const types: NotificationType[] = ["follow_request", "follow_accepted", "boost", "favourite", "poll_closed", "moderation_notice", "report_resolved"];
    const items: Notification[] = types.map((type, i) => ({
      id: `0192f0c4-7a8e-7000-8000-0000000000c${String(i)}`,
      type,
      created_at: "2026-10-16T09:00:00Z",
      ...(type === "moderation_notice" || type === "report_resolved" ? {} : { account: { ...otherAccountFixture, display_name: "" } }),
      ...(type === "boost"
        ? { status: { ...statusFixture("0192f0c4-7a8e-7000-8000-0000000000b2"), spoiler_text: "spoilers", content: "<p>the ending</p>" } }
        : {}),
    }));
    server.use(http.get(`${apiBase}/notifications`, () => HttpResponse.json({ items })));

    const { user } = await renderApp({ path: "/notifications" });

    const feed = await screen.findByRole("feed", { name: "Notifications" });
    // Without a display name, the username stands in.
    expect(within(feed).getAllByRole("article").map((article) => article.getAttribute("aria-posinset"))).toEqual(["1", "2", "3", "4", "5", "6", "7"]);
    expect(within(feed).getByRole("article", { name: /\u2068bob\u2069 requested to follow you/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /\u2068bob\u2069 accepted your follow request/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /\u2068bob\u2069 favourited your post/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /A poll you took part in has ended/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /The moderators of this server sent you a notice/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /A report you filed has been resolved/ })).toBeInTheDocument();
    const boost = within(feed).getByRole("article", { name: /\u2068bob\u2069 boosted your post/ });
    const toggle = await within(boost).findByRole("button", { name: "Show content" });
    expect(within(boost).getByText("the ending")).not.toBeVisible();
    await user.click(toggle);
    expect(within(boost).getByText("the ending")).toBeVisible();
  });

  test("offers a retry when the list fails", async () => {
    let fail = true;
    server.use(
      resumableSession(),
      http.get(`${apiBase}/notifications`, () =>
        fail ? problem(503, "about:blank", "Service Unavailable") : HttpResponse.json({ items: notificationsFixture }),
      ),
    );

    const { user } = await renderApp({ path: "/notifications" });

    expect(await screen.findByRole("heading", { level: 1, name: "Something went wrong" })).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("feed", { name: "Notifications" })).toBeInTheDocument();
  });

  test("offers to sign in when no session resumes", async () => {
    const { user } = await renderApp({ path: "/notifications" });

    await user.click(await screen.findByRole("link", { name: "Sign in" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
  });
});
