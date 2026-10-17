// SPDX-License-Identifier: AGPL-3.0-only
// The notifications page and the unread badge through the whole app (web-client.md T4): the real router
// and QueryClient, with MSW as the server (FR-205).
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { findLoadingRegion } from "../../test/loading";
import { apiBase, problem, resumableSession } from "../../test/msw/http";
import { notificationsFixture } from "./test/handlers";
import { otherAccountFixture, relationshipFixture, statusFixture } from "../../test/fixtures";
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
    // The mention came after the marker: it stays marked new while the page is open.
    expect(articles[0]).toHaveAccessibleName(/^Unread: Bob mentioned you/);
    expect(articles[1]).toHaveAccessibleName(/^Bob followed you/);
    // The post the mention is about is quoted as its text, in one link to its page.
    expect(within(articles[0] ?? feed).getByRole("link", { name: "@alice hello" })).toHaveAttribute("href", "/@bob/0192f0c4-7a8e-7000-8000-0000000000b1");
    // The time is relative as in the timelines, with the full date and time as its tooltip.
    const time = within(articles[0] ?? feed).getByRole("time");
    expect(time).toHaveAttribute("datetime", "2026-10-16T09:00:00Z");
    expect(time.getAttribute("title")).toMatch(/2026/);
    expect(time.textContent).not.toBe(time.getAttribute("title"));

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
    expect(within(feed).getAllByRole("article")[0]).toHaveAccessibleName(/^Bob mentioned you/);
    expect(marker.updates).toEqual([]);
  });

  test("opens the post a notification is about on its page in the app", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a2", unread_count: 0 });
    const { user, router } = await renderApp({ path: "/notifications" });
    const feed = await screen.findByRole("feed", { name: "Notifications" });

    await user.click(within(feed).getByRole("link", { name: "@alice hello" }));

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
    expect(within(feed).getByRole("article")).toHaveAccessibleName(/Bob followed you/);
    expect(requested).toEqual([["follow", "follow_request", "follow_accepted"]]);

    await user.click(within(tabs).getByRole("tab", { name: "Mentions" }));

    await waitFor(() => {
      expect(router.state.location.search).toEqual({ filter: "mentions" });
    });
    expect(await screen.findByRole("article", { name: /Bob mentioned you/ })).toBeInTheDocument();
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
    expect(await within(feed).findByRole("article", { name: /Bob followed you/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more notifications" })).not.toBeInTheDocument();
  });

  test("shows the heading and a skeleton of notifications while the first page loads", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a2", unread_count: 0 });
    let release = () => {};
    const released = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${apiBase}/notifications`, async () => {
        await released;
        return HttpResponse.json({ items: notificationsFixture });
      }),
    );
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await user.click(notificationsLink());

    // After the router's pending delay, the page has its heading and a skeleton named once.
    await findLoadingRegion("Loading notifications");
    expect(screen.getByRole("heading", { level: 1, name: "Notifications" })).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    release();

    expect(await screen.findByRole("feed", { name: "Notifications" })).toBeInTheDocument();
    expect(screen.queryByText("Loading notifications")).not.toBeInTheDocument();
  });

  test("says when there is nothing to show", async () => {
    server.use(resumableSession(), http.get(`${apiBase}/notifications`, () => HttpResponse.json({ items: [] })));

    await renderApp({ path: "/notifications" });

    expect(await screen.findByRole("heading", { level: 2, name: "No notifications yet" })).toBeInTheDocument();
  });

  test("names the actor of every type and hides a post behind its content warning", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000ff", unread_count: 0 });
    const types: NotificationType[] = ["follow_request", "follow_accepted", "boost", "favourite", "poll_closed", "moderation_notice"];
    const items: Notification[] = types.map((type, i) => ({
      id: `0192f0c4-7a8e-7000-8000-0000000000c${String(i)}`,
      type,
      created_at: "2026-10-16T09:00:00Z",
      ...(type === "moderation_notice" ? {} : { account: { ...otherAccountFixture, display_name: "" } }),
      ...(type === "boost"
        ? { status: { ...statusFixture("0192f0c4-7a8e-7000-8000-0000000000b2"), spoiler_text: "spoilers", content: "<p>the ending</p>" } }
        : {}),
    }));
    server.use(http.get(`${apiBase}/notifications`, () => HttpResponse.json({ items })));

    await renderApp({ path: "/notifications" });

    const feed = await screen.findByRole("feed", { name: "Notifications" });
    // Without a display name, the username stands in.
    expect(within(feed).getAllByRole("article").map((article) => article.getAttribute("aria-posinset"))).toEqual(["1", "2", "3", "4", "5", "6"]);
    expect(within(feed).getByRole("article", { name: /bob requested to follow you/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /bob accepted your follow request/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /bob favourited your post/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /A poll you took part in has ended/ })).toBeInTheDocument();
    expect(within(feed).getByRole("article", { name: /The moderators of this server sent you a notice/ })).toBeInTheDocument();
    // A post behind a content warning shows the warning only; its page asks to open it.
    const boost = within(feed).getByRole("article", { name: /bob boosted your post/ });
    expect(within(boost).getByRole("link", { name: "Content warning: spoilers" })).toHaveAttribute("href", "/@bob/0192f0c4-7a8e-7000-8000-0000000000b2");
    expect(within(boost).queryByText("the ending", { exact: false })).not.toBeInTheDocument();
    // The types without an actor link to no profile.
    expect(within(within(feed).getByRole("article", { name: /The moderators/ })).queryAllByRole("link")).toEqual([]);
  });

  test("links the actor to their profile and offers a follow back, one tab stop each", async () => {
    server.use(resumableSession());
    markerServer({ last_read_id: "0192f0c4-7a8e-7000-8000-0000000000a2", unread_count: 0 });
    server.use(
      http.get(`${apiBase}/accounts/relationships`, ({ request }) =>
        HttpResponse.json(new URL(request.url).searchParams.getAll("id").map((id) => relationshipFixture(id, { followed_by: true }))),
      ),
    );
    const { user, router } = await renderApp({ path: "/notifications?filter=follows" });
    const follow = await screen.findByRole("article", { name: /Bob followed you/ });

    // The follow back is the profile's follow button, without the menu of mute and block.
    const followBack = await within(follow).findByRole("button", { name: "Follow back" });
    expect(within(follow).getAllByRole("button")).toEqual([followBack]);
    // The avatar is the same link as the name for the pointer, out of the tab order and hidden from screen readers.
    const name = within(follow).getByRole("link", { name: "Bob" });
    expect(within(follow).getAllByRole("link")).toEqual([name]);

    // From the keyboard alone: the tab stops of the row are the name, then the follow back.
    name.focus();
    await user.tab();
    expect(followBack).toHaveFocus();
    await user.click(followBack);
    expect(await within(follow).findByRole("button", { name: "Following" })).toHaveFocus();

    await user.tab({ shift: true });
    expect(name).toHaveFocus();
    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/@bob");
    });
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

    // The page keeps its heading; the list shows what failed and why, without interrupting as an alert (G16).
    expect(await screen.findByText("Service Unavailable")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Could not load the notifications" })).toBeInTheDocument();
    expect(within(screen.getByRole("main")).queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Notifications" })).toBeInTheDocument();
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
